import React, { useEffect, useState } from "react";
import AccessDetails from "./component/access-details";
import { getUserInfo } from "./config/client";
import { UserInfo } from "./models/type";

interface Application {
  metadata: {
    labels: {
      [key: string]: string;
    };
  };
}

export const RequestAccessBtn = () => {
  return (
    <div qe-id="ext-access">
      <span className="flex items-center font-semibold justify-center text-center p-4">
        Permission
      </span>
    </div>
  );
};

export const ShowDeployBtn = (application: Application) => {
  return (
    application?.metadata?.labels &&
    window?.GLOBAL_ARGOCD_ACCESS_EXT_LABEL_KEY &&
    application?.metadata?.labels[window?.GLOBAL_ARGOCD_ACCESS_EXT_LABEL_KEY] ===
    window?.GLOBAL_ARGOCD_ACCESS_EXT_LABEL_VALUE
  );
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

  return <>{userInfo && <AccessDetails application={application} userInfo={userInfo} />}</>;
};