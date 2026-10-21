import { PrivacyNoticeSettings } from './chat-widget/PrivacyNoticeSettings';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { SettingsSection } from './SettingsSection';
import { Select as AISelect, SelectTrigger as AISelectTrigger, SelectValue as AISelectValue, SelectContent as AISelectContent, SelectItem as AISelectItem } from '@/components/design-system/quiet-dropdown-select';
import { SupportAIPreview } from "./SupportAIPreview";
import { WidgetOriginSettings } from './chat-widget/WidgetOriginSettings';
import { brandingDescription } from '@edition';
import { AIReplyChannelsSelect, getAIReplyChannels, type AIReplyChannels } from './chat-widget/AIReplyChannelsSelect';
import { AIFollowUpSettings } from './chat-widget/AIFollowUpSettings';
import { useEffect, useRef, useState } from 'react';
import { useSettingsAutosave } from '@/hooks/useSettingsAutosave';
import { SettingsAutosaveGuard } from './SettingsAutosaveGuard';
import { SettingsSaveBar } from './SettingsSaveBar';
import { SettingsSaveStatus } from './SettingsSaveStatus';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Switch } from '@/components/ui/switch';
import { Skeleton } from '@/components/ui/skeleton';
import { Textarea } from '@/components/ui/textarea';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs';
import { DEFAULT_PRIVACY_NOTICE_TEXT, getPrivacyPolicyURL, formatReplyTimeCopy, SPECIAL_NOTICE_MAX_LENGTH } from '@helpin-ai/shared';
import { toast } from 'sonner';
import { Shield01Icon, Copy01Icon, CodeIcon, Message01Icon, HelpCircleIcon, Image01Icon, Key01Icon, BotIcon, ArrowDown01Icon, StarIcon } from '@/lib/icons';
import { useChatSettings, useUpdateChatSettings, useRegenerateWidgetKey, useDocsSpaces } from '@/hooks/queries';
import { useWorkspaceBilling } from '@edition';
import { useSupportAgents, useSupportMailboxes } from '@/hooks/queries/useSupport';
import { useWorkspaceTeams } from '@/hooks/useWorkspaceTeams';
import { WidgetPreview } from './WidgetPreview';
import { CodeBlock } from '@/components/ui/code-block';
import { BrandColorPicker } from '@/components/pm/ColorPicker';
import { useAuthStore } from '@/stores/authStore';
import { useWorkspaceStore } from '@/stores/workspaceStore';
import { cn } from '@/lib/utils';
import type { BusinessHoursDay } from '@/lib/pmTypes';
import { canRemoveHelpinBranding, COLOR_SCHEME_OPTIONS, COMMON_TIMEZONES, DAYS, DEFAULT_EMAIL_FALLBACK_DELAY_SECS, ICON_OPTIONS, NO_AGENT_VALUE } from './chat-widget/constants';
import { PreviewLayout } from './chat-widget/PreviewLayout';
import {
  buildPreviewAvailability,
  normalizeBusinessHoursDay,
  normalizeBusinessHoursSchedule,
  previewEscalationMessage,
  sortHelpSpaceIds,
  type ChatSettingsDraft,
} from './chat-widget/utils';
import { DelayedTeamReplySettings } from './chat-widget/DelayedTeamReplySettings';
import { DEFAULT_DELAYED_TEAM_REPLY_MINUTES, DEFAULT_DELAYED_TEAM_REPLY_MESSAGE, DEFAULT_DELAYED_TEAM_REPLY_MESSAGE_NO_EMAIL } from './chat-widget/delayedTeamReply';
import { CHAT_WIDGET_ESCALATION_TABS } from './chat-widget/escalationTabs';
import {
  CHAT_WIDGET_AI_RESPONSE_MODES,
  DEFAULT_CHAT_WIDGET_AI_RESPONSE_MODE,
  getChatWidgetAIResponseModeForUI,
  isChatWidgetAIResponseModeActive,
  isChatWidgetAIFirst,
} from './chat-widget/responseModes';
import { getChatWidgetAIAssistantEnableBlocker } from './chat-widget/aiAssistantReadiness';
import { DEFAULT_AI_HANDOFF_FOLLOWUPS, formatAIHandoffFollowupOption } from './chat-widget/handoffFollowups';
import { buildWidgetInstallPrompt, type WidgetInstallFramework } from './chat-widget/installPrompts';
import { WidgetInstallAIPrompt } from './chat-widget/WidgetInstallAIPrompt';
import { CuratedGuidanceField } from './CuratedGuidanceField';
import { WelcomeMessageSettings } from './chat-widget/WelcomeMessageSettings';

/* ── Main component ──────────────────────────────────────────────────── */

type ChatGeneralTabMode = 'chat-widget' | 'ai-assistant';


export function ChatGeneralTab({ workspaceId, mode = 'chat-widget', canManageSigningSecret = false }: {
  workspaceId: string;
  mode?: ChatGeneralTabMode;
  canManageSigningSecret?: boolean;
}) {
  return <ChatGeneralSettings key={`${workspaceId}:${mode}`} workspaceId={workspaceId} mode={mode} canManageSigningSecret={canManageSigningSecret} />;
}

