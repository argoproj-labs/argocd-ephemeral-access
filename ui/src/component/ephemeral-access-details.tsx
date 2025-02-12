import React, { useCallback, useEffect, useRef, useState } from 'react';
import { ToastContainer, toast } from 'react-toastify';
import 'react-toastify/dist/ReactToastify.css';
import { BUTTON_LABELS } from '../constant';
import { UserInfo, Application } from '../models/type';
import { Spinner, getAccessRoles } from '../utils/utils';
import './ephemeral-access-details.scss';
import moment from 'moment';
import {
  AccessRequestResponseBody,
  AccessRequestResponseBodyStatus,
  createAccessrequest,
  CreateAccessRequestBody,
  listAccessrequest,
  AllowedRoleResponseBody
} from '../gen/ephemeral-access-api';
import { getHeaders } from '../config/client';

interface AccessDetailsComponentProps {
  application: Application;
  userInfo: UserInfo;
}

const EphemeralAccessDetails: React.FC<AccessDetailsComponentProps> = ({
                                                                         application,
                                                                         userInfo
                                                                       }) => {
  const [accessRequest, setAccessRequest] = useState<AccessRequestResponseBody | null>(null);
  const [enabled, setEnabled] = useState(true);
  const [roles, setRoles] = useState<AllowedRoleResponseBody[]>([]);
  const [selectedRole, setSelectedRole] = useState<string>('');
  const selectedRoleRef = useRef(selectedRole);

  const applicationNamespace = application?.metadata?.namespace || '';
  const applicationName = application?.metadata?.name || '';
  const project = application?.spec?.project || '';
  const username = userInfo?.username || '';

  const notify = (msg: string) => toast.warning('System message: ' + msg);

  const fetchRoles = useCallback(async () => {
    try {
      const fetchedRoles = await getAccessRoles(applicationName, applicationNamespace, project, username);
      setRoles(fetchedRoles);
      if (fetchedRoles.length === 1) {
        setSelectedRole(fetchedRoles[0].roleName);
      }
    } catch (error) {
      console.error('Error fetching roles:', error);
      notify('Failed to fetch roles: ' + error.message);
    }
  }, [applicationName, applicationNamespace, project, username]);

  const fetchAccess = useCallback(async (): Promise<AccessRequestResponseBody | null> => {
    try {
      const { data } = await listAccessrequest({
        baseURL: '/extensions/ephemeral/',
        headers: getHeaders({ applicationName, applicationNamespace, project, username })
      });
      const accessRequestData = data.items[0];
      if (data.items.length > 0) {
        setAccessRequest(accessRequestData);
        setEnabled(false);
        localStorage.setItem(
            applicationName,
            JSON.stringify(data.items.find(item => item.status === AccessRequestResponseBodyStatus.GRANTED) || null)
        );
      } else {
        setEnabled(true);
        localStorage.setItem(applicationName, 'null');
      }

      handleAccessRequestStatus(accessRequestData);
      return accessRequestData;
    } catch (error) {
      setEnabled(true);
      notify('Failed to connect to backend: ' + error.message);
      return null;
    }
  }, [applicationName, applicationNamespace, project, username]);

  const handleAccessRequestStatus = (accessRequestData: AccessRequestResponseBody) => {
    switch (accessRequestData?.status) {
      case AccessRequestResponseBodyStatus.GRANTED:
        setEnabled(false);
        break;
      case AccessRequestResponseBodyStatus.DENIED:
        notify('Last request was denied: ' + accessRequestData?.message + '. Please try again!');
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
  };

  const requestAccessHandler = async (): Promise<CreateAccessRequestBody | null> => {
    try {
      if (!selectedRoleRef.current && roles.length > 1) {
        notify('Please select a role from the dropdown');
        return null;
      }

      setEnabled(false);
      await createAccessrequest(
          { roleName: selectedRoleRef.current || roles[0].roleName },
          { baseURL: '/extensions/ephemeral/', headers: getHeaders({ applicationName, applicationNamespace, project, username }) }
      );

      const intervalId = setInterval(async () => {
        const updatedAccessData = await fetchAccess();
        if (
            updatedAccessData?.status === AccessRequestResponseBodyStatus.GRANTED ||
            updatedAccessData?.status === AccessRequestResponseBodyStatus.DENIED
        ) {
          if (updatedAccessData?.expiresAt) {
            const timeoutDuration = moment.parseZone(updatedAccessData.expiresAt).valueOf() - moment().valueOf();
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
      handleRequestError(error);
      return null;
    }
  };

  const handleRequestError = async (error: any) => {
    setEnabled(true);
    if (error.response) {
      switch (error.response.status) {
        case 409:
          notify('Permission request already exists');
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
          notify('Extension is not authorized: ' + error.message);
          break;
        case 502:
          notify('Error occurred while requesting permission: ' + error.message);
          break;
        default:
          notify('Failed to connect to backend: ' + error.message);
          break;
      }
    } else {
      notify('An unexpected error occurred: ' + error.message);
    }
  };

  const handleRoleChange = (e: React.ChangeEvent<HTMLSelectElement>) => {
    const selectedValue = e.target.value;
    selectedRoleRef.current = selectedValue;
  };

  const cancel = () => {
    setAccessRequest(null);
    selectedRoleRef.current = '';
    setEnabled(true);
  };

  useEffect(() => {
    fetchRoles();
    fetchAccess();
  }, [fetchRoles, fetchAccess]);

  return (
      <div className='access-form'>
        <button
            style={{ position: 'relative', minWidth: '120px', minHeight: '20px' }}
            className='argo-button argo-button--base'
            onClick={requestAccessHandler}
            disabled={!enabled || accessRequest?.status === AccessRequestResponseBodyStatus.GRANTED}
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
            onClick={cancel}
        >
          {BUTTON_LABELS.CANCEL}
        </button>

        {roles.length > 1 && accessRequest == null && (
            <div className='access-form__role-select'>
              <select
                  value={selectedRole}
                  onChange={handleRoleChange}
                  style={{
                    textAlign: 'center',
                    textAlignLast: 'center'
                  }}
              >
                <option value=''>---select a role---</option>
                {roles.map((role) => (
                    <option key={role.roleName} value={role.roleName}>
                      {role.roleDisplayName}
                    </option>
                ))}
              </select>
            </div>
        )}
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
                      target='_blank'
                      rel='noopener noreferrer'
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
                                    target='_blank'
                                    rel='noopener noreferrer'
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