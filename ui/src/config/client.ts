import { UserInfo } from '../models/type';


//Creates and returns the custom headers needed for the argocd extensions.
export function getHeaders({
  applicationName,
  applicationNamespace,
  project,
}: {
  applicationName: string;
  applicationNamespace: string;
  project: string;
  username?: string;
}) {
  const argocdApplicationName = `${applicationNamespace}:${applicationName}`;
  return {
    'cache-control': 'no-cache',
    'Content-Type': 'application/json',
    "Argocd-Application-Name": `${argocdApplicationName}`,
    "Argocd-Project-Name": `${project}`,
  };
}

export async function getUserInfo(application: any): Promise<UserInfo> {
  const applicationNamespace = application?.spec?.destination?.namespace || '';
  const applicationName = application?.metadata?.name || '';
  const project = application?.spec?.project || '';
  const url = '/api/v1/session/userinfo';
  try {
    const response = await fetch(url, {
      headers: getHeaders({ applicationName, applicationNamespace, project })
    });
    return await response.json();
  } catch (err) {
    return null;
  }
}
