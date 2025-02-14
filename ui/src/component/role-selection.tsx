import React from 'react';
import { Select, SelectOption } from 'argo-ui/src/components/select/select';

interface RoleSelectionProps {
  selectedRole: string;
  options: SelectOption[];
  selectRoleChange: (e: SelectOption) => void;
}

const RoleSelection: React.FC<RoleSelectionProps> = ({
  selectedRole,
  options,
  selectRoleChange
}) => {
  const getOptions = () => {
    return (
      <div className='access-form__role-select'>
        <Select
          id='role-select'
          value={selectedRole}
          options={options}
          onChange={selectRoleChange}
          placeholder='Select Role'
        />
      </div>
    );
  };
  return getOptions();
};

export default RoleSelection;
