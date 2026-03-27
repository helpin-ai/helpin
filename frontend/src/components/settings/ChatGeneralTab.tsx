import { useEffect, useRef, useState, type ReactNode } from 'react';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Switch } from '@/components/ui/switch';
import { Skeleton } from '@/components/ui/skeleton';
import { Textarea } from '@/components/ui/textarea';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { toast } from 'sonner';
import { Check, Copy, Code, Loader2, MessageSquare, HelpCircle, CircleHelp, ImageIcon, Monitor, Sun, Moon, KeyRound, Bot, ChevronDown, Star } from 'lucide-react';
import { useChatSettings, useUpdateChatSettings, useRegenerateWidgetKey, useDocsSpaces } from '@/hooks/queries';
import { useSupportAgents } from '@/hooks/queries/useSupport';
import { useWorkspaceTeams } from '@/hooks/useWorkspaceTeams';
import { WidgetPreview } from './WidgetPreview';
import { CodeBlock } from '@/components/ui/code-block';
import { BrandColorPicker } from '@/components/pm/ColorPicker';
import { useWorkspaceStore } from '@/stores/workspaceStore';
import { cn } from '@/lib/utils';
import type { BusinessHoursDay, SupportInboxSettings } from '@/lib/pmTypes';
import type { WidgetConfig } from '@helpin/widget-core';

const ICON_OPTIONS = [
  { value: 'chat_bubble', label: 'Chat Bubble', icon: MessageSquare },
  { value: 'question_mark', label: 'Question Mark', icon: HelpCircle },
  { value: 'help', label: 'Help', icon: CircleHelp },
];


const COLOR_SCHEME_OPTIONS = [
  { value: 'system', label: 'System', icon: Monitor },
  { value: 'light', label: 'Light', icon: Sun },
  { value: 'dark', label: 'Dark', icon: Moon },
];

const DAYS = [
  { key: 'mon', label: 'Monday' },
  { key: 'tue', label: 'Tuesday' },
  { key: 'wed', label: 'Wednesday' },
  { key: 'thu', label: 'Thursday' },
  { key: 'fri', label: 'Friday' },
  { key: 'sat', label: 'Saturday' },
  { key: 'sun', label: 'Sunday' },
];

const COMMON_TIMEZONES = [
  'America/New_York',
  'America/Chicago',
  'America/Denver',
  'America/Los_Angeles',
  'America/Anchorage',
  'Pacific/Honolulu',
  'Europe/London',
  'Europe/Berlin',
  'Europe/Paris',
  'Asia/Tokyo',
  'Asia/Shanghai',
  'Asia/Kolkata',
  'Australia/Sydney',
  'Pacific/Auckland',
  'UTC',
];

const NO_AGENT_VALUE = '__none__';
const DEFAULT_ONLINE_REPLY_TEXT = 'We typically reply in a few minutes';
const DEFAULT_BUSINESS_HOURS_DAY: BusinessHoursDay = { start: '09:00', end: '17:00', enabled: false };

type ChatSettingsDraft = Omit<SupportInboxSettings, 'ai_agent_id'> & {
  ai_agent_id: string;
};

function parseTimeToMinutes(value: string): number {
  const [hours = '0', minutes = '0'] = value.split(':');
  return Number(hours) * 60 + Number(minutes);
}

function weekdayKey(date: Date, timezone: string): string {
  const day = new Intl.DateTimeFormat('en-US', { timeZone: timezone, weekday: 'short' }).format(date).toLowerCase();
  return day.slice(0, 3);
}

function zonedMinutes(date: Date, timezone: string): number {
  const parts = new Intl.DateTimeFormat('en-US', {
    timeZone: timezone,
    hour: '2-digit',
    minute: '2-digit',
    hour12: false,
  }).formatToParts(date);
  const hour = Number(parts.find((part) => part.type === 'hour')?.value ?? '0');
  const minute = Number(parts.find((part) => part.type === 'minute')?.value ?? '0');
  return (hour * 60) + minute;
}

