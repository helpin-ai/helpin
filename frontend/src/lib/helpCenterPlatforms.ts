/** How the help center platforms an import can read by address are written. */
const PLATFORM_NAMES: Record<string, string> = {
  intercom: 'Intercom',
  zendesk: 'Zendesk',
  crisp: 'Crisp',
  freshdesk: 'Freshdesk',
  helpscout: 'Help Scout',
  hubspot: 'HubSpot',
  mintlify: 'Mintlify',
  ferndesk: 'Ferndesk',
  gitbook: 'GitBook',
  readme: 'ReadMe',
  fern: 'Fern',
  featurebase: 'Featurebase',
  document360: 'Document360',
  helpkit: 'HelpKit',
  plain: 'Plain',
  archbee: 'Archbee',
  kustomer: 'Kustomer',
};

/** The platform's name, or undefined for one that has no name worth showing. */
export function helpCenterPlatformName(platform?: string): string | undefined {
  return platform ? PLATFORM_NAMES[platform] : undefined;
}
