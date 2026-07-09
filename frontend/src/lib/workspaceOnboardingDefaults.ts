import { generateWorkspaceSlug } from './slugUtils';

export type WorkspaceOnboardingDefaults = {
  name: string;
  slug: string;
  workspaceKey: string;
  websiteUrl: string;
};

const emptyDefaults: WorkspaceOnboardingDefaults = {
  name: '',
  slug: '',
  workspaceKey: '',
  websiteUrl: '',
};

const personalEmailDomains = new Set([
  'aol.com',
  'gmail.com',
  'googlemail.com',
  'hotmail.com',
  'icloud.com',
  'live.com',
  'me.com',
  'msn.com',
  'outlook.com',
  'proton.me',
  'protonmail.com',
  'yahoo.com',
  'ymail.com',
]);

const twoPartCountrySuffixes = new Set([
  'ac.uk',
  'co.jp',
  'co.nz',
  'co.uk',
  'com.au',
  'com.br',
  'com.mx',
  'com.sg',
  'com.tr',
  'com.tw',
  'com.ua',
  'net.au',
  'org.uk',
]);

function capitalizeWord(word: string): string {
  if (!word) return '';
  return word.charAt(0).toUpperCase() + word.slice(1).toLowerCase();
}

function displayNameFromDomainLabel(label: string): string {
  return label
    .split(/[^a-zA-Z0-9]+/)
    .filter(Boolean)
    .map(capitalizeWord)
    .join(' ');
}

function workspaceKeyFromName(name: string): string {
  return name.replace(/[^a-zA-Z]/g, '').toUpperCase().slice(0, 3);
}

export function workspaceDefaultsFromUserEmail(email: string | null | undefined): WorkspaceOnboardingDefaults {
  const domain = email?.trim().toLowerCase().split('@')[1]?.replace(/\.+$/, '');
  if (!domain || personalEmailDomains.has(domain)) {
    return emptyDefaults;
  }

  const parts = domain.split('.').filter(Boolean);
  if (parts.length < 2) {
    return emptyDefaults;
  }

  const lastTwoParts = parts.slice(-2).join('.');
  const usesTwoPartCountrySuffix = parts.length >= 3 && twoPartCountrySuffixes.has(lastTwoParts);
  const companyLabel = usesTwoPartCountrySuffix ? parts[parts.length - 3] : parts[parts.length - 2];
  const registrableDomain = usesTwoPartCountrySuffix ? parts.slice(-3).join('.') : parts.slice(-2).join('.');
  const name = displayNameFromDomainLabel(companyLabel);
  if (!name) {
    return emptyDefaults;
  }

  return {
    name,
    slug: generateWorkspaceSlug(name),
    workspaceKey: workspaceKeyFromName(name),
    websiteUrl: `https://${registrableDomain}`,
  };
}

export function workspaceDefaultsForCreation({
  email,
  useEmailDefaults,
}: {
  email: string | null | undefined;
  useEmailDefaults: boolean;
}): WorkspaceOnboardingDefaults {
  return useEmailDefaults ? workspaceDefaultsFromUserEmail(email) : emptyDefaults;
}
