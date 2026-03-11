export interface WidgetConfig {
  workspaceId: string;
  branding: {
    primaryColor: string;
    logoUrl?: string;
    welcomeMessage: string;
    widgetPosition: 'bottom-right' | 'bottom-left';
  };
  features: {
    aiEnabled: boolean;
    fileUploads: boolean;
    preChatForm: boolean;
    csatRating: boolean;
  };
}
