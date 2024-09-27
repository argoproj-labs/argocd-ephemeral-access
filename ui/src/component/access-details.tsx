import React, { useEffect, useState } from 'react';
import { BUTTON_LABELS } from '../constant';
import { getAccess, requestAccess } from '../config/client';
import { UserInfo, Application, AccessRequest } from '../models/type';
import { Spinner } from '../utils/utils';
import './access-details.scss';
import moment from 'moment/moment';

interface AccessDetailsomponentProps {
  application: Application;
  userInfo: UserInfo;
}

const requestAccessHandler = async (
  application: Application,
  userInfo: UserInfo,
  setInitiated: React.Dispatch<React.SetStateAction<boolean>>,
  setAccessRequest: React.Dispatch<React.SetStateAction<AccessRequest>>,
) => {
  try {
    const response: AccessRequest = await requestAccess(application, userInfo.username);
    if (response) {
      localStorage.setItem(application.metadata.name, JSON.stringify(response));
    }
    setAccessRequest(response);
    setInitiated(true);
  } catch (error) {
    console.error('Error requesting access:', error);
    setInitiated(false);
  }
};

const AccessDetails: React.FC<AccessDetailsomponentProps> = ({ application, userInfo }) => {
  const [accessRequest, setAccessRequest] = useState<AccessRequest>(
    JSON.parse(localStorage.getItem(application?.metadata?.name)) || (null as AccessRequest)
  );
  const [initiated, setInitiated] = useState(false);
  useEffect(() => {
    const interval = setInterval(async () => {
      const response = await getAccess(application, userInfo.username);

      if (response && response?.items) {
        setInitiated(true);
        setAccessRequest(response?.items[0]);
      }
      if (accessRequest?.status === 'EXPIRED') {
        localStorage.removeItem(application.metadata.name);
        setAccessRequest({} as AccessRequest);
      }
    }, 5000);

    return () => clearInterval(interval);
  }, []);

  const cancel = () => {
    setAccessRequest({} as AccessRequest);
    setInitiated(false);
  };
  return (
    <div className='access-form'>
      <button
        style={{ position: 'relative', minWidth: '120px', minHeight: '20px' }}
        className='argo-button argo-button--base'
        disabled={initiated}
        onClick={() => requestAccessHandler(application, userInfo, setInitiated, setAccessRequest)}
      >
        {accessRequest?.status !== 'ACTIVE' && accessRequest?.status !== 'DENIED' && (
          <span>
            <Spinner show={initiated} style={{ marginRight: '5px' }} />
          </span>
        )}
        {BUTTON_LABELS.REQUEST_ACCESS}
      </button>
      <button
        style={{ position: 'relative', minWidth: '120px', minHeight: '20px' }}
        className='argo-button argo-button--base'
        disabled={!initiated}
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
            {window.GLOBAL_ARGOCD_ACCESS_EXT_MAIN_BANNER}
            {window.GLOBAL_ARGOCD_ACCESS_EXT_MAIN_BANNER_ADDITIONAL_INFO_LINK && (
              <a
                style={{ color: 'blue', textDecoration: 'underline' }}
                href={window.GLOBAL_ARGOCD_ACCESS_EXT_MAIN_BANNER_ADDITIONAL_INFO_LINK}
                target={'_blank'}
              >
                {' '}
                Read more.
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
            <div className='columns small-9'>
              {accessRequest?.permission?.toUpperCase() || 'Read Only'}
            </div>
          </div>
          {accessRequest?.expiresAt && (
            <div>
              <div className='row white-box__details-row'>
                <div className='columns small-3'>REQUEST DATA</div>
                <div className='columns small-9'>
                  {moment(accessRequest?.requestedAt).format('MMMM Do YYYY, h:mm:ss a')}
                </div>
              </div>
              <div className='row white-box__details-row'>
                <div className='columns small-3'>ROLE</div>
                <div className='columns small-9'>{accessRequest?.role?.toUpperCase()}</div>
              </div>
              <div className='row white-box__details-row'>
                <div className='columns small-3'>STATUS</div>
                <div className='columns small-9'>{accessRequest?.status?.toUpperCase()}</div>
              </div>
              <div className='row white-box__details-row'>
                <div className='columns small-3'>MESSAGE</div>
                <div className='columns small-9'>
                  {accessRequest?.status === 'PENDING' ? (
                    <span style={{ display: 'flex', flexDirection: 'column' }}>
                      {accessRequest?.message}
                      <a href={window.GLOBAL_ARGOCD_ACCESS_EXT_CHANGE_REQUEST_URL} style={{}}>
                        Click to create change request
                      </a>
                    </span>
                  ) : (
                    accessRequest?.message
                  )}
                </div>
              </div>
              {accessRequest?.status === 'ACTIVE' && accessRequest?.expiresAt && (
                <div className='row white-box__details-row'>
                  <div className='columns small-3'>Access Expires:</div>
                  <div className='columns small-9'>
                    {moment(accessRequest?.expiresAt).format('MMMM Do YYYY, h:mm:ss a')}
                  </div>
                </div>
              )}
            </div>
          )}
        </div>
      </div>
    </div>
  );
};

export default AccessDetails;
