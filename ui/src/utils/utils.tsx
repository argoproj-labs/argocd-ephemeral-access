import React from 'react';
import Moment from 'react-moment';
import { Application } from '../models/type';
import { ACCESS_DEFAULT_COLOR, ACCESS_PERMISSION_COLOR } from '../constant';
import { AccessRequestResponseBody } from '../gen/ephemeralAccessAPI';

export const Spinner = ({ show, style = {} }: { show: boolean; style?: React.CSSProperties }) =>
  show ? (
    <span style={style}>
      <i className='fa fa-circle-notch fa-spin' style={{ color: '#0DADEA' }} />
    </span>
  ) : null;

export enum AccessRole {
  DEFAULT_ACCESS = 'Read'
}
export const AccessPanel = ({ accessRequest }: { accessRequest: AccessRequestResponseBody }) => {
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
    return AccessRole.DEFAULT_ACCESS;
  } else {
    return accessRequest.permission;
  }
};

export const getDisplayTime = (accessRequest: AccessRequestResponseBody): any => {
  return (
    <span>
      <Moment fromNow={true} ago={true}>
        {new Date(accessRequest.expiresAt)}
      </Moment>
    </span>
  );
};

export const EnableEphemeralAccess = (application: Application) => {
  if (window?.EPHEMERAL_ACCESS_VARS) {
    return (
      application?.metadata?.labels &&
      window?.EPHEMERAL_ACCESS_VARS?.EPHEMERAL_ACCESS_LABEL_KEY &&
      application?.metadata?.labels[window?.EPHEMERAL_ACCESS_VARS?.EPHEMERAL_ACCESS_LABEL_KEY] ===
        window?.EPHEMERAL_ACCESS_VARS?.EPHEMERAL_ACCESS_LABEL_VALUE
    );
  }
  return true;
};
