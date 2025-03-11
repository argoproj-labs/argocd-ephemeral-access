import React, { useCallback, useEffect, useRef, useState } from 'react';
import { toast, ToastContainer } from 'react-toastify';
import 'react-toastify/dist/ReactToastify.css';
import { BUTTON_LABELS } from '../constant';
import { Application, UserInfo } from '../models/type';
import { getAccessRoles, getDisplayTime, getDisplayValue, Spinner } from "../utils/utils";
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

  const returnError = async (error: any) => {
    const status = error?.response?.status;
   setIsLoading(false);
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

  const handleAccessExpiration = (accessData: AccessRequestResponseBody) => {
    if (accessData.expiresAt) {
      const timeoutDuration = moment.parseZone(accessData.expiresAt).valueOf() - moment().valueOf();
      if (timeoutDuration > 0) {
        setTimeout(() => {
          setCurrentAccessRequest(null);
          setIsLoading(false);
          setSelectedRole('');
          selectedRoleRef.current = '';
          localStorage.setItem(applicationName, 'null');
        }, timeoutDuration);
      }
    }
  };

  const AccessRoles = useCallback(async () => {
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
  }, [applicationName, applicationNamespace, project, username]);

  const fetchAccessRequest = useCallback(async () => {
    let currentDelay = 300;
    const maxDelay = 45000;
    // 120 seconds max polling duration
    const maxPollingDuration = 120000;

    const pollingEndTime = Date.now() + maxPollingDuration;

    const poll = async () => {
      try {
        const { data } = await listAccessrequest({
          headers: getHeaders({ applicationName, applicationNamespace, project, username })
        });
        if (data.items.length > 0) {
          const accessRequestData = data.items[0];
          setCurrentAccessRequest(accessRequestData);
          const status = accessRequestData?.status;

          if (status === AccessRequestResponseBodyStatus.GRANTED) {
            localStorage.setItem(applicationName, JSON.stringify(accessRequestData || null));
            handleAccessExpiration(accessRequestData);
            return;
          } else if (
            accessRequestData.status === undefined ||
            status === AccessRequestResponseBodyStatus.REQUESTED
          ) {
            if (Date.now() < pollingEndTime) {
              currentDelay = Math.min(currentDelay * 2, maxDelay);
              setTimeout(poll, currentDelay);
            } else {
              setIsLoading(false);
              const errorObject = {
                response: {
                  status: 408
                },
                message: 'Check the status of the change request!'
              };
              returnError(errorObject);
            }
          } else if (
            status === AccessRequestResponseBodyStatus.DENIED ||
            status === AccessRequestResponseBodyStatus.INVALID
          ) {
            return;
          }
        } else {
          localStorage.setItem(applicationName, 'null');
        }
      } catch (error) {
        setTimeout(poll, currentDelay);
        returnError(error);
      }
    };
    poll();
  }, [applicationName, applicationNamespace, project, username]);

  const submitAccessRequest = async () => {
    try {
      if (!selectedRoleRef.current && roles.length > 1) {
        setIsLoading(false);
        setErrorMessage('Please select a role');
        return;
      }
      const roleName = selectedRoleRef.current || (roles.length > 0 ? roles[0].roleName : '');
      setIsLoading(true);
      await createAccessrequest(
        { roleName },
        {
          headers: getHeaders({ applicationName, applicationNamespace, project, username })
        }
      );

      fetchAccessRequest();
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
    setIsLoading(false);
  };

  useEffect(() => {
    AccessRoles();
    fetchAccessRequest();
  }, []);

  return (
    <div className='access-form'>
      <div className=''>
        <button
          style={{ position: 'relative', minWidth: '120px', minHeight: '20px' }}
          className='argo-button argo-button--base'
          onClick={() => {
            submitAccessRequest();
          }}
          disabled={isLoading}
        >
          {isLoading && (
            <span>
              <Spinner
                show={currentAccessRequest?.status === AccessRequestResponseBodyStatus.REQUESTED}
                style={{ marginRight: '5px' }}
              />{' '}
            </span>
          )}
          {BUTTON_LABELS.REQUEST_ACCESS}
        </button>

        {isLoading && currentAccessRequest?.status === AccessRequestResponseBodyStatus.REQUESTED && (
          <div className='access-form__error-msg'> Check the status of the change request</div>
        )}
      </div>
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
            <div className='columns small-9'>{getDisplayValue(currentAccessRequest?.permission) || 'read only'}</div>
          </div>
          {currentAccessRequest && (
            <div>
              <div className='row white-box__details-row'>
                <div className='columns small-3'>ROLE</div>
                <div className='columns small-9'>{getDisplayValue(currentAccessRequest?.role)}</div>
              </div>
              <div className='row white-box__details-row'>
                <div className='columns small-3'>STATUS</div>
                <div className='columns small-9'>{getDisplayValue(currentAccessRequest?.status)}</div>
              </div>
              <div className='row white-box__details-row'>
                <div className='columns small-3'>REQUESTED-AT</div>
                <div className='columns small-9'>
                  {getDisplayTime(currentAccessRequest?.requestedAt)}
                </div>
              </div>

              {currentAccessRequest?.expiresAt && (
                <div
                  className='row white-box__details-row'
                  style={{ display: 'flex', alignItems: 'center' }}
                >
                  <div className='columns small-3'>EXPIRES</div>
                  <div className='columns small-9'>
                    {moment(currentAccessRequest?.expiresAt).format('MMMM Do YYYY, h:mm:ss a')}
                  </div>
                </div>
              )}
              <div className='row white-box__details-row'>
                <div className='columns small-3'>MESSAGE</div>
                <div className='columns small-9' style={{ lineHeight: '1.75' }}>
                  {currentAccessRequest?.status === AccessRequestResponseBodyStatus.REQUESTED ? (
                    <span style={{ display: 'flex', flexDirection: 'column', margin: '0' }}>
                      {currentAccessRequest?.message}
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
                    currentAccessRequest?.message
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
