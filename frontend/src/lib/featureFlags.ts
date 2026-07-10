/**
 * Module-level feature flags.
 * Modules listed here are only visible to the specified emails.
 * Modules NOT listed are visible to everyone.
 */
const GATED_MODULES: Record<string, string[]> = {
  crm: [
    'waqar@contentstudio.io',
    'azhar@contentstudio.io',
    'amad@usermaven.com',
    'waqar.azeem1986@gmail.com',
  ],
  support: [
    'waqar@contentstudio.io',
    'azhar@contentstudio.io',
    'amad@usermaven.com',
    'waqar.azeem1986@gmail.com',
  ],
};

export function isModuleEnabled(moduleId: string, userEmail?: string): boolean {
  const resolvedModuleId = moduleId === 'agents' ? 'automation' : moduleId;
  const allowedEmails = GATED_MODULES[resolvedModuleId];
  if (!allowedEmails) return true;
  if (!userEmail) return false;
  return allowedEmails.includes(userEmail.toLowerCase());
}

// Development defaults on for local testing; production requires an explicit rollout flag.
export function isSetupSuccessEnabled(): boolean {
  return import.meta.env.DEV || import.meta.env.VITE_SETUP_SUCCESS_ENABLED === 'true';
}
