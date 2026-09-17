/** Workspace access returned by the server is the module visibility authority.
 * No staff email allowlist is shipped in either edition. */
export function isModuleEnabled(_moduleId: string, _userEmail?: string): boolean {
  return true;
}

// Coordinate with the backend SETUP_SUCCESS_ENABLED flag to avoid exposing a route the API has disabled.
export function isSetupSuccessEnabled(): boolean {
  return import.meta.env.VITE_SETUP_SUCCESS_ENABLED === 'true';
}
