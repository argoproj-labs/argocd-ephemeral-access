import React, { useEffect, useState, useRef } from 'react';
import Moment from 'react-moment';
import { Application } from '../models/type';
import { ARGO_GRAY6_COLOR } from '../shared/colors';
import { HelpIcon } from 'argo-ui/src/components/help-icon/help-icon';
import { EnableEphemeralAccess, getDefaultDisplayAccessRole } from '../utils/utils';
import moment from 'moment';
import {
  AccessRequestResponseBody,
  AccessRequestResponseBodyStatus,
  listAccessrequest
} from '../gen/ephemeral-access-api';
import { ACCESS_DEFAULT_COLOR, ACCESS_PERMISSION_COLOR } from '../constant';
import { getHeaders } from '../config/client';
const DisplayAccessPermission: React.FC<{ application: Application }> = ({ application }) => {
  const [accessRequest, setAccessRequest] = useState<AccessRequestResponseBody | null>(null);
  const [startPoll, setStartPoll] = useState(true);

  const pollTimeoutRef = useRef<NodeJS.Timeout | null>(null);

  const applicationNamespace = application?.metadata?.namespace || '';
  const applicationName = application?.metadata?.name || '';
  const project = application?.spec?.project || '';
  const POLLING_INTERVAL_MS = 300;
  const maxInterval = 30000;
  const showAccessPanel = EnableEphemeralAccess(application);

  useEffect(() => {
    const checkStorage = () => {
      const storedValue =
        (JSON.parse(localStorage.getItem(applicationName)) as AccessRequestResponseBody) || null;
      if (storedValue && storedValue.status !== accessRequest?.status) {
        setAccessRequest(storedValue);
        if (storedValue.status === AccessRequestResponseBodyStatus.GRANTED) {
          scheduleExpiryCheck(storedValue?.expiresAt);
        }
      } else {
        setAccessRequest(null);
      }
    };
    const intervalId = setInterval(checkStorage, POLLING_INTERVAL_MS);
    checkStorage();
    return () => {
      clearInterval(intervalId);
    };
  }, [applicationName]);

  useEffect(() => {
    let interval = POLLING_INTERVAL_MS;
    const cancel = new AbortController();

    const fetchAccessRequest = async (): Promise<void> => {
      if (!startPoll) return;
      try {
        const { data } = await listAccessrequest({
          headers: getHeaders({ applicationName, applicationNamespace, project }),
          signal: cancel.signal
        });

        const accessRequestData = data.items.find(
          (accessRequest: AccessRequestResponseBody) =>
            accessRequest.status === AccessRequestResponseBodyStatus.GRANTED
        );

        if (accessRequestData) {
          setAccessRequest(accessRequestData);
          localStorage.setItem(applicationName, JSON.stringify(accessRequestData));
          setStartPoll(false);
        } else if (
          data.items.some(
            (accessRequest: AccessRequestResponseBody) =>
              accessRequest.status === AccessRequestResponseBodyStatus.REQUESTED ||
              accessRequest.status === AccessRequestResponseBodyStatus.INVALID
          )
        ) {
          pollTimeoutRef.current = setTimeout(fetchAccessRequest, interval);
          interval = Math.min(interval * 2, maxInterval);
        } else {
          setAccessRequest(null);
          setStartPoll(false);
        }
      } catch (error) {
        return;
      }
    };
    fetchAccessRequest().then((r) => r);

    return () => {
      cancel.abort();
      if (pollTimeoutRef.current) {
        clearTimeout(pollTimeoutRef.current);
      }
    };
  }, [applicationName, accessRequest]);

  const scheduleExpiryCheck = (expiresAt: string | undefined) => {
    if (!expiresAt) return;
    const expiryTime = moment.parseZone(expiresAt);
    const diffInSeconds = expiryTime.diff(moment(), 'seconds');
    if (diffInSeconds <= 0) {
      setAccessRequest(null);
      localStorage.setItem(applicationName, 'null');
      setStartPoll(false);
    }
  };

  const AccessPanel = ({ accessRequest }: { accessRequest: AccessRequestResponseBody }) => {
    let color: string;
    let icon: string;
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
          aria-hidden='true'
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

  return showAccessPanel ? (
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
