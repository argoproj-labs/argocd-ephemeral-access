import { RequestAccessBtnFlyout, RequestAccessBtn, ShowDeployBtn } from './ephemeral-access';
import DisplayAccessPermission from './component/ephemeral-access-panel';

const PERMISSION_TITLE = 'Ephemeral Access';
const PERMISSION_ID = 'ephemeral_access';
const DISPLAY_PERMISSION_TITLE = 'Display_Ephemeral Access';
const DISPLAY_PERMISSION_ID = 'display_ephemeral_access';

function initializeExtensions(window: any) {
  window.extensionsAPI = window.extensionsAPI || {};

  window.extensionsAPI.registerStatusPanelExtension(
    DisplayAccessPermission,
    DISPLAY_PERMISSION_TITLE,
    DISPLAY_PERMISSION_ID
  );

  window.extensionsAPI.registerTopBarActionMenuExt(
    RequestAccessBtn,
    PERMISSION_TITLE,
    PERMISSION_ID,
    RequestAccessBtnFlyout,
    ShowDeployBtn,
    '',
    true
  );
}

// Entry point
((window: any) => {
  initializeExtensions(window);
})(window);
