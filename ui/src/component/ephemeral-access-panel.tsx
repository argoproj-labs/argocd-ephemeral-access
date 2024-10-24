import React, { useEffect, useState, useCallback } from 'react';
import moment from 'moment';
import { Application } from '../models/type';
import { ARGO_GRAY6_COLOR } from '../shared/colors';
import { HelpIcon } from 'argo-ui/src/components/help-icon/help-icon';
import { AccessPanel, EnableEphemeralAccess } from '../utils/utils';
import { AccessRequestResponseBody } from '../gen/ephemeralAccessAPI';
import { getDisplayTime } from '../utils/utils';
const DisplayAccessPermission: React.FC<{ application: Application }> = ({ application }) => {
  const [accessRequest, setAccessRequest] = useState<AccessRequestResponseBody | null>(null);

  const checkPermission = useCallback(() => {
    const accessPermission = JSON.parse(localStorage.getItem(application.metadata?.name));

    if (accessPermission) {
      const expiryTime = moment.parseZone(accessPermission.expiresAt);
      setAccessRequest(accessPermission);
      const diffInSeconds = expiryTime.diff(moment(), 'seconds');

      if (diffInSeconds <= 0) {
        // Access expired, remove from local storage and set to null
        localStorage.removeItem(application.metadata?.name);
        setAccessRequest(null);
      } else {
        setAccessRequest(accessPermission);
      }
    }
  }, [application.metadata?.name]);

  useEffect(() => {
    const intervalId = setInterval(checkPermission, 500);
    return () => clearInterval(intervalId);
  }, [checkPermission]);

  const handleLinkClick = useCallback(() => {
    window.location.href = `/applications/argocd/${application.metadata.name}?view=tree&resource=&extension=ephemeral_access`;
  }, []);

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
          onClick={handleLinkClick}
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
          <div
            className={
              'application-status-panel__item-value'
            }
            style={{ alignItems: 'baseline' }}
          >
            <AccessPanel accessRequest={accessRequest} />
          </div>
        </div>

        {accessRequest?.expiresAt && (
          <div className={'application-status-panel__item-name'}>
            Expires In: &nbsp;
            {getDisplayTime(accessRequest)}
          </div>
        )}
      </div>
    </div>
  ) : null;
};

export default DisplayAccessPermission;
