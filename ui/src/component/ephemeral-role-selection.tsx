import React from 'react';
import { Select, SelectOption } from 'argo-ui/src/components/select/select';
import { PermissionRole } from '../utils/utils';

interface RoleSelectionProps {
  selectedRole: string;
  options: SelectOption[];
  selectRoleChange: (e: SelectOption) => void;
}

const EphemeralRoleSelection: React.FC<RoleSelectionProps> = ({
  selectedRole,
  options,
  selectRoleChange
}) => {
  const getOptions = () => {
    return (
      <div>
        <p>{PermissionRole.PERMISSION_REQUEST}</p>
        <div className='row white-box__details-row'>
          <div className='columns small-3 access-form__label'>
            {PermissionRole.REQUEST_ROLE_LABEL}
          </div>
          <div className='access-form__select'>
            <Select
              id='role-select'
              value={selectedRole}
              options={options}
              onChange={selectRoleChange}
              placeholder='Select Role'
            />
          </div>
        </div>
      </div>
    );
  };
  return getOptions();
};

export default EphemeralRoleSelection;