import React, { useEffect, useState } from 'react';
import moment from 'moment';
import Moment from 'react-moment';
import { Application } from '../models/type';
import { ARGO_GRAY6_COLOR } from '../shared/colors';
import { HelpIcon } from 'argo-ui/src/components/help-icon/help-icon';
import { EnableEphemeralAccess, getDefaultDisplayAccessRole } from '../utils/utils';
import {
  AccessRequestResponseBody,
  AccessRequestResponseBodyStatus,
  listAccessrequest
} from '../gen/ephemeral-access-api';
import { ACCESS_DEFAULT_COLOR, ACCESS_PERMISSION_COLOR } from '../constant';
import { getHeaders } from '../config/client';
const DisplayAccessPermission: React.FC<{ application: Application }> = ({ application }) => {
  const [accessRequest, setAccessRequest] = useState<AccessRequestResponseBody | null>(null);
  const [appStorage, setAppStorage] = useState<string | null>(null);

  const applicationNamespace = application?.metadata?.namespace || '';
  const applicationName = application?.metadata?.name || '';
  const project = application?.spec?.project || '';

  useEffect(() => {
    const handleStorageChange = () => {
      const storedValue = localStorage.getItem(applicationName);
      setAppStorage(storedValue);
    };
    window.addEventListener('storage', handleStorageChange);
    handleStorageChange();
    return () => {
      window.removeEventListener('storage', handleStorageChange);
    };
  }, [applicationName]);

  useEffect(() => {
    const pollAccessRequest = async () => {
      try {
        console.log('Polling access request');
        const accessPermission = JSON.parse(
          localStorage.getItem(applicationName)
        ) as AccessRequestResponseBody;
        const currStatus = accessPermission?.status;
        if (
          accessPermission &&
          (currStatus === undefined ||
            currStatus=== AccessRequestResponseBodyStatus.REQUESTED ||
            currStatus === AccessRequestResponseBodyStatus.INITIATED)
        ) {
          const { data } = await listAccessrequest({
            headers: getHeaders({ applicationName, applicationNamespace, project })
          });

          const accessRequestData: AccessRequestResponseBody | null =
            data.items.length > 0 ? data.items[0] : null;

          if (accessRequestData) {
            const nextStatus = accessRequestData.status;
            if (
              nextStatus === AccessRequestResponseBodyStatus.GRANTED ||
              nextStatus === AccessRequestResponseBodyStatus.DENIED
            ) {
              setAccessRequest(accessRequestData);
              const expiryTime = moment.parseZone(accessRequestData?.expiresAt);

              const diffInSeconds = expiryTime.diff(moment(), 'seconds');
              if (diffInSeconds <= 0) {
                clearInterval(intervalId);
                setAccessRequest(null);
              }
            }
          }
        }
      } catch (error) {
        console.error('Error polling access request:', error);
      }
    };
    const intervalId = setInterval(pollAccessRequest, 3000);
    return () => clearInterval(intervalId);
  }, [applicationName, applicationNamespace, project, appStorage]);

  const AccessPanel = ({ accessRequest }: { accessRequest: AccessRequestResponseBody }) => {
    let color = ACCESS_DEFAULT_COLOR;
    let icon = 'fa-solid fa-lock';
    if (accessRequest) {
      color = ACCESS_PERMISSION_COLOR;
      icon = 'fa-solid fa-unlock';
    } else {
      color = ACCESS_DEFAULT_COLOR;
      icon = 'fa-solid fa-lock';
    }

    return (
      <React.Fragment>
        <i
          qe-id='Access-role-title'
          title={getRoleTitle(accessRequest)}
          className={'fa ' + icon}
          style={{ color, minWidth: '15px', textAlign: 'center' }}
        />{' '}
        &nbsp;
        {getRoleTitle(accessRequest)}
      </React.Fragment>
    );
  };

  const getRoleTitle = (accessRequest: AccessRequestResponseBody) => {
    if (accessRequest === null) {
      return getDefaultDisplayAccessRole();
    } else {
      return accessRequest.permission;
    }
  };

  return EnableEphemeralAccess(application) ? (
    <div
      key='ephemeral-access-status-icon'
      qe-id='ephemeral-access-status-title'
      className='application-status-panel__item'
    >
      <label
        style={{
          fontSize: '12px',
          fontWeight: 600,
          color: ARGO_GRAY6_COLOR,
          display: 'flex',
          alignItems: 'center',
          marginBottom: '0.5em'
        }}
      >
        CURRENT PERMISSION &nbsp;
        {<HelpIcon title={'user current permissions'} />}
      </label>
      <div style={{ display: 'flex', flexDirection: 'column', alignItems: 'flex-start' }}>
        <div
          style={{
            marginRight: '5px',
            position: 'relative',
            top: '2px',
            display: 'flex',
            alignItems: 'center',
            paddingTop: '2px',
            fontFamily: 'inherit'
          }}
        >
          <div className={'application-status-panel__item-value'} style={{ marginBottom: '0.5em' }}>
            <AccessPanel accessRequest={accessRequest} />
          </div>
        </div>

        {accessRequest?.expiresAt && (
          <div className={'application-status-panel__item-name'}>
            Expires In: &nbsp;
            <>
              <Moment fromNow ago>
                {new Date(accessRequest.expiresAt)}
              </Moment>
            </>
          </div>
        )}
      </div>
    </div>
  ) : null;
};

export default DisplayAccessPermission;