function ChatGeneralSettings({ workspaceId, mode, canManageSigningSecret }: { workspaceId: string; mode: ChatGeneralTabMode; canManageSigningSecret: boolean }) {
  const publicConfig = useAuthStore((s) => s.configuration);
  const widgetHost = publicConfig?.public_widget_url || window.location.origin;
  const sdkURL = publicConfig?.public_sdk_url || `${widgetHost}/sdk/lib.js`;
  const workspace = useWorkspaceStore((s) => s.currentWorkspace);
  const isAIAssistantPage = mode === 'ai-assistant';
  const { data, isLoading, refetch: refetchSettings } = useChatSettings(workspaceId);
  const { data: docsSpaces = [], isLoading: docsSpacesLoading } = useDocsSpaces(workspaceId);
  const { data: billing, isLoading: billingLoading } = useWorkspaceBilling(workspaceId);
  const updateMutation = useUpdateChatSettings(workspaceId);
  const regenerateKeyMutation = useRegenerateWidgetKey(workspaceId);
  const { teams } = useWorkspaceTeams(workspaceId);
  const { data: supportAgents = [] } = useSupportAgents(workspaceId);
  const { data: supportMailboxes = [] } = useSupportMailboxes(workspaceId);

  const [snippetTab, setSnippetTab] = useState<WidgetInstallFramework>('html');
  const [previewInitialView, setPreviewInitialView] = useState<'home' | 'conversation'>('home');

  // Identity state
  const [requireEmail, setRequireEmail] = useState(true);
  const [requirePhone, setRequirePhone] = useState(false);
  const [welcomeMessage, setWelcomeMessage] = useState('');
  const [privacyNoticeEnabled, setPrivacyNoticeEnabled] = useState(false);
  const [privacyPolicyUrl, setPrivacyPolicyUrl] = useState('');
  const [privacyNoticeText, setPrivacyNoticeText] = useState(DEFAULT_PRIVACY_NOTICE_TEXT);

  // Appearance state
  const [brandColor, setBrandColor] = useState('#6366F1');
  const [showBranding, setShowBranding] = useState(true);
  const [launcherPosition, setLauncherPosition] = useState('bottom_right');
  const [launcherIcon, setLauncherIcon] = useState('chat_bubble');
  const [colorScheme, setColorScheme] = useState('light');
  const [buttonColor, setButtonColor] = useState('#000000');
  const [buttonIconColor, setButtonIconColor] = useState('#FFFFFF');
  const [logoUrl, setLogoUrl] = useState('');
  const [widgetName, setWidgetName] = useState('');
  const [widgetAvatarUrl, setWidgetAvatarUrl] = useState('');
  const [widgetHelpSpaceIds, setWidgetHelpSpaceIds] = useState<string[]>([]);


  // AI & Routing state
  const [aiEnabled, setAiEnabled] = useState(false);
  const [aiEnableAttempted, setAiEnableAttempted] = useState(false);
  const [aiAgentId, setAiAgentId] = useState(NO_AGENT_VALUE);
  const [confidenceThreshold, setConfidenceThreshold] = useState('0.7');
  const [aiReplyChannels, setAiReplyChannels] = useState<AIReplyChannels>('chat');
  const [aiResponseMode, setAiResponseMode] = useState(DEFAULT_CHAT_WIDGET_AI_RESPONSE_MODE);
  const [aiFollowUpEnabled, setAiFollowUpEnabled] = useState(true);
  const [aiFollowUpDelay, setAiFollowUpDelay] = useState(24);
  const [aiFollowUpClose, setAiFollowUpClose] = useState(1);
  const [aiFollowUpSecondDelay, setAiFollowUpSecondDelay] = useState(24);
  const [aiMaxFollowups, setAiMaxFollowups] = useState(DEFAULT_AI_HANDOFF_FOLLOWUPS);
  const [showTalkToHuman, setShowTalkToHuman] = useState(true);
  const [escalationMessage, setEscalationMessage] = useState('Let me connect you with a team member who can help further.');
  const [escalationMessageBusy, setEscalationMessageBusy] = useState('');
  const [escalationMessageAfterHours, setEscalationMessageAfterHours] = useState('');
  const [delayedTeamReplyMinutes, setDelayedTeamReplyMinutes] = useState(DEFAULT_DELAYED_TEAM_REPLY_MINUTES);
  const [delayedTeamReplyMessage, setDelayedTeamReplyMessage] = useState(DEFAULT_DELAYED_TEAM_REPLY_MESSAGE);
  const [delayedTeamReplyMessageNoEmail, setDelayedTeamReplyMessageNoEmail] = useState(DEFAULT_DELAYED_TEAM_REPLY_MESSAGE_NO_EMAIL);
  const [handoffBehavior, setHandoffBehavior] = useState('unassigned');
  const [handoffTeamId, setHandoffTeamId] = useState<string | null>(null);
  const [aiHandoffMailboxId, setAiHandoffMailboxId] = useState<string | null>(null);
  const [businessHoursEnabled, setBusinessHoursEnabled] = useState(false);
  const [timezone, setTimezone] = useState('America/New_York');
  const [schedule, setSchedule] = useState<Record<string, BusinessHoursDay>>({});
  const [outsideMessage, setOutsideMessage] = useState('');
  const [replyTimePreset, setReplyTimePreset] = useState<string>('few_minutes');
  const [replyTimeCustomMinutes, setReplyTimeCustomMinutes] = useState<number | null>(null);
  const [specialNoticeText, setSpecialNoticeText] = useState<string>('');
  const [emailFallbackDelaySecs, setEmailFallbackDelaySecs] = useState(DEFAULT_EMAIL_FALLBACK_DELAY_SECS);
  const [emailFallbackFromName, setEmailFallbackFromName] = useState('');
  const [emailFallbackMaxDeliveryAgeMins, setEmailFallbackMaxDeliveryAgeMins] = useState(10);
  const [csatEnabled, setCsatEnabled] = useState(false);
  const [fileUploadsEnabled, setFileUploadsEnabled] = useState(true);
  const [forceVisitorIdentity, setForceVisitorIdentity] = useState(false);

  // Accordion state
  const [expandedSections, setExpandedSections] = useState<Set<string>>(
    () => new Set(['widget-settings']),
  );

  const logoInputRef = useRef<HTMLInputElement>(null);
  const avatarInputRef = useRef<HTMLInputElement>(null);
  const [dragOver, setDragOver] = useState(false);
  const canRemoveBranding = canRemoveHelpinBranding(billing);
  const effectiveShowBranding = canRemoveBranding ? showBranding : true;

  const [hydrated, setHydrated] = useState(false);

  useEffect(() => {
    if (data?.settings && !billingLoading && !hydrated) {
      const s = data.settings;
      const normalizedSchedule = normalizeBusinessHoursSchedule(s.business_hours_schedule);
      const sortedHelpSpaceIds = sortHelpSpaceIds(s.widget_help_space_ids);

      setHydrated(true);

      setRequireEmail(s.require_email_before_chat);
      setRequirePhone(s.require_phone_after_email);
      setWelcomeMessage(s.welcome_message);
      setPrivacyNoticeEnabled(s.privacy_notice_enabled ?? false);
      setPrivacyPolicyUrl(s.privacy_policy_url ?? '');
      setPrivacyNoticeText(s.privacy_notice_text || DEFAULT_PRIVACY_NOTICE_TEXT);
      setBrandColor(s.brand_color);
      setShowBranding(canRemoveBranding ? s.show_branding : true);
      setLauncherPosition(s.launcher_position);
      setLauncherIcon(s.launcher_icon);
      setColorScheme(s.color_scheme || 'light');
      setButtonColor(s.button_color || '#000000');
      setButtonIconColor(s.button_icon_color || '#FFFFFF');
      setLogoUrl(s.logo_url || '');
      setWidgetName(s.widget_name || '');
      setWidgetAvatarUrl(s.widget_avatar_url || '');
      setWidgetHelpSpaceIds(sortedHelpSpaceIds);
      setAiEnabled(s.ai_enabled && isChatWidgetAIResponseModeActive(s.ai_response_mode));
      setAiAgentId(s.ai_agent_id ?? NO_AGENT_VALUE);
      setConfidenceThreshold(String(s.ai_confidence_threshold));
      setAiResponseMode(getChatWidgetAIResponseModeForUI(s.ai_response_mode));
      setAiReplyChannels(getAIReplyChannels(s.ai_reply_channels));
      setAiFollowUpEnabled(s.ai_follow_up_enabled ?? true);
      setAiFollowUpDelay(s.ai_follow_up_delay_hours ?? 24);
      setAiFollowUpClose(s.ai_follow_up_close_hours ?? 1);
      setAiFollowUpSecondDelay(s.ai_follow_up_second_delay_hours ?? 24);
      setAiMaxFollowups(s.ai_max_followups ?? DEFAULT_AI_HANDOFF_FOLLOWUPS);
      setShowTalkToHuman(s.show_talk_to_human);
      setEscalationMessage(s.escalation_message || 'Let me connect you with a team member who can help further.');
      setEscalationMessageBusy(s.escalation_message_busy ?? '');
      setEscalationMessageAfterHours(s.escalation_message_after_hours ?? '');
      setDelayedTeamReplyMinutes(s.delayed_team_reply_minutes ?? DEFAULT_DELAYED_TEAM_REPLY_MINUTES);
      setDelayedTeamReplyMessage(s.delayed_team_reply_message ?? DEFAULT_DELAYED_TEAM_REPLY_MESSAGE);
      setDelayedTeamReplyMessageNoEmail(s.delayed_team_reply_message_no_email ?? DEFAULT_DELAYED_TEAM_REPLY_MESSAGE_NO_EMAIL);
      setHandoffBehavior(s.handoff_behavior);
      setHandoffTeamId(s.handoff_team_id);
      setAiHandoffMailboxId(s.ai_handoff_mailbox_id);
      setBusinessHoursEnabled(s.business_hours_enabled);
      setTimezone(s.business_hours_timezone);
      setSchedule(normalizedSchedule);
      setOutsideMessage(s.outside_hours_message);
      setReplyTimePreset(s.reply_time_preset ?? 'few_minutes');
      setReplyTimeCustomMinutes(s.reply_time_custom_minutes ?? null);
      setSpecialNoticeText(s.special_notice_text ?? '');
      setEmailFallbackDelaySecs(s.email_fallback_delay_secs ?? DEFAULT_EMAIL_FALLBACK_DELAY_SECS);
      setEmailFallbackFromName(s.email_fallback_from_name ?? '');
      setEmailFallbackMaxDeliveryAgeMins(Math.round((s.email_fallback_max_delivery_age_secs ?? 600) / 60));
      setCsatEnabled(s.csat_enabled);
      setFileUploadsEnabled(s.file_uploads_enabled ?? true);
      setForceVisitorIdentity(s.force_visitor_identity ?? false);
    }
  }, [data, billingLoading, canRemoveBranding, hydrated]);

  const aiAssistantEnableBlocker = getChatWidgetAIAssistantEnableBlocker({
    aiAgentId,
    supportAgentCount: supportAgents.length,
  });
  const showAIAssistantEnableBlocker = Boolean(aiAssistantEnableBlocker && (aiEnableAttempted || aiEnabled));

  const privacyNoticeError = privacyNoticeEnabled && !getPrivacyPolicyURL(privacyPolicyUrl)
    ? 'Enter a valid Privacy Policy URL to save your changes.'
    : privacyNoticeEnabled && !privacyNoticeText.trim()
      ? 'Enter notice text to save your changes.'
      : undefined;
  const settingsDraft: ChatSettingsDraft = {
    require_email_before_chat: requireEmail,
    require_phone_after_email: requirePhone,
    welcome_message: welcomeMessage,
    privacy_notice_enabled: privacyNoticeEnabled,
    privacy_policy_url: privacyPolicyUrl.trim(),
    privacy_notice_text: privacyNoticeText.trim(),
    widget_name: widgetName,
    widget_avatar_url: widgetAvatarUrl,
    widget_help_space_ids: sortHelpSpaceIds(widgetHelpSpaceIds),
    brand_color: brandColor,
    show_branding: effectiveShowBranding,
    launcher_position: launcherPosition,
    launcher_icon: launcherIcon,
    color_scheme: colorScheme,
    button_color: buttonColor,
    button_icon_color: buttonIconColor,
    logo_url: logoUrl,
    ai_enabled: aiEnabled && !aiAssistantEnableBlocker,
    ai_agent_id: aiAgentId === NO_AGENT_VALUE ? '' : aiAgentId,
    ai_confidence_threshold: parseFloat(confidenceThreshold),
    ai_response_mode: aiResponseMode,
    ai_reply_channels: aiReplyChannels,
    ai_max_followups: aiMaxFollowups,
    ai_follow_up_enabled: aiFollowUpEnabled,
    ai_follow_up_delay_hours: aiFollowUpDelay,
    ai_follow_up_close_hours: aiFollowUpClose,
    ai_follow_up_second_delay_hours: aiFollowUpSecondDelay,
    ai_follow_up_max_per_conversation: data?.settings.ai_follow_up_max_per_conversation ?? 2,
    ai_auto_resolve_timeout: data?.settings.ai_auto_resolve_timeout ?? 24,
    show_talk_to_human: showTalkToHuman,
    escalation_message: escalationMessage,
    escalation_message_busy: escalationMessageBusy || undefined,
    escalation_message_after_hours: escalationMessageAfterHours || undefined,
    delayed_team_reply_minutes: delayedTeamReplyMinutes,
    delayed_team_reply_message: delayedTeamReplyMessage,
    delayed_team_reply_message_no_email: delayedTeamReplyMessageNoEmail,
    handoff_behavior: handoffBehavior,
    handoff_team_id: handoffBehavior === 'assign_to_team' ? handoffTeamId : null,
    default_mailbox_id: data?.settings.default_mailbox_id ?? null,
    ai_handoff_mailbox_id: aiHandoffMailboxId,
    business_hours_enabled: businessHoursEnabled,
    business_hours_timezone: timezone,
    business_hours_schedule: normalizeBusinessHoursSchedule(schedule),
    outside_hours_message: outsideMessage,
    reply_time_preset: replyTimePreset,
    reply_time_custom_minutes: replyTimePreset === 'custom' ? replyTimeCustomMinutes : null,
    special_notice_text: specialNoticeText.trim() ? specialNoticeText : null,
    email_fallback_enabled: true,
    email_fallback_delay_secs: emailFallbackDelaySecs,
    email_fallback_from_name: emailFallbackFromName,
    email_fallback_max_delivery_age_secs: Math.max(
      emailFallbackDelaySecs,
      Math.max(2, Math.min(30, emailFallbackMaxDeliveryAgeMins)) * 60,
    ),
    forwarded_email_detection_enabled: data?.settings.forwarded_email_detection_enabled ?? true,
    forwarded_email_detection_mode: data?.settings.forwarded_email_detection_mode ?? 'high_confidence_any_sender',
    forwarded_email_min_confidence: data?.settings.forwarded_email_min_confidence ?? 80,
    csat_enabled: csatEnabled,
    file_uploads_enabled: fileUploadsEnabled,
    force_visitor_identity: forceVisitorIdentity,
  };
  const autosave = useSettingsAutosave({
    scopeKey: workspaceId,
    enabled: hydrated && !billingLoading && !privacyNoticeError,
    value: settingsDraft,
    // The hook captures this baseline only once after the form is hydrated.
    // Later query refreshes must not replace edits made while a save is pending.
    savedValue: hydrated ? settingsDraft : null,
    save: (draft: ChatSettingsDraft) => updateMutation.mutateAsync(draft),
    delayMs: 800,
  });

  const updateDay = (dayKey: string, patch: Partial<BusinessHoursDay>) => {
    setSchedule(prev => ({
      ...prev,
      [dayKey]: { ...normalizeBusinessHoursDay(prev[dayKey]), ...patch },
    }));
  };

  const toggleSection = (key: string) => {
    setExpandedSections(prev => {
      const next = new Set(prev);
      if (next.has(key)) {
        next.delete(key);
      } else {
        next.add(key);
      }
      return next;
    });
  };

  const isExpanded = (key: string) => expandedSections.has(key);

  const handleAIEnabledChange = (checked: boolean) => {
    if (checked) {
      if (aiAssistantEnableBlocker) {
        setAiEnableAttempted(true);
        setAiEnabled(false);
        return;
      }
      setAiEnableAttempted(false);
      setAiEnabled(true);
      return;
    }
    setAiEnableAttempted(false);
    setAiEnabled(false);
  };

  const handleAIAgentChange = (value: string) => {
    setAiAgentId(value);
    if (value && value !== NO_AGENT_VALUE) {
      setAiEnableAttempted(false);
    }
  };

  const externalDocsSpaces = docsSpaces.filter((space) => space.type === 'external_capable');

  const toggleHelpSpace = (spaceId: string, enabled: boolean) => {
    setWidgetHelpSpaceIds((current) => (
      enabled
        ? current.includes(spaceId) ? current : [...current, spaceId]
        : current.filter((id) => id !== spaceId)
    ));
  };


  const copyToClipboard = async (text: string, label: string) => {
    try {
      await navigator.clipboard.writeText(text);
      toast.success(`${label} copied`);
    } catch {
      toast.error(`Couldn't copy ${label.toLowerCase()}`);
    }
  };

  const handleLogoFileSelect = (file: File) => {
    if (!file.type.startsWith('image/')) {
      toast.error('Please select an image file (PNG, JPG, SVG)');
      return;
    }
    if (file.size > 2 * 1024 * 1024) {
      toast.error('Image must be under 2MB');
      return;
    }
    const reader = new FileReader();
    reader.onload = () => {
      setLogoUrl(reader.result as string);
    };
    reader.readAsDataURL(file);
  };

  const handleDragOver = (e: React.DragEvent) => {
    e.preventDefault();
    setDragOver(true);
  };

  const handleDragLeave = () => {
    setDragOver(false);
  };

  const handleDrop = (e: React.DragEvent) => {
    e.preventDefault();
    setDragOver(false);
    const file = e.dataTransfer.files?.[0];
    if (file) handleLogoFileSelect(file);
  };

  if (isLoading || billingLoading) {
    return (
      <div className="space-y-6">
        <Skeleton className="h-48 w-full rounded-lg" />
        <Skeleton className="h-48 w-full rounded-lg" />
        <Skeleton className="h-64 w-full rounded-lg" />
      </div>
    );
  }

  if (!data) {
    return <div role="alert" className="space-y-3 text-sm"><p>Could not load support settings.</p><Button variant="outline" onClick={() => void refetchSettings()}>Retry</Button></div>;
  }

  const widgetKey = data?.widget_key ?? '';
  const htmlSnippet = `<!-- Step 1: Install the pixel — add before closing </body> tag -->

<script type="text/javascript">
  (function () {
    window.helpin = window.helpin || function () {
      (window.helpinQ = window.helpinQ || []).push(arguments);
    };
    var t = document.createElement('script'),
        s = document.getElementsByTagName('script')[0];
    t.defer = true;
    t.id = 'helpin-widget';
    t.setAttribute('data-widget-key', '${widgetKey}');
    t.setAttribute('data-host', '${widgetHost}');
    t.setAttribute('data-support-only', 'true');
    t.src = '${sdkURL}';
    s.parentNode.insertBefore(t, s);
  })();
</script>

<!-- Step 2: Identify logged-in users & capture leads (optional) -->

<script type="text/javascript">
  // Identify a logged-in user
  helpin('id', {
    id: 'your-internal-user-id',
    email: 'user@example.com',
    first_name: 'Jane',
    last_name: 'Doe',
    company: {
      id: 'company-123',
      name: 'Acme Inc',
      created_at: '2024-01-15T00:00:00Z'
    }
  });

  // Capture a lead (e.g. from a signup form)
  helpin('lead', {
    email: 'visitor@example.com',
    first_name: 'New',
    last_name: 'Lead',
    company: {
      id: 'company-123',
      name: 'Acme Inc',
      created_at: '2024-01-15T00:00:00Z'
    }
  });
</script>`;

  const reactSnippet = `// 1. Install the packages
npm install @helpin-ai/react @helpin-ai/sdk-js

// 2. Wrap your app with HelpinProvider
import { createClient, HelpinProvider } from '@helpin-ai/react';

const client = createClient({
  widgetKey: '${widgetKey}',
  host: '${widgetHost}',
  widgetRuntimeUrl: '${sdkURL}',
  supportOnly: true,
});

function App() {
  return (
    <HelpinProvider client={client}>
      {/* Your app */}
    </HelpinProvider>
  );
}

// 3. Identify users (optional)
import { useEffect } from 'react';
import { useHelpin } from '@helpin-ai/react';

function Dashboard() {
  const { id, lead } = useHelpin();

  useEffect(() => {
    // Identify a logged-in user
    id({
      id: 'user-123',
      email: 'user@example.com',
      first_name: 'Jane',
      last_name: 'Doe',
      company: {
        id: 'company-123',
        name: 'Acme Inc',
        created_at: '2024-01-15T00:00:00Z',
      },
    });

    // Or capture a lead
    lead({
      email: 'visitor@example.com',
      first_name: 'New',
      last_name: 'Lead',
      company: {
        id: 'company-123',
        name: 'Acme Inc',
        created_at: '2024-01-15T00:00:00Z',
      },
    });
  }, []);
}`;

  const vueSnippet = `// 1. Install the packages
npm install @helpin-ai/vue @helpin-ai/sdk-js

// 2. Install the plugin in main.ts
import { createApp } from 'vue';
import { createClient, HelpinPlugin } from '@helpin-ai/vue';
import App from './App.vue';

const client = createClient({
  widgetKey: '${widgetKey}',
  host: '${widgetHost}',
  widgetRuntimeUrl: '${sdkURL}',
  supportOnly: true,
});

createApp(App)
  .use(HelpinPlugin, { client })
  .mount('#app');

// 3. Identify users from a descendant component (optional)
<script setup lang="ts">
import { onMounted } from 'vue';
import { useHelpin } from '@helpin-ai/vue';

const helpin = useHelpin();

onMounted(() => {
  void helpin.id({
    id: 'user-123',
    email: 'user@example.com',
    first_name: 'Jane',
    last_name: 'Doe',
  });
});
</script>`;

  const nextjsSnippet = `// 1. Install the packages
npm install @helpin-ai/nextjs @helpin-ai/sdk-js

// 2. Create a Client Component provider (app/providers.tsx)
'use client';

import { useMemo } from 'react';
import { createClient, HelpinProvider } from '@helpin-ai/nextjs';

export function Providers({ children }) {
  const client = useMemo(() => createClient({
    widgetKey: '${widgetKey}',
    host: '${widgetHost}',
  widgetRuntimeUrl: '${sdkURL}',
  supportOnly: true,
  }), []);

  return <HelpinProvider client={client}>{children}</HelpinProvider>;
}

// 3. Wrap your app from app/layout.tsx
import { Providers } from './providers';

export default function RootLayout({ children }) {
  return (
    <html>
      <body><Providers>{children}</Providers></body>
    </html>
  );
}

// 4. Identify users from a Client Component (optional)
'use client';
import { useEffect } from 'react';
import { useHelpin } from '@helpin-ai/nextjs';

function Dashboard() {
  const { id } = useHelpin();

  useEffect(() => {
    void id({
      id: 'user-123',
      email: 'user@example.com',
      first_name: 'Jane',
      last_name: 'Doe',
      company: {
        id: 'company-123',
        name: 'Acme Inc',
        created_at: '2024-01-15T00:00:00Z',
      },
    });
  }, [id]);
}`;

  const installPrompt = buildWidgetInstallPrompt({
    framework: snippetTab,
    widgetKey,
    host: widgetHost,
    runtimeURL: sdkURL,
  });

  // Derive help spaces for the widget preview
  const previewHelpSpaces = docsSpaces
    .filter(space => widgetHelpSpaceIds.includes(space.id))
    .map(space => ({ id: space.id, name: space.name, slug: space.slug }));

  const previewHost = widgetHost;
  const previewAvailability = buildPreviewAvailability({
    businessHoursEnabled,
    timezone,
    schedule,
    outsideMessage,
    replyTimePreset,
    replyTimeCustomMinutes,
    specialNoticeText: specialNoticeText.trim() || null,
  });
  const handoffMailboxName = aiHandoffMailboxId
    ? supportMailboxes.find((mailbox) => mailbox.id === aiHandoffMailboxId)?.name ?? 'Selected inbox'
    : 'Shared Inbox';
  const handoffTeamName = handoffTeamId
    ? teams.find((team) => team.id === handoffTeamId)?.name ?? null
    : null;
  const handoffSummaryElement = (() => {
    if (handoffBehavior === 'round_robin') {
      return (
        <>
          When AI hands off to a human, the conversation moves to <strong>{handoffMailboxName}</strong> and is assigned by <strong>round robin</strong>.
        </>
      );
    }
    if (handoffBehavior === 'assign_to_team') {
      return (
        <>
          When AI hands off to a human, the conversation moves to <strong>{handoffMailboxName}</strong> and is assigned to <strong>{handoffTeamName || 'the selected team'}</strong>.
        </>
      );
    }
    return (
      <>
        When AI hands off to a human, the conversation moves to <strong>{handoffMailboxName}</strong> and remains <strong>unassigned</strong>.
      </>
    );
  })();
  const routingAssignmentHref = workspace?.slug
    ? `/w/${workspace.slug}/settings/inboxes-routing?tab=routing`
    : null;
  const aiFirst = isChatWidgetAIFirst(settingsDraft.ai_enabled === true, aiResponseMode, aiReplyChannels);

  // Shared preview element used by both views
  const previewElement = (
    <WidgetPreview
      brandColor={brandColor}
      showBranding={effectiveShowBranding}
      launcherPosition={launcherPosition}
      launcherIcon={launcherIcon}
      welcomeMessage={welcomeMessage}
      privacyNotice={{ enabled: privacyNoticeEnabled, policyUrl: privacyPolicyUrl, text: privacyNoticeText }}
      initialView={isExpanded('privacy') ? 'conversation' : previewInitialView}
      workspaceName={widgetName || workspace?.name}
      workspaceLogoUrl={widgetAvatarUrl || workspace?.logo_url}
      colorScheme={colorScheme}
      buttonColor={buttonColor}
      buttonIconColor={buttonIconColor}
      logoUrl={logoUrl}
      helpSpaces={previewHelpSpaces}
      availability={previewAvailability}
      aiFirst={aiFirst}
      showTalkToHuman={showTalkToHuman}
      escalationMessage={escalationMessage}
      widgetKey={widgetKey}
      host={previewHost}
    />
  );

  const saveIndicator = (
    <SettingsSaveBar>
      <SettingsAutosaveGuard isDirty={autosave.isDirty} error={autosave.error} onRetry={autosave.retry} />
      {!isAIAssistantPage && <span className="mr-auto text-xs text-muted-foreground">Changes save automatically</span>}
      <SettingsSaveStatus status={autosave.status} error={autosave.error} onRetry={autosave.retry} />
    </SettingsSaveBar>
  );

  const aiAssistantHref = workspace?.slug
    ? `/w/${workspace.slug}/settings/support-ai-assistant`
    : null;
  const supportSectionClass = 'overflow-hidden rounded-lg border border-border/70 bg-card transition-colors';
  // Fallback copy shown in the textarea placeholders — reused so the preview
  // reflects what customers will actually see when a field is left blank.
  const ESCALATION_DEFAULT_PLACEHOLDER = 'Let me connect you with a team member who can help further.';
  const ESCALATION_BUSY_PLACEHOLDER = "I've notified the team. Everyone's helping other customers right now - expect a reply within {reply_time}.";
  const ESCALATION_AFTER_HOURS_PLACEHOLDER = "I've passed this on to the team. We're away right now and back {next_open}.";

  const escalationPreviewDefault = previewEscalationMessage(
    escalationMessage || ESCALATION_DEFAULT_PLACEHOLDER,
    replyTimePreset,
    replyTimeCustomMinutes,
  );
  const escalationPreviewBusy = previewEscalationMessage(
    escalationMessageBusy || ESCALATION_BUSY_PLACEHOLDER,
    replyTimePreset,
    replyTimeCustomMinutes,
  );
  const escalationPreviewAfterHours = previewEscalationMessage(
    escalationMessageAfterHours || ESCALATION_AFTER_HOURS_PLACEHOLDER,
    replyTimePreset,
    replyTimeCustomMinutes,
  );
  const businessHoursSettingsHref = workspace?.slug ? `/w/${workspace.slug}/settings/chat-general` : null;

  const aiSetupSection = (
    <>
      <Card className="gap-5 rounded-lg border-border/70">
        <CardHeader className="flex flex-row items-start justify-between gap-4">
          <div className="space-y-1">
            <CardTitle role="heading" aria-level={2} className="text-sm font-semibold text-quiet-text-primary">AI responses</CardTitle>
            <p id="ai-enabled-help" className="text-sm text-quiet-text-secondary">Use your support agent to handle incoming customer messages.</p>
          </div>
          <div className="flex shrink-0 items-center gap-2">
            <Label htmlFor="ai-assistant-enabled" className="text-sm">{aiEnabled && !aiAssistantEnableBlocker ? 'On' : 'Off'}</Label>
            <Switch id="ai-assistant-enabled" aria-label="Enable AI assistant" aria-describedby="ai-enabled-help" checked={aiEnabled && !aiAssistantEnableBlocker} onCheckedChange={handleAIEnabledChange} />
          </div>
        </CardHeader>
        <CardContent className="space-y-5">
          {showAIAssistantEnableBlocker && aiAssistantEnableBlocker && (
            <p role="alert" className="text-sm text-destructive">{aiAssistantEnableBlocker}</p>
          )}
          <div className="grid gap-x-8 gap-y-5 sm:grid-cols-2">
            <div className="min-w-0 space-y-2">
              <Label htmlFor="ai-support-agent">Support agent</Label>
              <AISelect value={aiAgentId} onValueChange={handleAIAgentChange}>
                <AISelectTrigger id="ai-support-agent" aria-label="Support agent" variant="underline" className="w-full"><AISelectValue placeholder="Select a support agent…" /></AISelectTrigger>
                <AISelectContent>
                  <AISelectItem value={NO_AGENT_VALUE}>No support agent selected</AISelectItem>
                  {supportAgents.map((agent) => <AISelectItem key={agent.id} value={agent.id}>{agent.name}</AISelectItem>)}
                </AISelectContent>
              </AISelect>
              {supportAgents.length === 0 && <p className="text-xs text-quiet-text-secondary">Create a support agent before enabling AI responses.</p>}
            </div>
            <div className="min-w-0 space-y-2">
              <Label htmlFor="ai-response-mode">Response mode</Label>
              <AISelect value={aiResponseMode} onValueChange={setAiResponseMode}>
                <AISelectTrigger id="ai-response-mode" aria-label="Response mode" aria-describedby="ai-response-mode-help" variant="underline" className="w-full"><AISelectValue /></AISelectTrigger>
                <AISelectContent>{CHAT_WIDGET_AI_RESPONSE_MODES.map((mode) => <AISelectItem key={mode.value} value={mode.value}>{mode.label}</AISelectItem>)}</AISelectContent>
              </AISelect>
              <p id="ai-response-mode-help" className="text-xs text-quiet-text-secondary">{aiResponseMode === 'ai_first' ? 'Replies are sent directly to the customer.' : 'Suggestions stay private for your team to review.'}</p>
            </div>
            <div className="min-w-0 space-y-2">
              <Label htmlFor="ai-reply-channels">Channels</Label>
              <AIReplyChannelsSelect value={aiReplyChannels} onChange={setAiReplyChannels} />
            </div>
            <div className="min-w-0 space-y-2">
              <Label htmlFor="ai-confidence">Minimum confidence</Label>
              <AISelect value={confidenceThreshold} onValueChange={setConfidenceThreshold}>
                <AISelectTrigger id="ai-confidence" aria-label="Minimum confidence" aria-describedby="ai-confidence-help" variant="underline" className="w-full"><AISelectValue /></AISelectTrigger>
                <AISelectContent>{[0.1, 0.2, 0.3, 0.4, 0.5, 0.6, 0.7, 0.8, 0.9, 1.0].map(v => <AISelectItem key={v} value={String(v)}>{(v * 100).toFixed(0)}%</AISelectItem>)}</AISelectContent>
              </AISelect>
              <p id="ai-confidence-help" className="text-xs text-quiet-text-secondary">Higher confidence means fewer AI answers and more human handoffs.</p>
            </div>
          </div>
        </CardContent>
      </Card>

    </>
  );

  const aiHandoffSection = (
    <>
      <SettingsSection title="Human handoff" description="Reply limits, customer messages, and team routing.">
        <div className="space-y-5 pt-4">
          <div className="grid gap-6 sm:grid-cols-2">
            <div className="min-w-0 space-y-2">
              <Label htmlFor="ai-handoff-limit">Hand off after</Label>
              <AISelect value={String(aiMaxFollowups)} onValueChange={(value) => setAiMaxFollowups(Number(value))}>
                <AISelectTrigger id="ai-handoff-limit" aria-label="Hand off after" aria-describedby="ai-handoff-help" variant="underline" className="w-full"><AISelectValue /></AISelectTrigger>
                <AISelectContent>{[1, 2, 3, 4, 5, 6, 7, 8, 9, 10].map(value => <AISelectItem key={value} value={String(value)}>{formatAIHandoffFollowupOption(value)}</AISelectItem>)}</AISelectContent>
              </AISelect>
              <p id="ai-handoff-help" className="text-xs text-quiet-text-secondary">Counts AI follow-up replies about the same issue. A new issue starts the count again.</p>
            </div>
            <div className="flex items-start justify-between gap-4">
              <div className="space-y-2">
                <Label htmlFor="ai-talk-to-human">Show “Talk to Human”</Label>
                <p className="text-xs text-quiet-text-secondary">Let customers request a teammate from the chat widget.</p>
              </div>
              <Switch id="ai-talk-to-human" checked={showTalkToHuman} onCheckedChange={setShowTalkToHuman} />
            </div>
          </div>
          <div className="min-w-0 space-y-3 border-t border-quiet-divider pt-5">
            <h3 className="text-sm font-semibold text-quiet-text-primary">Customer handoff messages</h3>
            <Tabs defaultValue="default" className="gap-2">
              <div className="min-w-0">
                <TabsList variant="quiet" aria-label="Handoff message type" className="w-full flex-wrap gap-x-5 gap-y-0">
                  {CHAT_WIDGET_ESCALATION_TABS.map((tab) => (
                    <TabsTrigger key={tab.value} value={tab.value} className="flex-none">
                      {tab.label}
                    </TabsTrigger>
                  ))}
                </TabsList>
                <TabsContent value="delayed_team_reply" className="mt-0">
                  <DelayedTeamReplySettings
                    minutes={delayedTeamReplyMinutes}
                    message={delayedTeamReplyMessage}
                    messageNoEmail={delayedTeamReplyMessageNoEmail}
                    onMinutesChange={setDelayedTeamReplyMinutes}
                    onMessageChange={setDelayedTeamReplyMessage}
                    onMessageNoEmailChange={setDelayedTeamReplyMessageNoEmail}
                  />
                </TabsContent>
                <TabsContent value="default" className="mt-0">
                  <Textarea
                    id="escalation-msg"
                    aria-label="Default escalation message"
                    value={escalationMessage}
                    onChange={(e) => setEscalationMessage(e.target.value)}
                    placeholder="Let me connect you with a team member who can help further."
                    rows={3}
                    className="rounded-none border-0 border-b border-quiet-field bg-transparent px-0 shadow-none focus-visible:border-quiet-text-primary focus-visible:ring-0 focus-visible:ring-offset-0"
                  />
                  {/\{(?:reply_time|next_open)\}/.test(escalationMessage || ESCALATION_DEFAULT_PLACEHOLDER) && (<p className="py-2 text-xs text-muted-foreground">
                    Preview: <span className="italic">&ldquo;{escalationPreviewDefault}&rdquo;</span>
                  </p>)}
                </TabsContent>
                <TabsContent value="busy" className="mt-0">
                  <Textarea
                    id="escalation-msg-busy"
                    aria-label="Team busy escalation message"
                    value={escalationMessageBusy}
                    onChange={(e) => setEscalationMessageBusy(e.target.value)}
                    placeholder="I've notified the team. Everyone's helping other customers right now - expect a reply within {reply_time}."
                    rows={3}
                    className="rounded-none border-0 border-b border-quiet-field bg-transparent px-0 shadow-none focus-visible:border-quiet-text-primary focus-visible:ring-0 focus-visible:ring-offset-0"
                  />
                  {/\{(?:reply_time|next_open)\}/.test(escalationMessageBusy || ESCALATION_BUSY_PLACEHOLDER) && (<p className="py-2 text-xs text-muted-foreground">
                    Preview: <span className="italic">&ldquo;{escalationPreviewBusy}&rdquo;</span>
                  </p>)}
                </TabsContent>
                <TabsContent value="after_hours" className="mt-0">
                  <Textarea
                    id="escalation-msg-ah"
                    aria-label="After hours escalation message"
                    value={escalationMessageAfterHours}
                    onChange={(e) => setEscalationMessageAfterHours(e.target.value)}
                    placeholder="I've passed this on to the team. We're away right now and back {next_open}."
                    rows={3}
                    className="rounded-none border-0 border-b border-quiet-field bg-transparent px-0 shadow-none focus-visible:border-quiet-text-primary focus-visible:ring-0 focus-visible:ring-offset-0"
                  />
                  {/\{(?:reply_time|next_open)\}/.test(escalationMessageAfterHours || ESCALATION_AFTER_HOURS_PLACEHOLDER) && (<p className="py-2 text-xs text-muted-foreground">
                    Preview: <span className="italic">&ldquo;{escalationPreviewAfterHours}&rdquo;</span> <span className="text-muted-foreground">(example — actual time depends on your business hours schedule)</span>
                  </p>)}
                  {!businessHoursEnabled && (
                    <p className="px-3 pb-2 text-xs text-muted-foreground">
                      Enable business hours so customers see an accurate return time.
                      {businessHoursSettingsHref ? (
                        <>
                          {' '}
                          <a href={businessHoursSettingsHref} className="font-medium text-primary hover:underline">
                            Set up business hours
                          </a>
                        </>
                      ) : null}
                    </p>
                  )}
                </TabsContent>
              </div>
              <p className="text-xs text-muted-foreground">
                Use <code>{'{reply_time}'}</code> for busy messages and <code>{'{next_open}'}</code> for after-hours messages.
              </p>
            </Tabs>

          </div>
          <div className="space-y-3 border-t border-quiet-divider pt-5">
            <h3 className="text-sm font-semibold text-quiet-text-primary">Team routing</h3>
            <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
              <p className="min-w-0 text-sm text-quiet-text-secondary">{handoffSummaryElement}</p>
              {routingAssignmentHref ? (
                <Button asChild variant="ghost" size="sm" className="shrink-0 self-start"><a href={routingAssignmentHref}>Configure routing &amp; assignment</a></Button>
              ) : <p className="text-xs text-quiet-text-secondary">Open a workspace to configure routing &amp; assignment.</p>}
            </div>
          </div>
        </div>
      </SettingsSection>
      <AIFollowUpSettings enabled={aiFollowUpEnabled} delayHours={aiFollowUpDelay} closeHours={aiFollowUpClose} secondDelayHours={aiFollowUpSecondDelay} onEnabledChange={setAiFollowUpEnabled} onDelayChange={setAiFollowUpDelay} onCloseChange={setAiFollowUpClose} onSecondDelayChange={setAiFollowUpSecondDelay} />
    </>
  );

  const chatWidgetAIAssistantCard = (
    <div className="overflow-hidden rounded-lg border border-border/70 bg-card">
      <div className="flex flex-col gap-4 p-4 sm:flex-row sm:items-center sm:justify-between">
        <div className="flex min-w-0 items-start gap-4">
          <div className="flex h-9 w-9 shrink-0 items-center justify-center rounded-lg bg-muted text-muted-foreground">
            <BotIcon className="h-4 w-4" />
          </div>
          <div className="min-w-0">
            <p className="text-sm font-medium">AI Assistant</p>
            <p className="mt-0.5 text-sm text-muted-foreground">
              AI replies, internal notes, confidence, and handoff settings now live in AI Assistant.
            </p>
            <p className="mt-2 text-xs text-muted-foreground">
              Current mode: {aiEnabled && !aiAssistantEnableBlocker ? (aiResponseMode === 'ai_first' ? 'Reply directly' : 'Add internal note') : 'Off'}
            </p>
          </div>
        </div>
        {aiAssistantHref ? (
          <Button asChild variant="outline" size="sm" className="shrink-0">
            <a href={aiAssistantHref}>Configure</a>
          </Button>
        ) : null}
      </div>
    </div>
  );

  if (isAIAssistantPage) {
    return (
      <div className="space-y-4">
        <SettingsAutosaveGuard isDirty={autosave.isDirty} error={autosave.error} onRetry={autosave.retry} />
        <Tabs defaultValue="setup">
          <div className="sticky top-0 z-10 flex flex-wrap items-center justify-between gap-x-4 gap-y-2 bg-background">
            <TabsList variant="line" aria-label="AI assistant settings" className="max-w-full overflow-x-auto">
              <TabsTrigger value="setup">Setup</TabsTrigger>
              <TabsTrigger value="answers">Preferred answers</TabsTrigger>
              <TabsTrigger value="handoff">Handoff &amp; follow-up</TabsTrigger>
            </TabsList>
            <div className="ml-auto flex min-w-0 max-w-full flex-wrap items-center justify-end gap-3">
              <SettingsSaveStatus status={autosave.status} error={autosave.error} onRetry={autosave.retry} />
              {data?.settings.ai_agent_id && (
                <SupportAIPreview key={`${workspaceId}:${data.settings.ai_agent_id}`} workspaceId={workspaceId} agentId={data.settings.ai_agent_id} />
              )}
            </div>
          </div>
          <TabsContent value="setup" className="mt-4">{aiSetupSection}</TabsContent>
          <TabsContent value="answers" className="mt-4">
            <CuratedGuidanceField
              key={aiAgentId}
              workspaceId={workspaceId}
              agentId={aiAgentId === NO_AGENT_VALUE ? undefined : aiAgentId}
            />
          </TabsContent>
          <TabsContent value="handoff" className="mt-4 space-y-4">{aiHandoffSection}</TabsContent>
        </Tabs>
      </div>
    );
  }

  /* ── Main settings view ────────────────────────────────────────────── */

  return (
    <PreviewLayout preview={previewElement}>
      <div className="flex flex-1 flex-col overflow-auto">
        <div className="flex-1 space-y-3 p-4">
        {saveIndicator}
        {data && <WidgetOriginSettings key={workspaceId} workspaceId={workspaceId} installation={data} canManageSigningSecret={canManageSigningSecret} />}
        {/* Widget Installation */}
        <div className={supportSectionClass}>
          <button
            type="button"
            onClick={() => toggleSection('widget-installation')}
            className="flex w-full items-center gap-4 px-4 py-4 text-left transition-colors hover:bg-muted/40"
          >
            <div className="flex h-9 w-9 shrink-0 items-center justify-center rounded-lg bg-muted text-muted-foreground">
              <CodeIcon className="h-4 w-4" />
            </div>
            <div className="min-w-0 flex-1">
              <p className="text-sm font-medium">2. Install the widget</p>
              <p className="text-sm text-muted-foreground">Embed the chat widget on your website</p>
            </div>
            <ArrowDown01Icon className={cn('h-4 w-4 shrink-0 text-muted-foreground transition-transform duration-200', isExpanded('widget-installation') && 'rotate-180')} />
          </button>
          <div className="accordion-animate" data-open={isExpanded('widget-installation')}>
            <div>
            <div className="border-t border-border px-6 py-6 space-y-4">
              {!widgetKey ? (
                <div className="flex flex-col items-center gap-3 py-6 text-center">
                  <div className="flex h-12 w-12 items-center justify-center rounded-full bg-muted">
                    <Key01Icon className="h-6 w-6 text-muted-foreground" />
                  </div>
                  <div>
                    <p className="text-sm font-medium">No API key found</p>
                    <p className="text-xs text-muted-foreground mt-1">
                      Generate an API key to embed the chat widget on your website.
                    </p>
                  </div>
                  <Button
                    onClick={() => regenerateKeyMutation.mutate(undefined, {
                      onSuccess: () => toast.success('Widget API key generated'),
                      onError: (err: unknown) => toast.error(err instanceof Error ? err.message : 'Failed to generate key'),
                    })}
                    disabled={regenerateKeyMutation.isPending}
                    size="sm"
                  >
                    {regenerateKeyMutation.isPending ? 'Generating...' : 'Generate API Key'}
                  </Button>
                </div>
              ) : (
                <>
                  <div className="space-y-2">
                    <Label className="text-sm">Widget Key</Label>
                    <div className="flex items-center gap-2">
                      <Input value={widgetKey} readOnly className="font-mono text-sm" />
                      <Button type="button" variant="outline" size="icon" className="shrink-0" aria-label="Copy widget key" onClick={() => void copyToClipboard(widgetKey, 'Widget key')}>
                        <Copy01Icon className="h-4 w-4" />
                      </Button>
                    </div>
                  </div>

                  <div className="space-y-3">
                    <Label className="text-sm">Installation Method</Label>
                    <div className="grid grid-cols-2 gap-2 sm:grid-cols-4">
                      {([
                        { id: 'html' as const, label: 'HTML / JS', icon: (
                          <img src="/icons/html-js.svg" alt="HTML/JS" className="h-4 w-4" />
                        )},
                        { id: 'react' as const, label: 'React', icon: (
                          <img src="/icons/react.png" alt="React" className="h-4 w-4" />
                        )},
                        { id: 'vue' as const, label: 'Vue', icon: (
                          <CodeIcon className="h-4 w-4 text-emerald-600 dark:text-emerald-400" />
                        )},
                        { id: 'nextjs' as const, label: 'Next.js', icon: (
                          <img src="/icons/nextjs.svg" alt="Next.js" className="h-4 w-4 dark:invert" />
                        )},
                      ] as const).map(tab => (
                        <button
                          type="button"
                          key={tab.id}
                          onClick={() => setSnippetTab(tab.id)}
                          aria-pressed={snippetTab === tab.id}
                          className={cn(
                            'flex items-center justify-center gap-1.5 rounded-md border px-3 py-2 text-xs transition-colors',
                            snippetTab === tab.id
                              ? 'border-primary/30 bg-primary/5 font-medium text-foreground shadow-sm'
                              : 'border-border/60 bg-background text-muted-foreground hover:border-border hover:text-foreground'
                          )}
                        >
                          {tab.icon}
                          {tab.label}
                        </button>
                      ))}
                    </div>
                    <Tabs defaultValue="manual" className="gap-3">
                      <TabsList className="grid h-9 w-full grid-cols-2 bg-muted/60 p-1">
                        <TabsTrigger value="manual" className="h-7 text-xs">Manual setup</TabsTrigger>
                        <TabsTrigger value="ai" className="h-7 text-xs">Install with AI</TabsTrigger>
                      </TabsList>
                      <TabsContent value="manual" className="mt-0 space-y-2">
                        <CodeBlock
                          code={
                            snippetTab === 'html' ? htmlSnippet
                            : snippetTab === 'react' ? reactSnippet
                            : snippetTab === 'vue' ? vueSnippet
                            : nextjsSnippet
                          }
                          language={snippetTab === 'html' ? 'markup' : 'typescript'}
                          showLineNumbers
                        />
                        <p className="text-xs text-muted-foreground">
                          {snippetTab === 'html'
                            ? 'Add the pixel once, then identify logged-in users when their session is ready.'
                            : snippetTab === 'react'
                            ? 'Install the React SDK and place HelpinProvider at the application root.'
                            : snippetTab === 'vue'
                            ? 'Install the Vue SDK and register HelpinPlugin in your application bootstrap.'
                            : 'Initialize Helpin from a Client Component so server rendering remains safe.'}
                        </p>
                      </TabsContent>
                      <TabsContent value="ai" className="mt-0">
                        <WidgetInstallAIPrompt
                          prompt={installPrompt}
                          onCopy={() => void copyToClipboard(installPrompt, 'AI installation prompt')}
                        />
                      </TabsContent>
                    </Tabs>
                  </div>
                </>
              )}
            </div>
            </div>
          </div>
        </div>

        {/* Identity Capture */}
        <div className={supportSectionClass}>
          <button
            type="button"
            onClick={() => toggleSection('identity-capture')}
            className="flex w-full items-center gap-4 px-4 py-4 text-left transition-colors hover:bg-muted/40"
          >
            <div className="flex h-9 w-9 shrink-0 items-center justify-center rounded-lg bg-muted text-muted-foreground">
              <Message01Icon className="h-4 w-4" />
            </div>
            <div className="min-w-0 flex-1">
              <p className="text-sm font-medium">Contact details</p>
              <p className="text-sm text-muted-foreground">Choose which contact details to ask for when visitors request human support.</p>
            </div>
            <ArrowDown01Icon className={cn('h-4 w-4 shrink-0 text-muted-foreground transition-transform duration-200', isExpanded('identity-capture') && 'rotate-180')} />
          </button>
          <div className="accordion-animate" data-open={isExpanded('identity-capture')}>
            <div>
            <div className="border-t border-border px-6 py-6 space-y-4">
              <div className="flex items-center justify-between">
                <div>
                  <Label className="text-sm">Ask for email before human support</Label>
                  <p className="text-xs text-muted-foreground">Show an email prompt when visitors request a teammate. Visitors whose email is already known are not asked again.</p>
                </div>
                <Switch checked={requireEmail} onCheckedChange={setRequireEmail} />
              </div>

              <div className="flex items-center justify-between">
                <div>
                  <Label className="text-sm">Also ask for a phone number</Label>
                  <p className="text-xs text-muted-foreground">Show an optional phone number prompt after the email step.</p>
                </div>
                <Switch checked={requirePhone} onCheckedChange={setRequirePhone} disabled={!requireEmail} />
              </div>

              <div className="flex items-center justify-between">
                <div>
                  <Label className="text-sm">Make email required</Label>
                  <p className="text-xs text-muted-foreground">Visitors cannot skip the email step. When off, they can continue without email. Phone number remains optional.</p>
                </div>
                <Switch checked={forceVisitorIdentity} onCheckedChange={setForceVisitorIdentity} disabled={!requireEmail} />
              </div>

              <WelcomeMessageSettings
                value={welcomeMessage}
                aiFirst={aiFirst}
                onChange={setWelcomeMessage}
                onFocus={() => setPreviewInitialView('conversation')}
              />
            </div>
            </div>
          </div>
        </div>

        <div className={supportSectionClass}>
          <button
            type="button"
            onClick={() => toggleSection('privacy')}
            aria-expanded={isExpanded('privacy')}
            className="flex w-full items-center gap-4 px-4 py-4 text-left transition-colors hover:bg-muted/40"
          >
            <div className="flex h-9 w-9 shrink-0 items-center justify-center rounded-lg bg-muted text-muted-foreground">
              <Shield01Icon className="h-4 w-4" />
            </div>
            <div className="min-w-0 flex-1">
              <p className="text-sm font-medium">Privacy notice</p>
              <p className="text-sm text-muted-foreground">Link your privacy policy before visitors start chatting.</p>
            </div>
            <ArrowDown01Icon className={cn('h-4 w-4 shrink-0 text-muted-foreground transition-transform duration-200', isExpanded('privacy') && 'rotate-180')} />
          </button>
          <div className="accordion-animate" data-open={isExpanded('privacy')}>
            <div>
              <div className="border-t border-border px-6 py-6">
                <PrivacyNoticeSettings
                  enabled={privacyNoticeEnabled}
                  policyUrl={privacyPolicyUrl}
                  text={privacyNoticeText}
                  error={privacyNoticeError}
                  onEnabledChange={setPrivacyNoticeEnabled}
                  onPolicyUrlChange={setPrivacyPolicyUrl}
                  onTextChange={setPrivacyNoticeText}
                />
              </div>
            </div>
          </div>
        </div>

        {/* Appearance */}
        <div className={supportSectionClass}>
          <button
            type="button"
            onClick={() => toggleSection('appearance')}
            className="flex w-full items-center gap-4 px-4 py-4 text-left transition-colors hover:bg-muted/40"
          >
            <div className="flex h-9 w-9 shrink-0 items-center justify-center rounded-lg bg-muted text-muted-foreground">
              <Image01Icon className="h-4 w-4" />
            </div>
            <div className="min-w-0 flex-1">
              <p className="text-sm font-medium">Appearance</p>
              <p className="text-sm text-muted-foreground">Customize the widget's visual appearance</p>
            </div>
            <ArrowDown01Icon className={cn('h-4 w-4 shrink-0 text-muted-foreground transition-transform duration-200', isExpanded('appearance') && 'rotate-180')} />
          </button>
          <div className="accordion-animate" data-open={isExpanded('appearance')}>
            <div>
            <div className="border-t border-border px-6 py-6 space-y-6">
              {/* Widget Identity */}
              <div className="space-y-3">
                <div>
                  <Label className="text-sm font-medium">Widget Identity</Label>
                  <p className="text-xs text-muted-foreground mt-0.5">
                    Customize the name and avatar your customers see in the widget.
                  </p>
                </div>
                <div className="flex items-start gap-4">
                  <div className="shrink-0">
                    {widgetAvatarUrl ? (
                      <div className="relative group">
                        <img
                          src={widgetAvatarUrl}
                          alt="Widget avatar"
                          className="h-14 w-14 rounded-full border object-cover cursor-pointer"
                          onClick={() => avatarInputRef.current?.click()}
                        />
                        <button
                          onClick={() => setWidgetAvatarUrl('')}
                          className="absolute -top-1 -right-1 h-5 w-5 rounded-full bg-destructive text-destructive-foreground flex items-center justify-center text-xs opacity-0 group-hover:opacity-100 transition-opacity"
                        >
                          &times;
                        </button>
                      </div>
                    ) : (
                      <div
                        className="h-14 w-14 rounded-full border-2 border-dashed flex items-center justify-center cursor-pointer hover:border-muted-foreground/50 transition-colors"
                        onClick={() => avatarInputRef.current?.click()}
                      >
                        <span className="text-lg font-semibold text-muted-foreground/50">
                          {(widgetName || workspace?.name || 'S').charAt(0).toUpperCase()}
                        </span>
                      </div>
                    )}
                    <input
                      ref={avatarInputRef}
                      type="file"
                      accept="image/png,image/jpeg,image/svg+xml"
                      className="hidden"
                      onChange={(e) => {
                        const file = e.target.files?.[0];
                        if (file) {
                          if (!file.type.startsWith('image/')) {
                            toast.error('Please select an image file');
                            return;
                          }
                          if (file.size > 2 * 1024 * 1024) {
                            toast.error('Image must be under 2MB');
                            return;
                          }
                          const reader = new FileReader();
                          reader.onload = () => setWidgetAvatarUrl(reader.result as string);
                          reader.readAsDataURL(file);
                        }
                        e.target.value = '';
                      }}
                    />
                  </div>
                  <div className="flex-1 space-y-1.5">
                    <Label className="text-xs text-muted-foreground">Display Name</Label>
                    <Input
                      value={widgetName}
                      onChange={(e) => setWidgetName(e.target.value)}
                      placeholder={workspace?.name || 'Support'}
                      className="text-sm"
                    />
                    <p className="text-xs text-muted-foreground">
                      Shown in the widget header and conversations. Defaults to your workspace name.
                    </p>
                  </div>
                </div>
              </div>

              {/* Logo */}
              <div className="space-y-3">
                <div>
                  <Label className="text-sm font-medium">Logo</Label>
                  <p className="text-xs text-muted-foreground mt-0.5">
                    The image that appears at the top of your widget. Recommended size: 512x512 pixels.
                  </p>
                </div>
                {logoUrl ? (
                  <div className="flex items-center gap-4">
                    <img
                      src={logoUrl}
                      alt="Widget logo"
                      className="h-16 w-16 rounded-lg border object-cover"
                    />
                    <div className="flex gap-2">
                      <Button variant="outline" size="sm" onClick={() => logoInputRef.current?.click()}>
                        Change
                      </Button>
                      <Button variant="outline" size="sm" onClick={() => setLogoUrl('')}>
                        Remove
                      </Button>
                    </div>
                  </div>
                ) : (
                  <div
                    className={cn(
                      'flex flex-col items-center justify-center gap-2 rounded-lg border-2 border-dashed p-6 transition-colors cursor-pointer',
                      dragOver ? 'border-primary bg-primary/5' : 'border-border hover:border-muted-foreground/50'
                    )}
                    onClick={() => logoInputRef.current?.click()}
                    onDragOver={handleDragOver}
                    onDragLeave={handleDragLeave}
                    onDrop={handleDrop}
                  >
                    <Image01Icon className="h-6 w-6 text-muted-foreground/50" />
                    <div className="text-center">
                      <p className="text-sm text-muted-foreground">
                        Drop an image or{' '}
                        <span className="text-primary font-medium">click to browse</span>
                      </p>
                      <p className="text-xs text-muted-foreground mt-1">Supports PNG, JPG, SVG</p>
                    </div>
                  </div>
                )}
                <input
                  ref={logoInputRef}
                  type="file"
                  accept="image/png,image/jpeg,image/svg+xml"
                  className="hidden"
                  onChange={(e) => {
                    const file = e.target.files?.[0];
                    if (file) handleLogoFileSelect(file);
                    e.target.value = '';
                  }}
                />
              </div>

              {/* Colors — 2-col */}
              <div className="grid grid-cols-2 gap-x-6 gap-y-4">
                <div className="space-y-2">
                  <Label className="text-sm font-medium">Primary color</Label>
                  <p className="text-xs text-muted-foreground mt-0.5">Links and accents in the widget.</p>
                  <BrandColorPicker value={brandColor} onChange={setBrandColor} />
                </div>
                <div className="space-y-2">
                  <Label className="text-sm font-medium">Color Scheme</Label>
                  <p className="text-xs text-muted-foreground mt-0.5">Default theme for the widget.</p>
                  <div className="flex items-center gap-1 p-1 rounded-lg border bg-muted/30 w-fit">
                    {COLOR_SCHEME_OPTIONS.map(opt => (
                      <button
                        key={opt.value}
                        onClick={() => setColorScheme(opt.value)}
                        className={cn(
                          'flex items-center gap-1.5 px-3 py-1.5 rounded-md text-sm transition-colors',
                          colorScheme === opt.value
                            ? 'bg-background shadow-sm font-medium text-foreground'
                            : 'text-muted-foreground hover:text-foreground'
                        )}
                      >
                        <opt.icon className="h-3.5 w-3.5" />
                        {opt.label}
                      </button>
                    ))}
                  </div>
                </div>
              </div>

              {/* Button colors — 2-col */}
              <div className="grid grid-cols-2 gap-x-6 gap-y-4">
                <div className="space-y-2">
                  <Label className="text-sm font-medium">Button color</Label>
                  <p className="text-xs text-muted-foreground mt-0.5">Floating button background.</p>
                  <BrandColorPicker value={buttonColor} onChange={setButtonColor} />
                </div>
                <div className="space-y-2">
                  <Label className="text-sm font-medium">Button icon color</Label>
                  <p className="text-xs text-muted-foreground mt-0.5">Floating button icon.</p>
                  <BrandColorPicker value={buttonIconColor} onChange={setButtonIconColor} />
                </div>
              </div>

              {/* Launcher — 2-col */}
              <div className="grid grid-cols-2 gap-x-6 gap-y-4">
                <div className="space-y-2">
                  <Label className="text-sm font-medium">Launcher Position</Label>
                  <Select value={launcherPosition} onValueChange={setLauncherPosition}>
                    <SelectTrigger>
                      <SelectValue />
                    </SelectTrigger>
                    <SelectContent>
                      <SelectItem value="bottom_right">Bottom Right</SelectItem>
                      <SelectItem value="bottom_left">Bottom Left</SelectItem>
                    </SelectContent>
                  </Select>
                </div>
                <div className="space-y-2">
                  <Label className="text-sm font-medium">Launcher Icon</Label>
                  <div className="flex gap-1.5">
                    {ICON_OPTIONS.map(opt => (
                      <button
                        key={opt.value}
                        type="button"
                        title={opt.label}
                        onClick={() => setLauncherIcon(opt.value)}
                        className={cn(
                          'flex items-center justify-center rounded-md border p-2 transition-colors',
                          launcherIcon === opt.value
                            ? 'border-primary bg-primary/5 text-primary'
                            : 'border-border text-muted-foreground hover:border-primary/40 hover:text-foreground'
                        )}
                      >
                        <opt.icon className="h-4.5 w-4.5" />
                      </button>
                    ))}
                  </div>
                </div>
              </div>

              {/* Show Powered By */}
              <div className="flex items-center justify-between pt-2">
                <div>
                  <Label className="text-sm font-medium">Show "Powered by Helpin"</Label>
                  <p className="text-xs text-muted-foreground mt-0.5">
                    {brandingDescription(billing)}
                  </p>
                </div>
                <Switch
                  checked={effectiveShowBranding}
                  onCheckedChange={(checked) => {
                    if (canRemoveBranding) {
                      setShowBranding(checked);
                    } else {
                      setShowBranding(true);
                    }
                  }}
                  disabled={!canRemoveBranding}
                />
              </div>
            </div>
            </div>
          </div>
        </div>

        {/* Help Center */}
        <div className={supportSectionClass}>
          <button
            type="button"
            onClick={() => toggleSection('help-center')}
            className="flex w-full items-center gap-4 px-4 py-4 text-left transition-colors hover:bg-muted/40"
          >
            <div className="flex h-9 w-9 shrink-0 items-center justify-center rounded-lg bg-muted text-muted-foreground">
              <HelpCircleIcon className="h-4 w-4" />
            </div>
            <div className="min-w-0 flex-1">
              <p className="text-sm font-medium">Help Center</p>
              <p className="text-sm text-muted-foreground">Select docs spaces for the widget Help tab</p>
            </div>
            <ArrowDown01Icon className={cn('h-4 w-4 shrink-0 text-muted-foreground transition-transform duration-200', isExpanded('help-center') && 'rotate-180')} />
          </button>
          <div className="accordion-animate" data-open={isExpanded('help-center')}>
            <div>
            <div className="border-t border-border px-6 py-6 space-y-4">
              {docsSpacesLoading && (
                <p className="text-sm text-muted-foreground">Loading available spaces...</p>
              )}

              {!docsSpacesLoading && externalDocsSpaces.length === 0 && (
                <p className="text-sm text-muted-foreground">
                  Create an external space in Docs to show articles in the widget.
                </p>
              )}

              {!docsSpacesLoading && externalDocsSpaces.length > 0 && (
                <div className="space-y-3">
                  {externalDocsSpaces.map((space) => {
                    const enabled = widgetHelpSpaceIds.includes(space.id);
                    return (
                      <div key={space.id} className="flex items-center justify-between gap-4 rounded-lg border p-4">
                        <div className="min-w-0">
                          <p className="text-sm font-medium">{space.name}</p>
                          <p className="text-xs text-muted-foreground">
                            /{space.slug}
                          </p>
                        </div>
                        <Switch
                          checked={enabled}
                          onCheckedChange={(checked) => toggleHelpSpace(space.id, checked)}
                        />
                      </div>
                    );
                  })}
                </div>
              )}
            </div>
            </div>
          </div>
        </div>
          {chatWidgetAIAssistantCard}

          {/* Availability */}
          <div className={supportSectionClass}>
            <button
              type="button"
              onClick={() => toggleSection('business-hours')}
              className="flex w-full items-center gap-4 px-4 py-4 text-left transition-colors hover:bg-muted/40"
            >
              <div className="flex h-9 w-9 shrink-0 items-center justify-center rounded-lg bg-muted text-muted-foreground">
                <Message01Icon className="h-4 w-4" />
              </div>
              <div className="min-w-0 flex-1">
                <p className="text-sm font-medium">Availability</p>
                <p className="text-sm text-muted-foreground">Set when your team appears online and what visitors should expect</p>
              </div>
              <ArrowDown01Icon className={cn('h-4 w-4 shrink-0 text-muted-foreground transition-transform duration-200', isExpanded('business-hours') && 'rotate-180')} />
            </button>
            <div className="accordion-animate" data-open={isExpanded('business-hours')}>
              <div>
              <div className="border-t border-border px-6 py-6 space-y-4">
                <div className="flex items-center justify-between">
                  <div>
                    <Label className="text-sm">Enable availability schedule</Label>
                    <p className="text-xs text-muted-foreground">Human team availability and reply expectations follow this schedule after AI handoff.</p>
                  </div>
                  <Switch checked={businessHoursEnabled} onCheckedChange={setBusinessHoursEnabled} />
                </div>

                {businessHoursEnabled && (
                  <>
                    <div className="space-y-2">
                      <Label className="text-sm">Timezone</Label>
                      <Select value={timezone} onValueChange={setTimezone}>
                        <SelectTrigger className="w-64">
                          <SelectValue />
                        </SelectTrigger>
                        <SelectContent>
                          {COMMON_TIMEZONES.map(tz => (
                            <SelectItem key={tz} value={tz}>{tz.replace(/_/g, ' ')}</SelectItem>
                          ))}
                        </SelectContent>
                      </Select>
                    </div>

                    <div className="space-y-2">
                      <Label className="text-sm">Schedule</Label>
                      <div className="space-y-1.5">
                        {DAYS.map(({ key, label }) => {
                          const day = normalizeBusinessHoursDay(schedule[key]);
                          return (
                            <div key={key} className="flex items-center gap-3">
                              <div className="w-24">
                                <Switch
                                  checked={day.enabled}
                                  onCheckedChange={(v) => updateDay(key, { enabled: v })}
                                />
                                <span className="ml-2 text-sm">{label.slice(0, 3)}</span>
                              </div>
                              <Input
                                type="time"
                                value={day.start}
                                onChange={(e) => updateDay(key, { start: e.target.value })}
                                disabled={!day.enabled}
                                className="w-28 h-8 text-sm"
                              />
                              <span className="text-xs text-muted-foreground">to</span>
                              <Input
                                type="time"
                                value={day.end}
                                onChange={(e) => updateDay(key, { end: e.target.value })}
                                disabled={!day.enabled}
                                className="w-28 h-8 text-sm"
                              />
                            </div>
                          );
                        })}
                      </div>
                    </div>

                    <div className="space-y-2">
                      <Label htmlFor="outside-msg" className="text-sm">Outside Hours Message</Label>
                      <Textarea
                        id="outside-msg"
                        value={outsideMessage}
                        onChange={(e) => setOutsideMessage(e.target.value)}
                        placeholder="We're currently offline. Leave a message and we'll get back to you!"
                        rows={2}
                      />
                    </div>
                  </>
                )}

                {/* Reply expectations — during business hours, what does the customer see? */}
                <div className="space-y-3 rounded-md border border-dashed border-border/60 p-4">
                  <div>
                    <Label className="text-sm">Reply expectations</Label>
                    <p className="text-xs text-muted-foreground">During business hours, tell customers when to expect a reply.</p>
                  </div>
                  <div className="grid grid-cols-1 gap-3 sm:grid-cols-[200px_1fr]">
                    <Label htmlFor="reply-time-preset" className="sm:pt-2 text-xs">Preset</Label>
                    <Select value={replyTimePreset} onValueChange={(v) => setReplyTimePreset(v)}>
                      <SelectTrigger id="reply-time-preset" className="w-full">
                        <SelectValue />
                      </SelectTrigger>
                      <SelectContent>
                        <SelectItem value="few_minutes">Usually a few minutes</SelectItem>
                        <SelectItem value="few_hours">Usually a few hours</SelectItem>
                        <SelectItem value="same_day">Within a day</SelectItem>
                        <SelectItem value="custom">Custom…</SelectItem>
                      </SelectContent>
                    </Select>
                    {replyTimePreset === 'custom' && (
                      <>
                        <Label htmlFor="reply-time-custom-minutes" className="sm:pt-2 text-xs">Custom time</Label>
                        <div className="flex items-center gap-2">
                          <Input
                            id="reply-time-custom-minutes"
                            type="number"
                            min={1}
                            max={10080}
                            value={replyTimeCustomMinutes ?? ''}
                            onChange={(e) => {
                              const v = e.target.value;
                              setReplyTimeCustomMinutes(v === '' ? null : Number(v));
                            }}
                            className="w-32 h-8 text-sm"
                            placeholder="30"
                          />
                          <span className="text-xs text-muted-foreground">minutes</span>
                        </div>
                      </>
                    )}
                    <Label className="sm:pt-2 text-xs">Preview</Label>
                    <div className="rounded-md bg-muted/40 px-3 py-2 text-xs text-muted-foreground">
                      “{formatReplyTimeCopy(replyTimePreset, replyTimeCustomMinutes ?? 0)}”
                    </div>
                  </div>
                </div>

                {/* Outage / maintenance banner — optional, renders as a slim amber banner in widget */}
                <div className="space-y-3 rounded-md border border-dashed border-border/60 p-4">
                  <div>
                    <Label className="text-sm">Outage / maintenance banner</Label>
                    <p className="text-xs text-muted-foreground">
                      Show a temporary notice across the widget when something is off. Leave empty to hide.
                    </p>
                  </div>
                  <Textarea
                    value={specialNoticeText}
                    onChange={(e) => {
                      const next = e.target.value;
                      if (next.length <= SPECIAL_NOTICE_MAX_LENGTH) {
                        setSpecialNoticeText(next);
                      }
                    }}
                    placeholder="Our team is catching up on a backlog — replies may be slower today."
                    rows={3}
                    maxLength={SPECIAL_NOTICE_MAX_LENGTH}
                  />
                  <div className="flex justify-end text-xs text-muted-foreground">
                    {specialNoticeText.length} / {SPECIAL_NOTICE_MAX_LENGTH}
                  </div>
                </div>
              </div>
              </div>
            </div>
          </div>

          {/* Chat Features — merged CSAT, File Uploads, Email */}
          <div className={supportSectionClass}>
            <button
              type="button"
              onClick={() => toggleSection('chat-features')}
              className="flex w-full items-center gap-4 px-4 py-4 text-left transition-colors hover:bg-muted/40"
            >
              <div className="flex h-9 w-9 shrink-0 items-center justify-center rounded-lg bg-muted text-muted-foreground">
                <StarIcon className="h-4 w-4" />
              </div>
              <div className="min-w-0 flex-1">
                <p className="text-sm font-medium">Chat Features</p>
                <p className="text-sm text-muted-foreground">File uploads, satisfaction surveys, and fallback emails</p>
              </div>
              <ArrowDown01Icon className={cn('h-4 w-4 shrink-0 text-muted-foreground transition-transform duration-200', isExpanded('chat-features') && 'rotate-180')} />
            </button>
            <div className="accordion-animate" data-open={isExpanded('chat-features')}>
              <div>
              <div className="border-t border-border px-6 py-6 space-y-4">
                <div className="flex items-center justify-between">
                  <div>
                    <Label className="text-sm">File uploads</Label>
                    <p className="text-xs text-muted-foreground">Allow visitors to upload images, documents, and other files (max 100 MB per file).</p>
                  </div>
                  <Switch checked={fileUploadsEnabled} onCheckedChange={setFileUploadsEnabled} />
                </div>

                <div className="flex items-center justify-between">
                  <div>
                    <div className="flex items-center gap-2">
                      <Label className="text-sm">CSAT surveys</Label>
                      <span className="rounded-full bg-muted px-2 py-0.5 text-[11px] font-medium text-muted-foreground">
                        Coming soon
                      </span>
                    </div>
                    <p className="text-xs text-muted-foreground">Satisfaction surveys after conversation resolution will be available soon.</p>
                  </div>
                  <Switch checked={csatEnabled} onCheckedChange={setCsatEnabled} disabled />
                </div>

                <div className="border-t border-border pt-4 space-y-4">
                  <div>
                    <div>
                      <Label className="text-sm">Email notifications</Label>
                      <p className="text-xs text-muted-foreground">Fallback emails are sent automatically if the visitor disconnects before your team replies.</p>
                    </div>
                  </div>

                  <div className="space-y-2">
                    <Label htmlFor="email-fallback-from-name" className="text-sm">From name</Label>
                    <Input
                      id="email-fallback-from-name"
                      value={emailFallbackFromName}
                      onChange={(e) => setEmailFallbackFromName(e.target.value)}
                      placeholder={workspace?.name || 'Workspace name'}
                      className="max-w-md"
                    />
                  </div>

                  <div className="grid max-w-md gap-4 sm:grid-cols-2">
                    <div className="space-y-2">
                      <Label htmlFor="email-fallback-delay" className="text-sm">Delay before sending</Label>
                      <Input
                        id="email-fallback-delay"
                        type="number"
                        min={10}
                        max={600}
                        value={emailFallbackDelaySecs}
                        onChange={(e) => setEmailFallbackDelaySecs(Number(e.target.value) || DEFAULT_EMAIL_FALLBACK_DELAY_SECS)}
                      />
                      <p className="text-xs text-muted-foreground">Seconds after the latest team reply.</p>
                    </div>

                    <div className="space-y-2">
                      <Label htmlFor="email-fallback-send-window" className="text-sm">Send window</Label>
                      <Input
                        id="email-fallback-send-window"
                        type="number"
                        min={2}
                        max={30}
                        value={emailFallbackMaxDeliveryAgeMins}
                        onChange={(e) => setEmailFallbackMaxDeliveryAgeMins(Number(e.target.value) || 10)}
                      />
                      <p className="text-xs text-muted-foreground">Minutes before an unread reply becomes too old to email.</p>
                    </div>
                  </div>
                </div>
              </div>
              </div>
            </div>
          </div>
        </div>

      </div>
    </PreviewLayout>
  );
}
