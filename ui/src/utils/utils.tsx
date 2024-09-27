import React from 'react';
import { AccessRequest } from '../models/type';
import { Access_READ_COLOR, Access_WRITE_COLOR } from '../constant';

export const Spinner = ({ show, style = {} }: { show: boolean; style?: React.CSSProperties }) =>
  show ? (
    <span style={style}>
      <i className='fa fa-circle-notch fa-spin' style={{ color: '#0DADEA' }} />
    </span>
  ) : null;

export enum AccessRole {
  Read = 'Read',
  Write = 'Write'
}
export const AccessPanel = ({
                                  accessRequest
}: {
  accessRequest: AccessRequest;
}) => {
  let color = Access_READ_COLOR;
  let icon = 'fa-solid fa-lock';
  if (accessRequest && accessRequest?.permission === 'Write') {
    color = Access_WRITE_COLOR;
    icon = 'fa-solid fa-unlock';
  } else {
    color = Access_READ_COLOR;
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

const getRoleTitle = (accessRequest: AccessRequest) => {
  if (accessRequest && accessRequest.permission === 'Write') {
    return AccessRole.Write;
  } else {
    return AccessRole.Read;
  }
};
