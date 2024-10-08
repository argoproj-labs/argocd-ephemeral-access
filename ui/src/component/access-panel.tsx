import React, { useEffect, useState } from 'react';
import moment from 'moment';
import { AccessRequest, Application } from '../models/type';
import { ARGO_GRAY6_COLOR } from '../shared/colors';
import { HelpIcon } from 'argo-ui/src/components/help-icon/help-icon';
import { AccessPanel } from '../utils/utils';
import { TIME_FORMAT } from '../constant';

const DisplayAccessPermission: React.FC<{ application: Application }> = ({ application }) => {
  const [accessRequest, setAccessRequest] = useState<AccessRequest | null>(null);

  useEffect(() => {
    const checkPermission = () => {
      const storedPermission = localStorage.getItem(application.metadata?.name);
      if (storedPermission) {
        const parsedPermission = JSON.parse(storedPermission);
        const expiryTime = moment(parsedPermission.accessExpires, TIME_FORMAT);
        const diffInSeconds = expiryTime.diff(moment(), 'seconds');
        if (diffInSeconds <= 0) {
          localStorage.removeItem(application.metadata?.name);
          setAccessRequest(null);
        } else {
          setAccessRequest(parsedPermission);
        }
      } else {
        setAccessRequest(null);
      }
    };

    const intervalId = setInterval(checkPermission, 1000);
    return () => clearInterval(intervalId);
  }, [application.metadata?.name]);

  return application?.metadata?.labels &&
    window?.GLOBAL_ARGOCD_ACCESS_EXT_LABEL_KEY &&
    application?.metadata?.labels[window?.GLOBAL_ARGOCD_ACCESS_EXT_LABEL_KEY] ===
      window?.GLOBAL_ARGOCD_ACCESS_EXT_LABEL_VALUE ? (
    <div
      key='permission-status-icon'
      qe-id='permission-status-title'
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
          className='application-status-panel__item-value'
          onClick={() => (
            <link
              href={`/applications/argocd/testapp?view=tree&resource=&extension=ephemeral_access`}
            />
          )}
          style={{
            color: 'green',
            marginRight: '5px',
            position: 'relative',
            top: '2px',
            display: 'flex',
            alignItems: 'center',
            paddingTop: '10px',
            fontSize: '12px',
            fontFamily: 'inherit'
          }}
        >
          <div
            className={
              'application-status-panel__item-value application-status-panel__item-value--Succeeded'
            }
            style={{ alignItems: 'baseline' }}
          >
            <AccessPanel accessRequest={accessRequest} />
          </div>
        </div>

        {accessRequest?.expiresAt && (
          <div className={'application-status-panel__item-name'}>
            Expires: &nbsp;
            {moment(accessRequest.expiresAt, TIME_FORMAT).diff(moment(), 'seconds')} seconds
          </div>
        )}
      </div>
    </div>
  ) : null;
};

export default DisplayAccessPermission;
