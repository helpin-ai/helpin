export interface HelpSpace {
  id: string;
  name: string;
  slug: string;
  icon?: string;
}

export interface WidgetConfig {
  workspaceId: string;
  workspaceName?: string;
  branding: {
    primaryColor: string;
    logoUrl?: string;
    welcomeMessage: string;
    widgetPosition: 'bottom-right' | 'bottom-left';
    showBranding: boolean;
    launcherIcon?: 'chat_bubble' | 'question_mark' | 'help';
    colorScheme?: 'system' | 'light' | 'dark';
    buttonColor?: string;
    buttonIconColor?: string;
  };
  features: {
    aiEnabled: boolean;
    showTalkToHuman: boolean;
    fileUploads: boolean;
    preChatForm: boolean;
    requirePhone: boolean;
    csatRating: boolean;
    forceIdentify: boolean;
  };
  helpSpaces?: HelpSpace[];
}
