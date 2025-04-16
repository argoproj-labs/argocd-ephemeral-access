import React, { useEffect, useRef, useState } from 'react';
import ReactMarkdown from 'react-markdown';
import { toast, ToastContainer } from 'react-toastify';
import 'react-toastify/dist/ReactToastify.css';
import { BUTTON_LABELS } from '../constant';
import { Application, UserInfo } from '../models/type';
import { getAccessRoles, getDisplayTime, getDisplayValue, Spinner } from '../utils/utils';
import EphemeralRoleSelection from './ephemeral-role-selection';
import './style.scss';
import moment from 'moment';
import {
  AccessRequestResponseBody,
  AccessRequestResponseBodyStatus,
  AllowedRoleResponseBody,
  createAccessrequest,
  listAccessrequest
} from '../gen/ephemeral-access-api';
import { getHeaders } from '../config/client';
import { SelectOption } from 'argo-ui/src/components/select/select';

interface AccessDetailsComponentProps {
  application: Application;
  userInfo: UserInfo;
}

const EphemeralAccessDetails: React.FC<AccessDetailsComponentProps> = ({
  application,
  userInfo
}) => {
  const [currentAccessRequest, setCurrentAccessRequest] =
    useState<AccessRequestResponseBody | null>(null);
  const [roles, setRoles] = useState<AllowedRoleResponseBody[]>([]);
  const [selectedRole, setSelectedRole] = useState<string>('');
  const [errorMessage, setErrorMessage] = useState('');
  const [isLoading, setIsLoading] = useState<boolean>(false);
  const applicationNamespace = application?.metadata?.namespace || '';
  const applicationName = application?.metadata?.name || '';
  const project = application?.spec?.project || '';
  const username = userInfo?.username || '';
  const selectedRoleRef = useRef(selectedRole);

  const notify = (msg: string) => toast.warning('System message: ' + msg);

  const returnError = (error: any) => {
    const status = error?.response?.status;
    switch (status) {
      case 409:
        notify(`${selectedRole} role: A permission request already exists.`);
        break;
      case 401:
        notify(`Unauthorized request: ${error.message}`);
        break;
      case 403:
        notify('Access Request Denied: No valid role was found. Please verify your permissions.');
        break;
      case 408:
        break;
      default:
        notify(`Error occurred while requesting permission: ${error.message}`);
        break;
    }
  };

  const resetPermissionState = () => {
    setCurrentAccessRequest(null);
    setSelectedRole('');
    selectedRoleRef.current = '';
    localStorage.setItem(applicationName, 'null');
  };

  const handleAccessExpiration = (accessData: AccessRequestResponseBody) => {
    if (accessData?.expiresAt) {
      localStorage.setItem(applicationName, JSON.stringify(accessData || null));
      const timeoutDuration = moment.parseZone(accessData.expiresAt).valueOf() - moment().valueOf();
      if (timeoutDuration > 0) {
        setTimeout(() => {
          resetPermissionState();
        }, timeoutDuration);
      }
    }
  };

  const AccessRoles: () => Promise<void> = async () => {
    try {
      const accessRoles = await getAccessRoles(
        applicationName,
        applicationNamespace,
        project,
        username
      );
      setRoles(accessRoles || []);
      if ((accessRoles || []).length === 1) {
        setSelectedRole(accessRoles[0]?.roleName);
      }
    } catch (error) {
      returnError(error);
    }
  };

  const fetchAccessRequest: () => Promise<void> = async () => {
    let currentDelay = 300;
    // max delay 3 seconds
    const maxDelay = 3000;
    // 1 hour max polling duration
    const maxPollingDuration = 3600000;

    const pollingEndTime = Date.now() + maxPollingDuration;

    const poll = async () => {
      try {
        const { data } = await listAccessrequest({
          headers: getHeaders({ applicationName, applicationNamespace, project })
        });
        const accessRequestData: AccessRequestResponseBody | null =
          data.items.length > 0 ? data.items[0] : null;

        const status = accessRequestData && accessRequestData?.status;
        setCurrentAccessRequest(accessRequestData);
        switch (status) {
          case AccessRequestResponseBodyStatus.GRANTED:
            handleAccessExpiration(accessRequestData);
            break;
          case undefined:
          case AccessRequestResponseBodyStatus.REQUESTED:
          case AccessRequestResponseBodyStatus.INITIATED:
            if (Date.now() < pollingEndTime) {
              setIsLoading(true);
              // Exponential backoff
              currentDelay = Math.min(currentDelay * 2, maxDelay);
              setTimeout(poll, currentDelay);
            } else {
              setIsLoading(false);
              const errorObject = {
                response: {
                  status: 408
                },
                message: 'Polling timed out. Check the status of the change request!'
              };
              returnError(errorObject);
            }
            break;
          default:
            resetPermissionState();
            break;
        }
        return;
      } catch (error) {
        setIsLoading(false);
        returnError(error);
      }
    };
    await poll();
  };

  const submitAccessRequest = async () => {
    try {
      if (!selectedRoleRef.current && roles.length > 1) {
        setErrorMessage('Please select a role');
        return;
      }
      const roleName = selectedRoleRef.current || (roles.length > 0 ? roles[0].roleName : '');
      await createAccessrequest(
        { roleName },
        {
          headers: getHeaders({ applicationName, applicationNamespace, project })
        }
      );

      await fetchAccessRequest();
    } catch (error) {
      returnError(error);
    }
  };

  const options: SelectOption[] = roles.map((role) => ({
    value: role.roleName,
    title: role.roleDisplayName
  }));

  const selectRoleChange = (selectedOption: SelectOption) => {
    if (selectedOption.value !== '') {
      setErrorMessage('');
    }
    selectedRoleRef.current = selectedOption.value;
    setSelectedRole(selectedOption.value);
  };

  useEffect(() => {
    AccessRoles().then((r) => r);
    fetchAccessRequest().then((r) => r);
  }, [applicationName]);

  const {
    status = '',
    permission = '',
    role = '',
    requestedAt = '',
    message = '',
    expiresAt = ''
  } = currentAccessRequest || {};
  const changeRequestUrl = window?.EPHEMERAL_ACCESS_VARS?.EPHEMERAL_ACCESS_CHANGE_REQUEST_URL;
  const mainBanner = window?.EPHEMERAL_ACCESS_VARS?.EPHEMERAL_ACCESS_MAIN_BANNER;
  const mainBannerLink =
    window?.EPHEMERAL_ACCESS_VARS?.EPHEMERAL_ACCESS_MAIN_BANNER_ADDITIONAL_INFO_LINK;
  return (
    <div className='access-form'>
      <div className=''>
        <button
          qe-id='request-access-btn'
          style={{ position: 'relative', minWidth: '120px', minHeight: '20px' }}
          className='argo-button argo-button--base'
          onClick={() => {
            submitAccessRequest();
          }}
        >
          {BUTTON_LABELS.REQUEST_ACCESS}
        </button>
      </div>
      <div className='access-form__usrmsg'>
        <i aria-hidden='true' className='fa fa-info-circle icon-background' />
        <div className='access-form__usrmsg__warning'>
          <div className='access-form__usrmsg__warning-title'>
            About Requesting Temporary Access
          </div>
          <div className='access-form__usrmsg__warning-content'>
            {mainBanner}
            {mainBannerLink && (
              <a
                style={{ color: 'blue', textDecoration: 'underline' }}
                href={mainBannerLink}
                target='_blank'
                rel='noopener noreferrer'
              >
                Read More
              </a>
            )}
          </div>
        </div>
      </div>
      <div style={{ marginTop: '15px' }}>
        {roles.length > 1 && (
          <div className='white-box' style={{ marginTop: '15px' }}>
            <EphemeralRoleSelection
              selectedRole={selectedRole}
              options={options}
              selectRoleChange={selectRoleChange}
              validationMessage={errorMessage}
            />
          </div>
        )}
      </div>
      <div className='white-box' style={{ marginTop: '15px' }}>
        <div className='white-box__details'>
          <p>USER'S CURRENT PERMISSION</p>

          <div className='row white-box__details-row'>
            <div className='columns small-3'>USER NAME</div>
            <div className='columns small-9'>{getDisplayValue(userInfo?.username)}</div>
          </div>
          <div className='row white-box__details-row'>
            <div className='columns small-3'>PERMISSION</div>
            <div className='columns small-9'>{getDisplayValue(permission) || 'read only'}</div>
          </div>
          {currentAccessRequest && (
            <div>
              <div className='row white-box__details-row'>
                <div className='columns small-3'>ROLE</div>
                <div className='columns small-9'>{getDisplayValue(role)}</div>
              </div>
              <div className='row white-box__details-row'>
                <div className='columns small-3'>STATUS</div>
                <div className='columns small-9'>
                  {getDisplayValue(status)}
                  {isLoading && status === AccessRequestResponseBodyStatus.REQUESTED && (
                    <>
                      <Spinner show={true} style={{ margin: '5px' }} />
                    </>
                  )}
                </div>
              </div>
              <div className='row white-box__details-row'>
                <div className='columns small-3'>REQUESTED AT</div>
                <div className='columns small-9'>{getDisplayTime(requestedAt)}</div>
              </div>
              <div className='row white-box__details-row'>
                <div className='columns small-3'>EXPIRES</div>
                <div className='columns small-9'>{getDisplayTime(expiresAt)}</div>
              </div>
              <div className='row white-box__details-row'>
                <div className='columns small-3'>MESSAGE</div>
                <div className='columns small-9' style={{ lineHeight: '1.75' }}>
                  <span style={{ display: 'flex', flexDirection: 'column', marginTop: '15px' }}>
                    <ReactMarkdown
                      components={{
                        a: ({ node, ...props }) => (
                          <a {...props} target='_blank' rel='noopener noreferrer'>
                            {props.children}
                          </a>
                        )
                      }}
                    >
                      {message}
                    </ReactMarkdown>
                    {status === AccessRequestResponseBodyStatus.REQUESTED && changeRequestUrl && (
                      <a
                        href={changeRequestUrl}
                        style={{ color: 'blue', textDecoration: 'underline' }}
                        target='_blank'
                        rel='noopener noreferrer'
                      >
                        Click here to create
                      </a>
                    )}
                  </span>
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
