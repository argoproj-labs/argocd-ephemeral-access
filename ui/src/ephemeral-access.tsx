import React, { useEffect, useState } from 'react';
import EphemeralAccessDetails from './component/ephemeral-access-details';
import { getUserInfo } from './config/client';
import { Application, UserInfo } from './models/type';
import { EnableEphemeralAccess } from './utils/utils';

export const RequestAccessBtn = () => {
  return (
    <div className="show-for-large" qe-id='ext-access'>
      <i className="fa-solid fa-lock" style={{ marginLeft: '-5px', marginRight: '5px' }}></i>
      <span style={{paddingLeft: '2px'}}>Permission</span>
    </div>
  );
};

export const ShowDeployBtn = (application: Application) => {
  return EnableEphemeralAccess(application);
};

interface RequestAccessBtnFlyoutProps {
  application: any;
}

export const RequestAccessBtnFlyout = ({ application }: RequestAccessBtnFlyoutProps) => {
  const [userInfo, setUserInfo] = useState<UserInfo | null>(null);

  useEffect(() => {
    if (!application) return;

    const fetchUserInfo = async () => {
      const info = await getUserInfo(application);
      setUserInfo(info);
    };

    fetchUserInfo();
  }, [application]);

  return (
    <>{userInfo && <EphemeralAccessDetails application={application} userInfo={userInfo} />}</>
  );
};
