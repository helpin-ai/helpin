export interface WidgetConfig {
  workspaceId: string;
  branding: {
    primaryColor: string;
    logoUrl?: string;
    welcomeMessage: string;
    widgetPosition: 'bottom-right' | 'bottom-left';
    showBranding: boolean;
    launcherIcon?: 'chat_bubble' | 'question_mark' | 'help';
  };
  features: {
    aiEnabled: boolean;
    fileUploads: boolean;
    preChatForm: boolean;
    csatRating: boolean;
  };
}
