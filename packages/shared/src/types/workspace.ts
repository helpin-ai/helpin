export interface Workspace {
  id: string;
  organizationId: string;
  name: string;
  slug: string;
  customDomain?: string;
  defaultLanguage: string;
  supportedLanguages: string[];
  branding: WorkspaceBranding;
  createdAt: string;
}

export interface WorkspaceBranding {
  primaryColor: string;
  logoUrl?: string;
  welcomeMessage: string;
  widgetPosition: 'bottom-right' | 'bottom-left';
}