function buildPreviewAvailability(
  businessHoursEnabled: boolean,
  timezone: string,
  schedule: Record<string, BusinessHoursDay>,
  outsideMessage: string,
): WidgetConfig['availability'] {
  const fallbackMessage = outsideMessage || "We're currently offline. Leave a message and we'll get back to you!";
  if (!businessHoursEnabled) {
    return {
      isOnline: true,
      statusText: 'Online now',
      replyTimeText: DEFAULT_ONLINE_REPLY_TEXT,
    };
  }

  try {
    const now = new Date();
    const day = schedule[weekdayKey(now, timezone)];
    const nowMinutes = zonedMinutes(now, timezone);
    const withinHours = Boolean(
      day?.enabled
      && nowMinutes >= parseTimeToMinutes(day.start)
      && nowMinutes < parseTimeToMinutes(day.end),
    );

    if (withinHours) {
      return {
        isOnline: true,
        statusText: 'Online now',
        replyTimeText: DEFAULT_ONLINE_REPLY_TEXT,
      };
    }

    return {
      isOnline: false,
      statusText: 'Offline now',
      replyTimeText: fallbackMessage,
      outsideHoursMessage: fallbackMessage,
    };
  } catch {
    return {
      isOnline: true,
      statusText: 'Online now',
      replyTimeText: DEFAULT_ONLINE_REPLY_TEXT,
    };
  }
}

function normalizeBusinessHoursDay(day?: Partial<BusinessHoursDay> | null): BusinessHoursDay {
  return {
    start: day?.start ?? DEFAULT_BUSINESS_HOURS_DAY.start,
    end: day?.end ?? DEFAULT_BUSINESS_HOURS_DAY.end,
    enabled: day?.enabled ?? DEFAULT_BUSINESS_HOURS_DAY.enabled,
  };
}

function normalizeBusinessHoursSchedule(
  schedule?: Record<string, BusinessHoursDay> | null,
): Record<string, BusinessHoursDay> {
  return DAYS.reduce<Record<string, BusinessHoursDay>>((acc, day) => {
    acc[day.key] = normalizeBusinessHoursDay(schedule?.[day.key]);
    return acc;
  }, {});
}

function sortHelpSpaceIds(ids?: string[] | null): string[] {
  return [...(ids ?? [])].sort();
}

function buildSettingsDraftFromServer(settings: SupportInboxSettings): ChatSettingsDraft {
  return {
    ...settings,
    ai_agent_id: settings.ai_agent_id ?? '',
    business_hours_schedule: normalizeBusinessHoursSchedule(settings.business_hours_schedule),
    widget_help_space_ids: sortHelpSpaceIds(settings.widget_help_space_ids),
  };
}

function serializeSettingsDraft(draft: ChatSettingsDraft): string {
  return JSON.stringify(draft);
}

/* ── Two-column layout shell ─────────────────────────────────────────── */

function PreviewLayout({ children, preview }: { children: ReactNode; preview: ReactNode }) {
  return (
    <div className="grid h-full grid-cols-1 xl:grid-cols-[minmax(0,1fr)_420px]">
      <div className="flex min-w-0 flex-col overflow-hidden">
        {children}
      </div>

      <aside className="hidden self-start xl:block xl:sticky xl:top-0">
        <div className="h-[calc(100svh-8rem)] min-h-[36rem] overflow-y-auto overscroll-contain">
          <div className="h-full pr-1">
            {preview}
          </div>
        </div>
      </aside>
    </div>
  );
}

/* ── Main component ──────────────────────────────────────────────────── */

