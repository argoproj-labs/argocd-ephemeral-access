import React, { useEffect, useState, useCallback } from 'react';
import { ToastContainer, toast } from 'react-toastify';
import 'react-toastify/dist/ReactToastify.css';

import { BUTTON_LABELS } from '../constant';
import { UserInfo, Application } from '../models/type';
import { Spinner } from '../utils/utils';
import './ephemeral-access-details.scss';
import moment from 'moment/moment';

import {
  AccessRequestResponseBody,
  AccessRequestResponseBodyStatus,
  createAccessrequest,
  CreateAccessRequestBody,
  listAccessrequest
} from '../gen/ephemeralAccessAPI';
import { getHeaders } from '../config/client';

interface AccessDetailsComponentProps {
  application: Application;
  userInfo: UserInfo;
}

const EphemeralAccessDetails: React.FC<AccessDetailsComponentProps> = ({
  application: application,
  userInfo
}) => {
  const [accessRequest, setAccessRequest] = useState<AccessRequestResponseBody>(null);
  const [enabled, setEnabled] = useState(accessRequest === null);
  const applicationNamespace = application?.metadata?.namespace || '';
  const applicationName = application?.metadata?.name || '';
  const project = application?.spec?.project || '';
  const username = userInfo?.username;
  const notify = (msg: string) => toast.warning('system message: ' + msg);

  const fetchAccess = useCallback(async (): Promise<AccessRequestResponseBody | null> => {
    try {
      const { data } = await listAccessrequest({
        baseURL: '/extensions/ephemeral/',
        headers: getHeaders({ applicationName, applicationNamespace, project, username })
      });
      const accessRequestData = data.items[0];
      if (data && data.items.length > 0) {
        setAccessRequest(accessRequestData);
        setEnabled(false);
        localStorage.setItem(
          application?.metadata?.name,
          JSON.stringify(
            data.items.find((item) => item.status === AccessRequestResponseBodyStatus.GRANTED) ||
              null
          )
        );
      } else {
        setEnabled(true);
        localStorage.setItem(application?.metadata?.name, 'null');
      }

      switch (accessRequestData?.status) {
        case AccessRequestResponseBodyStatus.GRANTED:
          setEnabled(false);
          break;
        case AccessRequestResponseBodyStatus.DENIED:
          notify(
            'last request was denied: ' + accessRequestData?.message &&
              accessRequestData?.message + '. Please try again!'
          );
          setEnabled(true);
          setAccessRequest(null);
          break;
        case AccessRequestResponseBodyStatus.REQUESTED:
          setEnabled(false);
          break;
        default:
          setEnabled(true);
          break;
      }
      return accessRequestData;
    } catch (error) {
      setEnabled(true);
      notify('Failed to connect to  backend: ' + error.message);
    }

    return null;
  }, []);

  const requestAccessHandler = useCallback(async (): Promise<CreateAccessRequestBody | null> => {
    try {
      await createAccessrequest(
        {
          roleName: window?.EPHEMERAL_ACCESS_VARS?.EPHEMERAL_ACCESS_DEFAULT_ROLE
        },
        {
          baseURL: '/extensions/ephemeral/',
          headers: getHeaders({ applicationName, applicationNamespace, project, username })
        }
      );

      const intervalId = setInterval(async () => {
        const updatedAccessData = await fetchAccess();
        if (
          updatedAccessData?.status === AccessRequestResponseBodyStatus.GRANTED ||
          updatedAccessData?.status === AccessRequestResponseBodyStatus.DENIED
        ) {
          if (updatedAccessData?.expiresAt) {
            const timeoutDuration =
              moment.parseZone(updatedAccessData.expiresAt).valueOf() - moment().valueOf();
            if (timeoutDuration > 0) {
              setTimeout(() => {
                setAccessRequest(null);
                setEnabled(true);
              }, timeoutDuration);
            }
          }
          clearInterval(intervalId);
        }
      }, 500);
    } catch (error) {
      setEnabled(true);
      if (error.response) {
        switch (error.response.status) {
          case 409:
            notify('permission request already exists');
            const accessData = await fetchAccess();
            if (
              accessData?.status === AccessRequestResponseBodyStatus.GRANTED ||
              accessData?.status === AccessRequestResponseBodyStatus.DENIED
            ) {
              setAccessRequest(accessData);
              setEnabled(false);
            }
            break;
          case 401:
          case 403:
            notify('extension is not authorized: ' + error.message);
            break;
          case 502:
            notify('error occurred while requesting permission: ' + error.message);
            break;
          default:
            notify('failed to connect to backend: ' + error.message);
            break;
        }
      } else {
        notify('An unexpected error occurred: ' + error.message);
      }
      setEnabled(false);
      return null;
    }
  }, [fetchAccess]);
  useEffect(() => {
    const fetchData = async () => {
      const updatedAccessData = await fetchAccess();
      if (updatedAccessData?.expiresAt) {
        const timeoutDuration =
          moment.parseZone(updatedAccessData.expiresAt).valueOf() - moment().valueOf();
        if (timeoutDuration > 0) {
          setTimeout(() => {
            setAccessRequest(null);
            setEnabled(true);
          }, timeoutDuration);
        }
      }
    };

    fetchData();
  }, []);

  const cancel = useCallback(() => {
    setAccessRequest(null);
    setEnabled(true);
  }, []);

  return (
    <div className='access-form'>
      <button
        style={{ position: 'relative', minWidth: '120px', minHeight: '20px' }}
        className='argo-button argo-button--base'
        disabled={!enabled}
        onClick={requestAccessHandler}
      >
        {accessRequest?.status !== AccessRequestResponseBodyStatus.GRANTED &&
          accessRequest?.status !== AccessRequestResponseBodyStatus.DENIED && (
            <span>
              <Spinner show={!enabled} style={{ marginRight: '5px' }} />
            </span>
          )}
        {BUTTON_LABELS.REQUEST_ACCESS}
      </button>
      <button
        style={{ position: 'relative', minWidth: '120px', minHeight: '20px' }}
        className='argo-button argo-button--base'
        disabled={enabled || accessRequest?.status === AccessRequestResponseBodyStatus.GRANTED}
        onClick={cancel}
      >
        {BUTTON_LABELS.CANCEL}
      </button>

      <div className='access-form__usrmsg'>
        <i className='fa fa-info-circle icon-background' />
        <div className='access-form__usrmsg__warning'>
          <div className='access-form__usrmsg__warning-title'>
            About Requesting Temporary Access
          </div>
          <div className='access-form__usrmsg__warning-content'>
            {window?.EPHEMERAL_ACCESS_VARS?.EPHEMERAL_ACCESS_MAIN_BANNER}
            {window?.EPHEMERAL_ACCESS_VARS?.EPHEMERAL_ACCESS_MAIN_BANNER_ADDITIONAL_INFO_LINK && (
              <a
                style={{ color: 'blue', textDecoration: 'underline' }}
                href={
                  window?.EPHEMERAL_ACCESS_VARS?.EPHEMERAL_ACCESS_MAIN_BANNER_ADDITIONAL_INFO_LINK
                }
                target={'_blank'}
              >
                Read More
              </a>
            )}
          </div>
        </div>
      </div>
      <div className='white-box' style={{ marginTop: '15px' }}>
        <div className='white-box__details'>
          <p>USER'S CURRENT PERMISSION</p>

          <div className='row white-box__details-row'>
            <div className='columns small-3'>USER NAME</div>
            <div className='columns small-9'>{userInfo?.username?.toUpperCase()}</div>
          </div>
          <div className='row white-box__details-row'>
            <div className='columns small-3'>PERMISSION</div>
            <div className='columns small-9'>{accessRequest?.permission || 'Read Only'}</div>
          </div>
          {accessRequest && (
            <div>
              <div className='row white-box__details-row'>
                <div className='columns small-3'>REQUEST DATA</div>
                <div className='columns small-9'>
                  {moment(accessRequest?.requestedAt).format('MMMM Do YYYY, h:mm:ss a')}
                </div>
              </div>
              <div className='row white-box__details-row'>
                <div className='columns small-3'>ROLE</div>
                <div className='columns small-9'>{accessRequest?.role}</div>
              </div>
              <div className='row white-box__details-row'>
                <div className='columns small-3'>STATUS</div>
                <div className='columns small-9'>{accessRequest?.status}</div>
              </div>

              {accessRequest?.expiresAt && (
                <div
                  className='row white-box__details-row'
                  style={{ display: 'flex', alignItems: 'center' }}
                >
                  <div className='columns small-3'>EXPIRES</div>
                  <div className='columns small-9'>
                    {moment(accessRequest?.expiresAt).format('MMMM Do YYYY, h:mm:ss a')}
                  </div>
                </div>
              )}
              <div className='row white-box__details-row'>
                <div className='columns small-3'>MESSAGE</div>
                <div className='columns small-9'>
                  {accessRequest?.status === AccessRequestResponseBodyStatus.REQUESTED ? (
                    <span style={{ display: 'flex', flexDirection: 'column', margin: '0' }}>
                      {accessRequest?.message}
                      {window?.EPHEMERAL_ACCESS_VARS?.EPHEMERAL_ACCESS_CHANGE_REQUEST_URL && (
                        <a
                          href={window?.EPHEMERAL_ACCESS_VARS?.EPHEMERAL_ACCESS_CHANGE_REQUEST_URL}
                          style={{ color: 'blue', textDecoration: 'underline' }}
                          target={'_blank'}
                        >
                          Click here to create
                        </a>
                      )}
                    </span>
                  ) : (
                    accessRequest?.message
                  )}
                </div>
              </div>
            </div>
          )}
        </div>
      </div>
      <ToastContainer />
    </div>
  );
};

export default EphemeralAccessDetails;
