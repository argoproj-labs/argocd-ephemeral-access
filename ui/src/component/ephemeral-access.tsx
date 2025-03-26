import React, { useEffect, useState } from 'react';
import EphemeralAccessDetails from './ephemeral-access-details';
import { getUserInfo } from '../config/client';
import { Application, UserInfo } from '../models/type';
import { EnableEphemeralAccess } from '../utils/utils';

export const PermissionBtnComponent = () => {
  return (
    <div className='show-for-large' qe-id='ext-access'>
      <i
        className='fa-solid fa-lock'
        style={{ marginLeft: '-5px', marginRight: '5px' }}
        aria-hidden='true'
      />
      <span style={{ paddingLeft: '2px' }}>Permission</span>
    </div>
  );
};

export const ShowPermissionBtn = (application: Application) => {
  return EnableEphemeralAccess(application);
};

interface RequestAccessBtnFlyoutProps {
  application: any;
}

export const PermissionBtnFlyout = ({ application }: RequestAccessBtnFlyoutProps) => {
  const [userInfo, setUserInfo] = useState<UserInfo | null>(null);

  useEffect(() => {
    if (!application) return;

    const fetchUserInfo = async () => {
      const info = await getUserInfo(application);
      setUserInfo(info);
    };

    fetchUserInfo();
  }, []);

  return (
    <>{userInfo && <EphemeralAccessDetails application={application} userInfo={userInfo} />}</>
  );
};
