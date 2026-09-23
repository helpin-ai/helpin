// Help center URLs, served at helpin.ai/docs by the Pages Function in
// website/functions/docs. Same-site paths, so they open in the current tab.
// Article paths end in the article's public ID and stay stable across renames.
export const DOCS = {
  home: '/docs',
  developer: '/docs/c/8-developer-amp-api-documentation-15a57c13',
  widgetSdk: '/docs/articles/widget-sdk-developer-guide-40bf71ab',
  mcpServer: '/docs/articles/connect-ai-clients-to-the-helpin-mcp-server-13528aff',
  externalMcp: '/docs/articles/external-mcp-servers-1e6d59ec',
  automation: '/docs/c/5-automation-a1529cea',
  emailForwarding: '/docs/articles/support-email-forwarding-routes-c33014e7',
  senderAddresses: '/docs/articles/custom-sender-addresses-domains-d9017160',
  aiConnections: '/docs/c/54-agents-071e3280',

  selfHosting: '/docs/c/self-hosting-15b46d4c',
  selfHostingInstall: '/docs/c/self-hosting-install-deploy-35ba824f',
  selfHostingDeploy: '/docs/c/self-hosting-install-deploy-35ba824f',
  selfHostingConfigure: '/docs/c/self-hosting-configure-48cd823c',
  selfHostingAI: '/docs/c/self-hosting-configure-48cd823c',
  selfHostingBackups: '/docs/c/self-hosting-operate-3222d66f',
} as const;
