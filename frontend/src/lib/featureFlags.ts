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
  ],
  support: [
    'waqar@contentstudio.io',
    'azhar@contentstudio.io',
    'amad@usermaven.com',
  ],
  agents: [
    'waqar@contentstudio.io',
    'azhar@contentstudio.io',
    'amad@usermaven.com',
  ],
};

export function isModuleEnabled(moduleId: string, userEmail?: string): boolean {
  const allowedEmails = GATED_MODULES[moduleId];
  if (!allowedEmails) return true;
  if (!userEmail) return false;
  return allowedEmails.includes(userEmail.toLowerCase());
}