export function ChatGeneralTab({ workspaceId }: { workspaceId: string }) {
  const workspace = useWorkspaceStore((s) => s.currentWorkspace);
  const { data, isLoading } = useChatSettings(workspaceId);
  const { data: docsSpaces = [], isLoading: docsSpacesLoading } = useDocsSpaces(workspaceId);
  const updateMutation = useUpdateChatSettings(workspaceId);
  const regenerateKeyMutation = useRegenerateWidgetKey(workspaceId);
  const { teams } = useWorkspaceTeams(workspaceId);
  const { data: supportAgents = [] } = useSupportAgents(workspaceId);

  const [snippetTab, setSnippetTab] = useState<'basic' | 'advanced'>('basic');

  // Identity state
  const [requireEmail, setRequireEmail] = useState(true);
  const [requirePhone, setRequirePhone] = useState(false);
  const [welcomeMessage, setWelcomeMessage] = useState('');

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
  const [aiAgentId, setAiAgentId] = useState(NO_AGENT_VALUE);
  const [confidenceThreshold, setConfidenceThreshold] = useState('0.7');
  const [aiResponseMode, setAiResponseMode] = useState('off');
  const [aiMaxFollowups, setAiMaxFollowups] = useState(3);
  const [showTalkToHuman, setShowTalkToHuman] = useState(true);
  const [escalationMessage, setEscalationMessage] = useState('Let me connect you with a team member who can help further.');
  const [handoffBehavior, setHandoffBehavior] = useState('unassigned');
  const [handoffTeamId, setHandoffTeamId] = useState<string | null>(null);
  const [businessHoursEnabled, setBusinessHoursEnabled] = useState(false);
  const [timezone, setTimezone] = useState('America/New_York');
  const [schedule, setSchedule] = useState<Record<string, BusinessHoursDay>>({});
  const [outsideMessage, setOutsideMessage] = useState('');
  const [emailFallbackDelaySecs, setEmailFallbackDelaySecs] = useState(120);
  const [emailFallbackFromName, setEmailFallbackFromName] = useState('');
  const [csatEnabled, setCsatEnabled] = useState(false);
  const [fileUploadsEnabled, setFileUploadsEnabled] = useState(true);
  const [forceVisitorIdentity, setForceVisitorIdentity] = useState(false);

  // Accordion state
  const [expandedSections, setExpandedSections] = useState<Set<string>>(new Set(['widget-settings']));

  const logoInputRef = useRef<HTMLInputElement>(null);
  const avatarInputRef = useRef<HTMLInputElement>(null);
  const [dragOver, setDragOver] = useState(false);

  useEffect(() => {
    if (data?.settings) {
      const s = data.settings;
      const normalizedSchedule = normalizeBusinessHoursSchedule(s.business_hours_schedule);
      const sortedHelpSpaceIds = sortHelpSpaceIds(s.widget_help_space_ids);

      lastSyncedDraftRef.current = serializeSettingsDraft(buildSettingsDraftFromServer(s));
      initializedRef.current = true;

      setRequireEmail(s.require_email_before_chat);
      setRequirePhone(s.require_phone_after_email);
      setWelcomeMessage(s.welcome_message);
      setBrandColor(s.brand_color);
      setShowBranding(s.show_branding);
      setLauncherPosition(s.launcher_position);
      setLauncherIcon(s.launcher_icon);
      setColorScheme(s.color_scheme || 'light');
      setButtonColor(s.button_color || '#000000');
      setButtonIconColor(s.button_icon_color || '#FFFFFF');
      setLogoUrl(s.logo_url || '');
      setWidgetName(s.widget_name || '');
      setWidgetAvatarUrl(s.widget_avatar_url || '');
      setWidgetHelpSpaceIds(sortedHelpSpaceIds);
      setAiEnabled(s.ai_enabled);
      setAiAgentId(s.ai_agent_id ?? NO_AGENT_VALUE);
      setConfidenceThreshold(String(s.ai_confidence_threshold));
      setAiResponseMode(s.ai_response_mode ?? 'off');
      setAiMaxFollowups(s.ai_max_followups ?? 3);
      setShowTalkToHuman(s.show_talk_to_human);
      setEscalationMessage(s.escalation_message || 'Let me connect you with a team member who can help further.');
      setHandoffBehavior(s.handoff_behavior);
      setHandoffTeamId(s.handoff_team_id);
      setBusinessHoursEnabled(s.business_hours_enabled);
      setTimezone(s.business_hours_timezone);
      setSchedule(normalizedSchedule);
      setOutsideMessage(s.outside_hours_message);
      setEmailFallbackDelaySecs(s.email_fallback_delay_secs ?? 120);
      setEmailFallbackFromName(s.email_fallback_from_name ?? '');
      setCsatEnabled(s.csat_enabled);
      setFileUploadsEnabled(s.file_uploads_enabled ?? true);
      setForceVisitorIdentity(s.force_visitor_identity ?? false);
    }
  }, [data]);

  const [saveStatus, setSaveStatus] = useState<'idle' | 'saving' | 'saved'>('idle');
  const savedTimerRef = useRef<ReturnType<typeof setTimeout>>(undefined);
  const debounceRef = useRef<ReturnType<typeof setTimeout>>(undefined);
  const initializedRef = useRef(false);
  const lastSyncedDraftRef = useRef<string>('');

  const settingsDraft: ChatSettingsDraft = {
    require_email_before_chat: requireEmail,
    require_phone_after_email: requirePhone,
    welcome_message: welcomeMessage,
    widget_name: widgetName,
    widget_avatar_url: widgetAvatarUrl,
    widget_help_space_ids: sortHelpSpaceIds(widgetHelpSpaceIds),
    brand_color: brandColor,
    show_branding: showBranding,
    launcher_position: launcherPosition,
    launcher_icon: launcherIcon,
    color_scheme: colorScheme,
    button_color: buttonColor,
    button_icon_color: buttonIconColor,
    logo_url: logoUrl,
    ai_enabled: aiEnabled,
    ai_agent_id: aiAgentId === NO_AGENT_VALUE ? '' : aiAgentId,
    ai_confidence_threshold: parseFloat(confidenceThreshold),
    ai_response_mode: aiResponseMode,
    ai_max_followups: aiMaxFollowups,
    ai_auto_resolve_timeout: data?.settings.ai_auto_resolve_timeout ?? 24,
    show_talk_to_human: showTalkToHuman,
    escalation_message: escalationMessage,
    handoff_behavior: handoffBehavior,
    handoff_team_id: handoffBehavior === 'assign_to_team' ? handoffTeamId : null,
    business_hours_enabled: businessHoursEnabled,
    business_hours_timezone: timezone,
    business_hours_schedule: normalizeBusinessHoursSchedule(schedule),
    outside_hours_message: outsideMessage,
    email_fallback_enabled: true,
    email_fallback_delay_secs: emailFallbackDelaySecs,
    email_fallback_from_name: emailFallbackFromName,
    csat_enabled: csatEnabled,
    file_uploads_enabled: fileUploadsEnabled,
    force_visitor_identity: forceVisitorIdentity,
  };
  const settingsDraftKey = serializeSettingsDraft(settingsDraft);
  const settingsDraftRef = useRef(settingsDraft);
  settingsDraftRef.current = settingsDraft;
  const mutateSettingsRef = useRef(updateMutation.mutate);
  mutateSettingsRef.current = updateMutation.mutate;

  // Auto-save with debounce when any setting changes
  useEffect(() => {
    if (!initializedRef.current || settingsDraftKey === lastSyncedDraftRef.current) {
      return;
    }

    clearTimeout(debounceRef.current);
    debounceRef.current = setTimeout(() => {
      setSaveStatus('saving');
      mutateSettingsRef.current(settingsDraftRef.current, {
        onSuccess: () => {
          lastSyncedDraftRef.current = settingsDraftKey;
          setSaveStatus('saved');
          clearTimeout(savedTimerRef.current);
          savedTimerRef.current = setTimeout(() => setSaveStatus('idle'), 2000);
        },
        onError: (err: unknown) => {
          setSaveStatus('idle');
          toast.error(err instanceof Error ? err.message : 'Failed to save');
        },
      });
    }, 800);

    return () => clearTimeout(debounceRef.current);
  }, [settingsDraftKey]);

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

  const externalDocsSpaces = docsSpaces.filter((space) => space.type === 'external_capable');

  const toggleHelpSpace = (spaceId: string, enabled: boolean) => {
    setWidgetHelpSpaceIds((current) => (
      enabled
        ? current.includes(spaceId) ? current : [...current, spaceId]
        : current.filter((id) => id !== spaceId)
    ));
  };


  const copyToClipboard = (text: string, label: string) => {
    navigator.clipboard.writeText(text);
    toast.success(`${label} copied`);
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

  if (isLoading) {
    return (
      <div className="space-y-6">
        <Skeleton className="h-48 w-full rounded-lg" />
        <Skeleton className="h-48 w-full rounded-lg" />
        <Skeleton className="h-64 w-full rounded-lg" />
      </div>
    );
  }

  const widgetKey = data?.widget_key ?? '';
  const embedSnippet = `<script type="text/javascript">
  (function () {
    window.helpin = window.helpin || function () {
      (window.helpinQ = window.helpinQ || []).push(arguments);
    };
    var t = document.createElement('script'),
        s = document.getElementsByTagName('script')[0];
    t.defer = true;
    t.id = 'helpin-widget';
    t.setAttribute('data-widget-key', '${widgetKey}');
    t.setAttribute('data-host', 'https://client.prod.helpin.ai');
    t.src = 'https://cdn.helpin.ai/lib.js';
    s.parentNode.insertBefore(t, s);
  })();
</script>`;
  const jsApiSnippet = `<script type="text/javascript">
  (function () {
    window.helpin = window.helpin || function () {
      (window.helpinQ = window.helpinQ || []).push(arguments);
    };
    var t = document.createElement('script'),
        s = document.getElementsByTagName('script')[0];
    t.defer = true;
    t.id = 'helpin-widget';
    t.setAttribute('data-widget-key', '${widgetKey}');
    t.setAttribute('data-host', 'https://client.prod.helpin.ai');
    t.src = 'https://cdn.helpin.ai/lib.js';
    s.parentNode.insertBefore(t, s);

    // Identify logged-in users (optional)
    helpin('boot', {
      key: '${widgetKey}',
      user: {
        email: 'user@example.com',
        name: 'Jane Doe',
        userId: 'your-internal-id'
      }
    });
  })();
</script>`;

  // Derive help spaces for the widget preview
  const previewHelpSpaces = docsSpaces
    .filter(space => widgetHelpSpaceIds.includes(space.id))
    .map(space => ({ id: space.id, name: space.name, slug: space.slug }));

  const previewHost = (import.meta.env.VITE_API_URL || 'http://localhost:8080/api').replace(/\/api\/?$/, '');
  const previewAvailability = buildPreviewAvailability(businessHoursEnabled, timezone, schedule, outsideMessage);

  // Shared preview element used by both views
  const previewElement = (
    <WidgetPreview
      brandColor={brandColor}
      showBranding={showBranding}
      launcherPosition={launcherPosition}
      launcherIcon={launcherIcon}
      welcomeMessage={welcomeMessage}
      workspaceName={widgetName || workspace?.name}
      workspaceLogoUrl={widgetAvatarUrl || workspace?.logo_url}
      colorScheme={colorScheme}
      buttonColor={buttonColor}
      buttonIconColor={buttonIconColor}
      logoUrl={logoUrl}
      helpSpaces={previewHelpSpaces}
      availability={previewAvailability}
      aiFirst={aiEnabled && aiResponseMode === 'ai_first'}
      showTalkToHuman={showTalkToHuman}
      escalationMessage={escalationMessage}
      widgetKey={widgetKey}
      host={previewHost}
    />
  );

  /* ── Main settings view ────────────────────────────────────────────── */

  return (
    <PreviewLayout preview={previewElement}>
      <div className="flex flex-1 flex-col overflow-auto">
        <div className="flex-1 space-y-3 p-4">
        {/* Auto-save indicator */}
        {saveStatus !== 'idle' && (
          <div className="fixed bottom-4 right-4 z-50 flex items-center gap-2 rounded-lg border bg-background/95 px-3 py-2 text-sm shadow-lg backdrop-blur animate-in fade-in slide-in-from-bottom-2 duration-200">
            {saveStatus === 'saving' && (
              <>
                <Loader2 className="h-3.5 w-3.5 animate-spin text-muted-foreground" />
                <span className="text-muted-foreground">Saving...</span>
              </>
            )}
            {saveStatus === 'saved' && (
              <>
                <Check className="h-3.5 w-3.5 text-green-500" />
                <span className="text-muted-foreground">Saved</span>
              </>
            )}
          </div>
        )}
        {/* Widget Installation */}
        <div className="overflow-hidden rounded-lg border border-border bg-background">
          <button
            type="button"
            onClick={() => toggleSection('widget-installation')}
            className="flex w-full items-center gap-4 px-4 py-4 text-left transition-colors hover:bg-muted/40"
          >
            <div className="flex h-9 w-9 shrink-0 items-center justify-center rounded-lg bg-muted text-muted-foreground">
              <Code className="h-4 w-4" />
            </div>
            <div className="min-w-0 flex-1">
              <p className="text-sm font-medium">Widget Installation</p>
              <p className="text-sm text-muted-foreground">Embed the chat widget on your website</p>
            </div>
            <ChevronDown className={cn('h-4 w-4 shrink-0 text-muted-foreground transition-transform duration-200', isExpanded('widget-installation') && 'rotate-180')} />
          </button>
          <div className="accordion-animate" data-open={isExpanded('widget-installation')}>
            <div>
            <div className="border-t border-border p-4 space-y-4">
              {!widgetKey ? (
                <div className="flex flex-col items-center gap-3 py-6 text-center">
                  <div className="flex h-12 w-12 items-center justify-center rounded-full bg-muted">
                    <KeyRound className="h-6 w-6 text-muted-foreground" />
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
                      <Button type="button" variant="outline" size="icon" className="shrink-0" onClick={() => copyToClipboard(widgetKey, 'Widget key')}>
                        <Copy className="h-4 w-4" />
                      </Button>
                    </div>
                  </div>

                  <div className="space-y-2">
                    <Label className="text-sm">Embed Snippet</Label>
                    <div className="flex items-center gap-1 p-0.5 rounded-md border bg-muted/30 w-fit mb-2">
                      <button
                        onClick={() => setSnippetTab('basic')}
                        className={cn(
                          'px-2.5 py-1 rounded text-xs transition-colors',
                          snippetTab === 'basic'
                            ? 'bg-background shadow-sm font-medium text-foreground'
                            : 'text-muted-foreground hover:text-foreground'
                        )}
                      >
                        Basic
                      </button>
                      <button
                        onClick={() => setSnippetTab('advanced')}
                        className={cn(
                          'px-2.5 py-1 rounded text-xs transition-colors',
                          snippetTab === 'advanced'
                            ? 'bg-background shadow-sm font-medium text-foreground'
                            : 'text-muted-foreground hover:text-foreground'
                        )}
                      >
                        With User Identity
                      </button>
                    </div>
                    <CodeBlock
                      code={snippetTab === 'basic' ? embedSnippet : jsApiSnippet}
                      language="markup"
                      showLineNumbers
                    />
                    <p className="text-xs text-muted-foreground">
                      {snippetTab === 'basic'
                        ? 'Add this script tag before the closing </body> tag on every page where you want the widget.'
                        : 'Use this to identify logged-in users. Replace the placeholder values with real user data from your app.'}
                    </p>
                  </div>
                </>
              )}
            </div>
            </div>
          </div>
        </div>

        {/* Identity Capture */}
        <div className="overflow-hidden rounded-lg border border-border bg-background">
          <button
            type="button"
            onClick={() => toggleSection('identity-capture')}
            className="flex w-full items-center gap-4 px-4 py-4 text-left transition-colors hover:bg-muted/40"
          >
            <div className="flex h-9 w-9 shrink-0 items-center justify-center rounded-lg bg-muted text-muted-foreground">
              <MessageSquare className="h-4 w-4" />
            </div>
            <div className="min-w-0 flex-1">
              <p className="text-sm font-medium">Identity Capture</p>
              <p className="text-sm text-muted-foreground">Control what information is collected before starting a chat</p>
            </div>
            <ChevronDown className={cn('h-4 w-4 shrink-0 text-muted-foreground transition-transform duration-200', isExpanded('identity-capture') && 'rotate-180')} />
          </button>
          <div className="accordion-animate" data-open={isExpanded('identity-capture')}>
            <div>
            <div className="border-t border-border p-4 space-y-4">
              <div className="flex items-center justify-between">
                <div>
                  <Label className="text-sm">Require email before chat</Label>
                  <p className="text-xs text-muted-foreground">Visitors must enter their email to start a conversation.</p>
                </div>
                <Switch checked={requireEmail} onCheckedChange={setRequireEmail} />
              </div>

              <div className="flex items-center justify-between">
                <div>
                  <Label className="text-sm">Require phone number after email</Label>
                  <p className="text-xs text-muted-foreground">Also ask for the visitor's phone number.</p>
                </div>
                <Switch checked={requirePhone} onCheckedChange={setRequirePhone} disabled={!requireEmail} />
              </div>

              <div className="flex items-center justify-between">
                <div>
                  <Label className="text-sm">Force visitors to identify themselves</Label>
                  <p className="text-xs text-muted-foreground">Visitors must provide their email (or phone) before chatting. When disabled, they can skip the identity step.</p>
                </div>
                <Switch checked={forceVisitorIdentity} onCheckedChange={setForceVisitorIdentity} disabled={!requireEmail} />
              </div>

              <div className="space-y-2">
                <Label htmlFor="welcome-msg" className="text-sm">Welcome Message</Label>
                <Input
                  id="welcome-msg"
                  value={welcomeMessage}
                  onChange={(e) => setWelcomeMessage(e.target.value)}
                  placeholder="Hi there! How can we help you today?"
                />
              </div>
            </div>
            </div>
          </div>
        </div>

        {/* Appearance */}
        <div className="overflow-hidden rounded-lg border border-border bg-background">
          <button
            type="button"
            onClick={() => toggleSection('appearance')}
            className="flex w-full items-center gap-4 px-4 py-4 text-left transition-colors hover:bg-muted/40"
          >
            <div className="flex h-9 w-9 shrink-0 items-center justify-center rounded-lg bg-muted text-muted-foreground">
              <ImageIcon className="h-4 w-4" />
            </div>
            <div className="min-w-0 flex-1">
              <p className="text-sm font-medium">Appearance</p>
              <p className="text-sm text-muted-foreground">Customize the widget's visual appearance</p>
            </div>
            <ChevronDown className={cn('h-4 w-4 shrink-0 text-muted-foreground transition-transform duration-200', isExpanded('appearance') && 'rotate-180')} />
          </button>
          <div className="accordion-animate" data-open={isExpanded('appearance')}>
            <div>
            <div className="border-t border-border p-4 space-y-6">
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
                    <ImageIcon className="h-6 w-6 text-muted-foreground/50" />
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
                  <p className="text-xs text-muted-foreground mt-0.5">Display branding in the widget footer.</p>
                </div>
                <Switch checked={showBranding} onCheckedChange={setShowBranding} />
              </div>
            </div>
            </div>
          </div>
        </div>

        {/* Help Center */}
        <div className="overflow-hidden rounded-lg border border-border bg-background">
          <button
            type="button"
            onClick={() => toggleSection('help-center')}
            className="flex w-full items-center gap-4 px-4 py-4 text-left transition-colors hover:bg-muted/40"
          >
            <div className="flex h-9 w-9 shrink-0 items-center justify-center rounded-lg bg-muted text-muted-foreground">
              <HelpCircle className="h-4 w-4" />
            </div>
            <div className="min-w-0 flex-1">
              <p className="text-sm font-medium">Help Center</p>
              <p className="text-sm text-muted-foreground">Select docs spaces for the widget Help tab</p>
            </div>
            <ChevronDown className={cn('h-4 w-4 shrink-0 text-muted-foreground transition-transform duration-200', isExpanded('help-center') && 'rotate-180')} />
          </button>
          <div className="accordion-animate" data-open={isExpanded('help-center')}>
            <div>
            <div className="border-t border-border p-4 space-y-4">
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
          {/* AI Auto-Reply */}
          <div className="overflow-hidden rounded-lg border border-border bg-background">
            <button
              type="button"
              onClick={() => toggleSection('ai-auto-reply')}
              className="flex w-full items-center gap-4 px-4 py-4 text-left transition-colors hover:bg-muted/40"
            >
              <div className="flex h-9 w-9 shrink-0 items-center justify-center rounded-lg bg-muted text-muted-foreground">
                <Bot className="h-4 w-4" />
              </div>
              <div className="min-w-0 flex-1">
                <p className="text-sm font-medium">AI Auto-Reply</p>
                <p className="text-sm text-muted-foreground">Configure AI-powered automatic responses</p>
              </div>
              <ChevronDown className={cn('h-4 w-4 shrink-0 text-muted-foreground transition-transform duration-200', isExpanded('ai-auto-reply') && 'rotate-180')} />
            </button>
            <div className="accordion-animate" data-open={isExpanded('ai-auto-reply')}>
              <div>
              <div className="border-t border-border p-4 space-y-4">
                <div className="flex items-center justify-between">
                  <div>
                    <Label className="text-sm">Enable AI auto-reply</Label>
                    <p className="text-xs text-muted-foreground">AI will attempt to answer questions using your knowledge base.</p>
                  </div>
                  <Switch checked={aiEnabled} onCheckedChange={setAiEnabled} />
                </div>

                <div className="space-y-2">
                  <Label className="text-sm">Support Agent</Label>
                  <Select value={aiAgentId} onValueChange={setAiAgentId}>
                    <SelectTrigger className="w-full">
                      <SelectValue placeholder="Select a support agent..." />
                    </SelectTrigger>
                    <SelectContent>
                      <SelectItem value={NO_AGENT_VALUE}>No support agent selected</SelectItem>
                      {supportAgents.map((agent) => (
                        <SelectItem key={agent.id} value={agent.id}>{agent.name}</SelectItem>
                      ))}
                    </SelectContent>
                  </Select>
                  <p className="text-xs text-muted-foreground">
                    Chat auto-replies will run this support agent on new visitor messages.
                  </p>
                </div>
                {/* AI settings — 2-col grid */}
                <div className="grid grid-cols-2 gap-4">
                  <div className="space-y-1.5">
                    <Label className="text-sm">Response Mode</Label>
                    <Select value={aiResponseMode} onValueChange={setAiResponseMode}>
                      <SelectTrigger>
                        <SelectValue />
                      </SelectTrigger>
                      <SelectContent>
                        <SelectItem value="off">Off (manual only)</SelectItem>
                        <SelectItem value="ai_first">AI First (auto-reply)</SelectItem>
                      </SelectContent>
                    </Select>
                  </div>
                  <div className="space-y-1.5">
                    <Label className="text-sm">Confidence Threshold</Label>
                    <Select value={confidenceThreshold} onValueChange={setConfidenceThreshold}>
                      <SelectTrigger>
                        <SelectValue />
                      </SelectTrigger>
                      <SelectContent>
                        {[0.1, 0.2, 0.3, 0.4, 0.5, 0.6, 0.7, 0.8, 0.9, 1.0].map(v => (
                          <SelectItem key={v} value={String(v)}>{(v * 100).toFixed(0)}%</SelectItem>
                        ))}
                      </SelectContent>
                    </Select>
                  </div>
                  <div className="space-y-1.5">
                    <Label className="text-sm">Max Follow-ups</Label>
                    <Select value={String(aiMaxFollowups)} onValueChange={(v) => setAiMaxFollowups(Number(v))}>
                      <SelectTrigger>
                        <SelectValue />
                      </SelectTrigger>
                      <SelectContent>
                        {[1, 2, 3, 4, 5, 6, 7, 8, 9, 10].map(v => (
                          <SelectItem key={v} value={String(v)}>{v}</SelectItem>
                        ))}
                      </SelectContent>
                    </Select>
                  </div>
                </div>

                <div className="flex items-center justify-between">
                  <div>
                    <Label className="text-sm">Show "Talk to Human" button</Label>
                    <p className="text-xs text-muted-foreground">Let visitors request help from a team member at any time.</p>
                  </div>
                  <Switch checked={showTalkToHuman} onCheckedChange={setShowTalkToHuman} />
                </div>

                <div className="space-y-2">
                  <Label htmlFor="escalation-msg" className="text-sm">Escalation Message</Label>
                  <Textarea
                    id="escalation-msg"
                    value={escalationMessage}
                    onChange={(e) => setEscalationMessage(e.target.value)}
                    placeholder="Let me connect you with a team member who can help further."
                    rows={2}
                  />
                  <p className="text-xs text-muted-foreground">Message shown to the visitor when AI hands off to a human agent.</p>
                </div>

                {/* Handoff Routing — merged sub-section */}
                <div className="border-t border-border pt-4 space-y-3">
                  <div>
                    <Label className="text-sm font-medium">Handoff Routing</Label>
                    <p className="text-xs text-muted-foreground mt-0.5">How conversations are assigned when human help is needed.</p>
                  </div>
                  <div className="grid grid-cols-2 gap-4">
                    <div className="space-y-1.5">
                      <Label className="text-sm">Behavior</Label>
                      <Select value={handoffBehavior} onValueChange={setHandoffBehavior}>
                        <SelectTrigger>
                          <SelectValue />
                        </SelectTrigger>
                        <SelectContent>
                          <SelectItem value="unassigned">Leave unassigned</SelectItem>
                          <SelectItem value="assign_to_team">Assign to team</SelectItem>
                          <SelectItem value="round_robin">Round robin</SelectItem>
                        </SelectContent>
                      </Select>
                    </div>
                    {handoffBehavior === 'assign_to_team' && (
                      <div className="space-y-1.5">
                        <Label className="text-sm">Team</Label>
                        <Select value={handoffTeamId ?? ''} onValueChange={setHandoffTeamId}>
                          <SelectTrigger>
                            <SelectValue placeholder="Select a team..." />
                          </SelectTrigger>
                          <SelectContent>
                            {teams.map(team => (
                              <SelectItem key={team.id} value={team.id}>{team.name}</SelectItem>
                            ))}
                          </SelectContent>
                        </Select>
                      </div>
                    )}
                  </div>
                </div>
              </div>
              </div>
            </div>
          </div>

          {/* Availability */}
          <div className="overflow-hidden rounded-lg border border-border bg-background">
            <button
              type="button"
              onClick={() => toggleSection('business-hours')}
              className="flex w-full items-center gap-4 px-4 py-4 text-left transition-colors hover:bg-muted/40"
            >
              <div className="flex h-9 w-9 shrink-0 items-center justify-center rounded-lg bg-muted text-muted-foreground">
                <MessageSquare className="h-4 w-4" />
              </div>
              <div className="min-w-0 flex-1">
                <p className="text-sm font-medium">Availability</p>
                <p className="text-sm text-muted-foreground">Set when your team appears online and what visitors should expect</p>
              </div>
              <ChevronDown className={cn('h-4 w-4 shrink-0 text-muted-foreground transition-transform duration-200', isExpanded('business-hours') && 'rotate-180')} />
            </button>
            <div className="accordion-animate" data-open={isExpanded('business-hours')}>
              <div>
              <div className="border-t border-border p-4 space-y-4">
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
              </div>
              </div>
            </div>
          </div>

          {/* Chat Features — merged CSAT, File Uploads, Email */}
          <div className="overflow-hidden rounded-lg border border-border bg-background">
            <button
              type="button"
              onClick={() => toggleSection('chat-features')}
              className="flex w-full items-center gap-4 px-4 py-4 text-left transition-colors hover:bg-muted/40"
            >
              <div className="flex h-9 w-9 shrink-0 items-center justify-center rounded-lg bg-muted text-muted-foreground">
                <Star className="h-4 w-4" />
              </div>
              <div className="min-w-0 flex-1">
                <p className="text-sm font-medium">Chat Features</p>
                <p className="text-sm text-muted-foreground">File uploads, satisfaction surveys, and fallback emails</p>
              </div>
              <ChevronDown className={cn('h-4 w-4 shrink-0 text-muted-foreground transition-transform duration-200', isExpanded('chat-features') && 'rotate-180')} />
            </button>
            <div className="accordion-animate" data-open={isExpanded('chat-features')}>
              <div>
              <div className="border-t border-border p-4 space-y-4">
                <div className="flex items-center justify-between">
                  <div>
                    <Label className="text-sm">File uploads</Label>
                    <p className="text-xs text-muted-foreground">Allow visitors to upload images, documents, and other files (max 10 MB).</p>
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
