export interface HelpSpace {
  id: string;
  name: string;
  slug: string;
  icon?: string;
}

export interface WidgetConfig {
  workspaceId: string;
  workspaceName?: string;
  availableTeammates?: Array<{
    userId: string;
    name: string;
    avatarUrl?: string;
    status?: 'online' | 'away' | 'offline';
  }>;
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
    aiFirst: boolean;
    showTalkToHuman: boolean;
    escalationMessage?: string;
    fileUploads: boolean;
    preChatForm: boolean;
    requirePhone: boolean;
    csatRating: boolean;
    forceIdentify: boolean;
  };
  availability: {
    isOnline: boolean;
    statusText: string;
    replyTimeText: string;
    outsideHoursMessage?: string;
    nextOnlineAt?: string;
  };
  helpSpaces?: HelpSpace[];
}
