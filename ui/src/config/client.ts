import { UserInfo, Application } from '../models/type';

export async function getAccess(application: Application, username: string) {
  const applicationNamespace = application?.spec?.destination?.namespace || '';
  const applicationName = application?.metadata?.name || '';
  const project = application?.spec?.project || '';

  const url = `/extensions/access/accessrequests`;
  return fetch(url, {
    headers: getHeaders({ applicationName, applicationNamespace, project, username })
  })
    .then((response) => {
      return response.json();
    })
    .catch((err) => {
      return {};
    });
}

export async function requestAccess(application: Application, username: string) {
  const applicationNamespace = application?.spec?.destination?.namespace || '';
  const applicationName = application?.metadata?.name || '';
  const project = application?.spec?.project || '';
  const url = `/extensions/access/accessrequests`;
  const argocdApplicationName = `${applicationNamespace}:${applicationName}`;

  try {
    return await fetch(url, {
      method: 'POST',
      headers: getHeaders({ applicationName, applicationNamespace, project, username }),
      body: JSON.stringify({ appName: argocdApplicationName, username: username })
    }).then((response) => {
      return response.json();
    });
  } catch (err) {
    console.error('Error updating Access:', err);
    throw err;
  }
}

//Creates and returns the custom headers needed for the argocd extensions.
export function getHeaders({
  applicationName,
  applicationNamespace,
  project,
  username
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
    'Argocd-Application-Name': `${argocdApplicationName}`,
    'Argocd-Project-Name': `${project}`,
    'Argocd-Username': `${username}`,
    'Argocd-User-Groups': 'dummy'
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
    console.error('Error fetching user info:', err);
    return null;
  }
}
