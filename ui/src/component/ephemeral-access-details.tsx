import React, { useCallback, useEffect, useRef, useState } from 'react';
import { toast, ToastContainer } from 'react-toastify';
import 'react-toastify/dist/ReactToastify.css';
import { BUTTON_LABELS } from '../constant';
import { Application, UserInfo } from '../models/type';
import { getAccessRoles, Spinner } from '../utils/utils';
import EphemeralRoleSelection from './ephemeral-role-selection';
import './style.scss';
import moment from 'moment';
import {
  AccessRequestResponseBody,
  AccessRequestResponseBodyStatus,
  AllowedRoleResponseBody,
  createAccessrequest,
  CreateAccessRequestBody,
  listAccessrequest,
  ListAccessRequestResponseBody
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

  const [enableBtn, setEnableBtn] = useState<boolean>(true);
  const applicationNamespace = application?.metadata?.namespace || '';
  const applicationName = application?.metadata?.name || '';
  const project = application?.spec?.project || '';
  const username = userInfo?.username || '';
  const selectedRoleRef = useRef(selectedRole);
  const notify = (msg: string) => toast.warning('System message: ' + msg);

  const getUserRoles = useCallback(async () => {
    try {
      const usrRoles = await getAccessRoles(
        applicationName,
        applicationNamespace,
        project,
        username
      );
      setRoles(usrRoles || []);
      if ((usrRoles || []).length === 1) {
        setSelectedRole(usrRoles[0]?.roleName);
      }
    } catch (error) {
      setEnableBtn(false);
      notify('Failed to fetch roles: ' + error.message);
    }
  }, [applicationName, applicationNamespace, project, username]);

  function handleAccessExpiration(accessRequestData: AccessRequestResponseBody) {
    if (accessRequestData.expiresAt) {
      const timeoutDuration =
        moment.parseZone(accessRequestData.expiresAt).valueOf() - moment().valueOf();
      if (timeoutDuration > 0) {
        setTimeout(() => {
          setCurrentAccessRequest(null);
          setEnableBtn(false);
          setSelectedRole('');
          selectedRoleRef.current = '';
          localStorage.setItem(applicationName, 'null');
        }, timeoutDuration);
      }
    }
  }

  async function saveAccessRequest(data: ListAccessRequestResponseBody) {
    if (data.items.length === 0) {
      localStorage.setItem(applicationName, 'null');
      setEnableBtn(false);
      return null;
    }

    const accessRequestData = data.items[0];
    const grantedPermission = data.items.find(
      (item) => item.status === AccessRequestResponseBodyStatus.GRANTED
    );
    setCurrentAccessRequest(accessRequestData);
    if (accessRequestData.status === AccessRequestResponseBodyStatus.GRANTED) {
      setSelectedRole(accessRequestData.role);
    }
    localStorage.setItem(applicationName, JSON.stringify(grantedPermission || null));

    switch (accessRequestData?.status) {
      case AccessRequestResponseBodyStatus.GRANTED:
        break;
      case AccessRequestResponseBodyStatus.DENIED:
        notify('Last request was denied: ' + accessRequestData?.message + '. Please try again!');
        setCurrentAccessRequest(null);
        break;
      case AccessRequestResponseBodyStatus.REQUESTED:
        break;
      default:
        break;
    }

    if (
      accessRequestData.status === AccessRequestResponseBodyStatus.GRANTED ||
      accessRequestData.status === AccessRequestResponseBodyStatus.DENIED
    ) {
      handleAccessExpiration(accessRequestData);
    }
    return accessRequestData;
  }

  const getUserAccess = useCallback(async (): Promise<AccessRequestResponseBody | null> => {
    try {
      const { data } = await listAccessrequest({
        headers: getHeaders({ applicationName, applicationNamespace, project, username })
      });
      return await saveAccessRequest(data);
    } catch (error) {
      return null;
    }
  }, [applicationName, applicationNamespace, project, username]);

  const submitAccessRequest = async (): Promise<CreateAccessRequestBody | null> => {
    try {
      if (!selectedRoleRef.current && roles.length > 1) {
        setErrorMessage('Please select a role');
        return null;
      }

      setEnableBtn(false);
      const roleName = selectedRoleRef.current || (roles.length > 0 ? roles[0].roleName : '');

      await createAccessrequest(
        { roleName: roleName },
        {
          headers: getHeaders({ applicationName, applicationNamespace, project, username })
        }
      );

      // start polling for access request status
      const intervalId = setInterval(async () => {
        const updatedAccessData = await getUserAccess();
        if (
          updatedAccessData &&
          (updatedAccessData.status === AccessRequestResponseBodyStatus.GRANTED ||
            updatedAccessData.status === AccessRequestResponseBodyStatus.DENIED)
        ) {
          handleAccessExpiration(updatedAccessData);
          clearInterval(intervalId);
        }
      }, 200);

      setEnableBtn(true);
      return { roleName: selectedRoleRef.current || roles[0].roleName };
    } catch (error) {
      setEnableBtn(true);
      returnError(error);
      return null;
    }
  };

  const returnError = async (error: any) => {
    const status = error?.response?.status;

    switch (status) {
      case 409:
        notify(`${selectedRole} role: A permission request already exists.`);
        const accessData = await getUserAccess();
        if (
          accessData?.status === AccessRequestResponseBodyStatus.GRANTED ||
          accessData?.status === AccessRequestResponseBodyStatus.DENIED
        ) {
          setCurrentAccessRequest(accessData);
        }
        break;
      case 401:
        notify(`Unauthorized request: ${error.message}`);
        break;
      case 403:
        notify('Access Request Denied: No valid role was found. Please verify your permissions.');
        break;
      default:
        notify(`Error occurred while requesting permission: ${error.message}`);
        break;
    }
  };

  const options: SelectOption[] = roles.map((role) => ({
    value: role.roleName,
    title: role.roleDisplayName
  }));

  const selectRoleChange = (selectedOption: SelectOption) => {
    if (selectedOption.value != '') {
      setErrorMessage('');
    } else {
      setErrorMessage('Please select a role');
    }
    selectedRoleRef.current = selectedOption.value;
    setSelectedRole(selectedOption.value);
    setEnableBtn(false);
  };

  useEffect(() => {
    getUserRoles();
  }, []);

  useEffect(() => {
    getUserAccess();
  }, []);

  return (
    <div className='access-form'>
      <div className=''>
        <button
          style={{ position: 'relative', minWidth: '120px', minHeight: '20px' }}
          className='argo-button argo-button--base'
          onClick={submitAccessRequest}
          disabled={enableBtn}
        >
          {enableBtn && (
            <span>
              <Spinner
                show={currentAccessRequest?.status === AccessRequestResponseBodyStatus.REQUESTED}
                style={{ marginRight: '5px' }}
              />{' '}
            </span>
          )}
          {BUTTON_LABELS.REQUEST_ACCESS}
        </button>
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
            <div className='columns small-9'>{userInfo?.username?.toUpperCase()}</div>
          </div>
          <div className='row white-box__details-row'>
            <div className='columns small-3'>PERMISSION</div>
            <div className='columns small-9'>{currentAccessRequest?.permission || 'Read Only'}</div>
          </div>
          {currentAccessRequest?.status === AccessRequestResponseBodyStatus.GRANTED && (
            <div>
              <div className='row white-box__details-row'>
                <div className='columns small-3'>ROLE</div>
                <div className='columns small-9'>{currentAccessRequest?.role}</div>
              </div>
              <div className='row white-box__details-row'>
                <div className='columns small-3'>STATUS</div>
                <div className='columns small-9'>{currentAccessRequest?.status}</div>
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
                <div className='columns small-9'>
                  {currentAccessRequest?.status === AccessRequestResponseBodyStatus.GRANTED ? (
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
