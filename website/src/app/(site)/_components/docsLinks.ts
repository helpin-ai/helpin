const DOCS_ROOT = '/docs';
// Help center URLs, served at helpin.ai/docs by the Pages Function in
// website/functions/docs. Same-site paths, so they open in the current tab.
// Article paths end in the article's public ID and stay stable across renames.
export const DOCS = {
  home: `${DOCS_ROOT}`,
  developer: `${DOCS_ROOT}/c/8-developer-amp-api-documentation-15a57c13`,
  widgetSdk: `${DOCS_ROOT}/articles/widget-sdk-developer-guide-40bf71ab`,
  mcpServer: `${DOCS_ROOT}/articles/connect-ai-clients-to-the-helpin-mcp-server-13528aff`,
  externalMcp: `${DOCS_ROOT}/articles/external-mcp-servers-1e6d59ec`,
  automation: `${DOCS_ROOT}/c/5-automation-a1529cea`,
  emailForwarding: `${DOCS_ROOT}/articles/support-email-forwarding-routes-c33014e7`,
  senderAddresses: `${DOCS_ROOT}/articles/custom-sender-addresses-domains-d9017160`,
  aiConnections: `${DOCS_ROOT}/articles/ai-connections-profiles-01b6b5d7`,

  selfHosting: `${DOCS_ROOT}/articles/self-hosting-overview-8de2104d`,
  selfHostingInstall: `${DOCS_ROOT}/articles/install-with-the-cli-b0520064`,
  selfHostingDeploy: `${DOCS_ROOT}/articles/deploy-on-a-public-server-ed3a4a14`,
  selfHostingConfigure: `${DOCS_ROOT}/c/self-hosting-configure-48cd823c`,
  selfHostingAI: `${DOCS_ROOT}/articles/ai-providers-knowledge-search-0eee18c5`,
  selfHostingBackups: `${DOCS_ROOT}/articles/back-up-and-restore-8a351119`,
} as const;
