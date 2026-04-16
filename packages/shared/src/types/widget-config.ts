export interface HelpSpace {
  id: string;
  name: string;
  slug: string;
  icon?: string;
}

/**
 * Canonical reply-time presets. Mirrors
 * server/internal/model/support_reply_expectations.go.
 */
export const REPLY_TIME_PRESETS = ['few_minutes', 'few_hours', 'same_day', 'custom'] as const;

export type ReplyTimePreset = (typeof REPLY_TIME_PRESETS)[number];

/** Min/Max bounds on the custom minutes field — matches the Go constants. */
export const REPLY_TIME_CUSTOM_MINUTES_MIN = 1;
export const REPLY_TIME_CUSTOM_MINUTES_MAX = 10080;

/** Character cap on the outage / maintenance banner. Matches Go. */
export const SPECIAL_NOTICE_MAX_LENGTH = 500;

export interface WidgetConfig {
  workspaceId: string;
  workspaceName?: string;
  visitorName?: string;
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
    /** Structured reply-time preset that drives replyTimeText. Widget uses
     *  this when it needs to render its own copy (e.g. localized). */
    replyTimePreset?: ReplyTimePreset;
    /** Only set when replyTimePreset is "custom". */
    replyTimeMinutes?: number;
    /** Optional amber banner rendered above conversation surfaces. */
    specialNoticeText?: string;
    /** Set when the effective preset came from a mailbox override. */
    mailboxId?: string;
  };
  helpSpaces?: HelpSpace[];
}
