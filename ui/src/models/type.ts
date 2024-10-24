export interface UserInfo {
  loggedIn: boolean;
  username: string;
  iss: string;
  groups: string[];
}

export interface AccessRequest {
  userId: string;
  permission: 'ReadOnly' | 'Write';
  requestedAt?: Date;
  role?: string;
  status?: 'PENDING' | 'ACTIVE' | 'INACTIVE' | 'DENIED' | 'EXPIRED';
  message?: string;
  expiresAt?: Date;
}

export interface Application {
  spec: {
    destination: {
      namespace: string;
    };

    project: string;
  };
  metadata: {
    name: string;
    labels: any
    namespace: string;
  };
}


