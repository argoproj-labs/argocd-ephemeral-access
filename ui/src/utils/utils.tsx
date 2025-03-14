import React from 'react';
import { Application } from '../models/type';
import { AllowedRoleResponseBody, listAllowedroles } from '../gen/ephemeral-access-api';
import { getHeaders } from '../config/client';
import moment from "moment/moment";

export enum PermissionRole {
  DEFAULT_DISPLAY_ACCESS = 'Read',
  PERMISSION_REQUEST = 'PERMISSION REQUEST',
  REQUEST_ROLE_LABEL = 'REQUEST ROLE'
}

export const Spinner = ({ show, style = {} }: { show: boolean; style?: React.CSSProperties }) =>
  show ? (
    <span style={style}>
      <i className='fa fa-circle-notch fa-spin' style={{ color: '#0DADEA' }} />
    </span>
  ) : null;

export const getDefaultDisplayAccessRole = (): string => {
  return (
    window?.EPHEMERAL_ACCESS_VARS?.EPHEMERAL_ACCESS_DEFAULT_DISPLAY_ACCESS ||
    PermissionRole.DEFAULT_DISPLAY_ACCESS
  );
};

export const EnableEphemeralAccess = (application: Application) => {
  if (
    window?.EPHEMERAL_ACCESS_VARS === undefined ||
    window?.EPHEMERAL_ACCESS_VARS?.EPHEMERAL_ACCESS_LABEL_KEY === undefined ||
    window?.EPHEMERAL_ACCESS_VARS?.EPHEMERAL_ACCESS_LABEL_VALUE === undefined
  ) {
    return true;
  }

  return (
    application?.metadata?.labels &&
    window?.EPHEMERAL_ACCESS_VARS?.EPHEMERAL_ACCESS_LABEL_KEY &&
    application?.metadata?.labels[window?.EPHEMERAL_ACCESS_VARS?.EPHEMERAL_ACCESS_LABEL_KEY] ===
      window?.EPHEMERAL_ACCESS_VARS?.EPHEMERAL_ACCESS_LABEL_VALUE
  );
};

export const getAccessRoles = async (
  applicationName: string,
  applicationNamespace: string,
  project: string,
  username: string
): Promise<AllowedRoleResponseBody[]> => {
  const defaultRole = window?.EPHEMERAL_ACCESS_VARS?.EPHEMERAL_ACCESS_DEFAULT_TARGET_ROLE;
  const defaultDisplayRole = window?.EPHEMERAL_ACCESS_VARS?.EPHEMERAL_ACCESS_DEFAULT_DISPLAY_ACCESS;
  if (defaultRole) {
    return [
      { roleName: defaultRole, roleDisplayName: defaultDisplayRole }
    ] as unknown as AllowedRoleResponseBody[];
  } else {
    try {
      const response = await listAllowedroles({
        headers: getHeaders({ applicationName, applicationNamespace, project, username })
      });
      return response.data.items;
    } catch (error) {
      throw new Error(`Failed to get allowed roles: ${error}`);
    }
  }
};

export function getDisplayTime(dateStr: string): string {
  const date = moment.utc(dateStr);

  if (!date.isValid()) {
    return '';
  }

  return date.local().format('MMMM Do YYYY, h:mm:ss a');
}

export function getDisplayValue(value: string) {
  if (value === '' || value === null || value === undefined) {
    return '';
  }
  return value.toLowerCase();
}
