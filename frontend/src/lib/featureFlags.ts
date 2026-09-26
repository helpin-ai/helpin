import type { AuthConfig } from '@/lib/services/authService';

/** Workspace access returned by the server is the module visibility authority.
 * No staff email allowlist is shipped in either edition. */
export function isModuleEnabled(_moduleId: string, _userEmail?: string): boolean {
  return true;
}

/**
 * The API reports whether it serves the workspace Setup guide
 * (`setup_guide_enabled` from /auth/config; on by default in Community).
 * The build-time VITE_SETUP_SUCCESS_ENABLED flag is only a fallback for an API
 * that predates the field or whose configuration has not loaded, so a
 * prebuilt dashboard never exposes a route the API has disabled.
 */
export function isSetupSuccessEnabled(
  config?: Pick<AuthConfig, 'setup_guide_enabled'> | null,
): boolean {
  if (typeof config?.setup_guide_enabled === 'boolean') {
    return config.setup_guide_enabled;
  }
  return import.meta.env.VITE_SETUP_SUCCESS_ENABLED === 'true';
}
