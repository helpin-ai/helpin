import { useEffect, useRef, useState } from 'react';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Switch } from '@/components/ui/switch';
import { Skeleton } from '@/components/ui/skeleton';
import { Textarea } from '@/components/ui/textarea';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { toast } from 'sonner';
import { Tick01Icon, Copy01Icon, CodeIcon, Loading01Icon, Message01Icon, HelpCircleIcon, Image01Icon, Key01Icon, BotIcon, ArrowDown01Icon, StarIcon } from '@/lib/icons';
import { useChatSettings, useUpdateChatSettings, useRegenerateWidgetKey, useDocsSpaces } from '@/hooks/queries';
import { useSupportAgents, useSupportMailboxes } from '@/hooks/queries/useSupport';
import { useWorkspaceTeams } from '@/hooks/useWorkspaceTeams';
import { WidgetPreview } from './WidgetPreview';
import { CodeBlock } from '@/components/ui/code-block';
import { BrandColorPicker } from '@/components/pm/ColorPicker';
import { useWorkspaceStore } from '@/stores/workspaceStore';
import { cn } from '@/lib/utils';
import type { BusinessHoursDay } from '@/lib/pmTypes';
import { COLOR_SCHEME_OPTIONS, COMMON_TIMEZONES, DAYS, ICON_OPTIONS, NO_AGENT_VALUE } from './chat-widget/constants';
import { PreviewLayout } from './chat-widget/PreviewLayout';
import {
  buildPreviewAvailability,
  buildSettingsDraftFromServer,
  normalizeBusinessHoursDay,
  normalizeBusinessHoursSchedule,
  serializeSettingsDraft,
  sortHelpSpaceIds,
  type ChatSettingsDraft,
} from './chat-widget/utils';

/* ── Main component ──────────────────────────────────────────────────── */

export function ChatGeneralTab({ workspaceId }: { workspaceId: string }) {
  const workspace = useWorkspaceStore((s) => s.currentWorkspace);
  const { data, isLoading } = useChatSettings(workspaceId);
  const { data: docsSpaces = [], isLoading: docsSpacesLoading } = useDocsSpaces(workspaceId);
  const updateMutation = useUpdateChatSettings(workspaceId);
  const regenerateKeyMutation = useRegenerateWidgetKey(workspaceId);
  const { teams } = useWorkspaceTeams(workspaceId);
  const { data: supportAgents = [] } = useSupportAgents(workspaceId);
  const { data: supportMailboxes = [] } = useSupportMailboxes(workspaceId);

  const [snippetTab, setSnippetTab] = useState<'html' | 'html-identify' | 'react' | 'nextjs'>('html');

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
  const [defaultMailboxId, setDefaultMailboxId] = useState<string | null>(null);
  const [aiHandoffMailboxId, setAiHandoffMailboxId] = useState<string | null>(null);
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
      setDefaultMailboxId(s.default_mailbox_id);
      setAiHandoffMailboxId(s.ai_handoff_mailbox_id);
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
    default_mailbox_id: defaultMailboxId,
    ai_handoff_mailbox_id: aiHandoffMailboxId,
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
    t.setAttribute('data-host', 'https://client.helpin.ai');
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
    t.setAttribute('data-host', 'https://client.helpin.ai');
    t.src = 'https://cdn.helpin.ai/lib.js';
    s.parentNode.insertBefore(t, s);

    // Identify logged-in users (optional)
    helpin('boot', {
      widgetKey: '${widgetKey}',
      host: 'https://client.helpin.ai',
      user: {
        email: 'user@example.com',
        name: 'Jane Doe',
        userId: 'your-internal-id'
      }
    });
  })();
</script>`;

  const reactSnippet = `// 1. Install the package
npm install @helpin-ai/react

// 2. Wrap your app with HelpinProvider
import { createClient, HelpinProvider } from '@helpin-ai/react';

const client = createClient({
  widgetKey: '${widgetKey}',
  host: 'https://client.helpin.ai',
});

function App() {
  return (
    <HelpinProvider client={client}>
      {/* Your app */}
    </HelpinProvider>
  );
}

// 3. Identify users (optional)
import { useHelpin } from '@helpin-ai/react';

function Dashboard() {
  const { id, lead } = useHelpin();

  useEffect(() => {
    // Identify a logged-in user
    id({ email: 'user@example.com', name: 'Jane Doe', id: 'user-123' });

    // Or capture a lead
    lead({ email: 'visitor@example.com', name: 'New Lead' });
  }, []);
}`;

  const nextjsSnippet = `// 1. Install the package
npm install @helpin-ai/nextjs

// 2. Add to your root layout (app/layout.tsx)
import { HelpinProvider, createClient } from '@helpin-ai/nextjs';

const client = createClient({
  widgetKey: '${widgetKey}',
  host: 'https://client.helpin.ai',
});

export default function RootLayout({ children }) {
  return (
    <html>
      <body>
        <HelpinProvider client={client}>
          {children}
        </HelpinProvider>
      </body>
    </html>
  );
}

// 3. Identify users (optional)
'use client';
import { useHelpin } from '@helpin-ai/nextjs';

function Dashboard() {
  const { id, lead } = useHelpin();

  useEffect(() => {
    // Identify a logged-in user
    id({ email: 'user@example.com', name: 'Jane Doe', id: 'user-123' });

    // Or capture a lead
    lead({ email: 'visitor@example.com', name: 'New Lead' });
  }, []);
}`;

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
                <Loading01Icon className="h-3.5 w-3.5 animate-spin text-muted-foreground" />
                <span className="text-muted-foreground">Saving...</span>
              </>
            )}
            {saveStatus === 'saved' && (
              <>
                <Tick01Icon className="h-3.5 w-3.5 text-green-500" />
                <span className="text-muted-foreground">Saved</span>
              </>
            )}
          </div>
        )}
        {/* Widget Installation */}
        <div className={cn("overflow-hidden rounded-lg border bg-background transition-colors", isExpanded('widget-installation') ? "border-primary/20" : "border-border/60")}>
          <button
            type="button"
            onClick={() => toggleSection('widget-installation')}
            className="flex w-full items-center gap-4 px-4 py-4 text-left transition-colors hover:bg-muted/40"
          >
            <div className="flex h-9 w-9 shrink-0 items-center justify-center rounded-lg bg-muted text-muted-foreground">
              <CodeIcon className="h-4 w-4" />
            </div>
            <div className="min-w-0 flex-1">
              <p className="text-sm font-medium">Widget Installation</p>
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
                      <Button type="button" variant="outline" size="icon" className="shrink-0" onClick={() => copyToClipboard(widgetKey, 'Widget key')}>
                        <Copy01Icon className="h-4 w-4" />
                      </Button>
                    </div>
                  </div>

                  <div className="space-y-3">
                    <Label className="text-sm">Installation Method</Label>
                    <div className="grid grid-cols-4 gap-2">
                      {([
                        { id: 'html' as const, label: 'HTML', icon: (
                          <svg className="h-4 w-4" viewBox="0 0 24 24" fill="currentColor"><path d="M1.5 0h21l-1.91 21.563L11.977 24l-8.564-2.438L1.5 0zm7.031 9.75l-.232-2.718 10.059.003.071-.757.206-2.34.033-.37H6.693l.012.147.609 6.035h6.939l-.33 3.528-2.45.672h-.013l-2.457-.66-.158-1.752H6.27l.313 3.528 4.947 1.365h.02l4.92-1.364.667-7.318H8.531z"/></svg>
                        )},
                        { id: 'html-identify' as const, label: 'JavaScript', icon: (
                          <svg className="h-4 w-4" viewBox="0 0 24 24" fill="currentColor"><path d="M0 0h24v24H0V0zm22.034 18.276c-.175-1.095-.888-2.015-3.003-2.873-.736-.345-1.554-.585-1.797-1.14-.091-.33-.105-.51-.046-.705.15-.646.915-.84 1.515-.66.39.12.75.42.976.9 1.034-.676 1.034-.676 1.755-1.125-.27-.42-.405-.6-.586-.78-.63-.705-1.469-1.065-2.834-1.034l-.705.089c-.676.165-1.32.525-1.71 1.005-1.14 1.291-.811 3.541.569 4.471 1.365 1.02 3.361 1.244 3.616 2.205.24 1.17-.87 1.545-1.966 1.41-.811-.18-1.26-.586-1.755-1.336l-1.83 1.051c.21.48.45.689.81 1.109 1.74 1.756 6.09 1.666 6.871-1.004.029-.09.24-.705.074-1.65l.046.067zm-8.983-7.245h-2.248c0 1.938-.009 3.864-.009 5.805 0 1.232.063 2.363-.138 2.711-.33.689-1.18.601-1.566.48-.396-.196-.597-.466-.83-.855-.063-.105-.11-.196-.127-.196l-1.825 1.125c.305.63.75 1.172 1.324 1.517.855.51 2.004.675 3.207.405.783-.226 1.458-.691 1.811-1.411.51-.93.402-2.07.397-3.346.012-2.054 0-4.109 0-6.179l.004-.056z"/></svg>
                        )},
                        { id: 'react' as const, label: 'React', icon: (
                          <svg className="h-4 w-4" viewBox="0 0 24 24" fill="currentColor"><path d="M14.23 12.004a2.236 2.236 0 0 1-2.235 2.236 2.236 2.236 0 0 1-2.236-2.236 2.236 2.236 0 0 1 2.235-2.236 2.236 2.236 0 0 1 2.236 2.236zm2.648-10.69c-1.346 0-3.107.96-4.888 2.622-1.78-1.653-3.542-2.602-4.887-2.602-.31 0-.592.068-.837.188-.924.472-1.34 1.768-.967 3.625.076.378.179.763.304 1.152-1.378.414-2.503.964-3.254 1.616C1.56 8.63 1.29 9.41 1.29 10.186c0 1.452 1.236 2.88 3.298 3.856-.132.43-.228.857-.294 1.27-.362 1.84.024 3.115.937 3.583.235.122.513.181.817.181 1.346 0 3.107-.96 4.888-2.624 1.78 1.655 3.542 2.604 4.887 2.604.31 0 .592-.068.837-.188.924-.472 1.34-1.768.967-3.625-.076-.378-.18-.763-.304-1.152 1.378-.414 2.503-.964 3.254-1.616.79-.676 1.06-1.456 1.06-2.232 0-1.452-1.236-2.88-3.298-3.856.132-.43.228-.857.294-1.27.362-1.84-.024-3.115-.937-3.583a1.77 1.77 0 0 0-.817-.181zM17.5 3.473c.156 0 .29.03.395.085.343.175.59.753.46 1.41-.057.296-.144.59-.256.886a16.147 16.147 0 0 0-2.1-.44 16.374 16.374 0 0 0-1.388-1.642c1.45-1.373 2.843-2.3 3.889-2.3zM12 8.01a15.16 15.16 0 0 1 1.39 1.627 18.098 18.098 0 0 1-2.78 0A15.681 15.681 0 0 1 12 8.01zm-4.593 2.09c.208-.34.425-.667.65-.98.338.065.685.119 1.04.164a17.91 17.91 0 0 0-.85 1.418c-.302-.14-.59-.292-.84-.46v-.142zm-.857 1.786c.73.333 1.528.607 2.374.823a17.48 17.48 0 0 0 1.074 2.2 15.684 15.684 0 0 1-2.337 1.37c-.67-.604-1.14-1.405-1.112-2.3v-2.093zm4.45 6.107c-1.45 1.373-2.843 2.3-3.889 2.3-.156 0-.29-.03-.395-.085-.343-.175-.59-.753-.46-1.41.057-.296.144-.59.256-.886.53.162 1.1.3 1.695.41a16.374 16.374 0 0 0 1.388 1.642c-.194.012-.39.03-.595.03zm1-2.003a15.16 15.16 0 0 1-1.39-1.627c.913.065 1.852.065 2.78 0A15.681 15.681 0 0 1 12 15.99zm4.593-2.09c-.208.34-.425.667-.65.98-.338-.065-.685-.119-1.04-.164.296-.453.582-.926.85-1.418.302.14.59.292.84.46v.142zm1.265-1.926c-.73-.333-1.528-.607-2.374-.823a17.48 17.48 0 0 0-1.074-2.2 15.684 15.684 0 0 1 2.337-1.37c.67.604 1.14 1.405 1.112 2.3v2.093zm-2.966-3.597a16.147 16.147 0 0 0-1.695-.41 16.374 16.374 0 0 0-1.388-1.642c1.45-1.373 2.843-2.3 3.889-2.3.156 0 .29.03.395.085.343.175.59.753.46 1.41-.057.296-.144.59-.256.886z"/></svg>
                        )},
                        { id: 'nextjs' as const, label: 'Next.js', icon: (
                          <svg className="h-4 w-4" viewBox="0 0 24 24" fill="currentColor"><path d="M11.572 0c-.176 0-.31.001-.358.007a19.76 19.76 0 0 1-.364.033C7.443.346 4.25 2.185 2.228 5.012a11.875 11.875 0 0 0-2.119 5.243c-.096.659-.108.854-.108 1.747s.012 1.089.108 1.748c.652 4.506 3.86 8.292 8.209 9.695.779.25 1.6.422 2.534.525.363.04 1.935.04 2.299 0 1.611-.178 2.977-.577 4.323-1.264.207-.106.247-.134.219-.158-.02-.013-.9-1.193-1.955-2.62l-1.919-2.592-2.404-3.558a338.739 338.739 0 0 0-2.422-3.556c-.009-.002-.018 1.579-.023 3.51-.007 3.38-.01 3.515-.052 3.595a.426.426 0 0 1-.206.214c-.075.037-.14.044-.495.044H7.81l-.108-.068a.438.438 0 0 1-.157-.171l-.05-.106.006-4.703.007-4.705.072-.092a.645.645 0 0 1 .174-.143c.096-.047.134-.051.54-.051.478 0 .558.018.682.154.035.038 1.337 1.999 2.895 4.361a10760.433 10760.433 0 0 0 4.735 7.17l1.9 2.879.096-.063a12.317 12.317 0 0 0 2.466-2.163 11.944 11.944 0 0 0 2.824-6.134c.096-.66.108-.854.108-1.748 0-.893-.012-1.088-.108-1.747-.652-4.506-3.86-8.292-8.209-9.695a12.597 12.597 0 0 0-2.499-.523A33.119 33.119 0 0 0 11.572 0zm4.069 7.217c.347 0 .408.005.486.047a.473.473 0 0 1 .237.277c.018.06.023 1.365.018 4.304l-.006 4.218-.744-1.14-.746-1.14v-3.066c0-1.982.01-3.097.023-3.15a.478.478 0 0 1 .233-.296c.096-.05.13-.054.5-.054z"/></svg>
                        )},
                      ] as const).map(tab => (
                        <button
                          key={tab.id}
                          onClick={() => setSnippetTab(tab.id)}
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
                    <CodeBlock
                      code={
                        snippetTab === 'html' ? embedSnippet
                        : snippetTab === 'html-identify' ? jsApiSnippet
                        : snippetTab === 'react' ? reactSnippet
                        : nextjsSnippet
                      }
                      language={snippetTab === 'react' || snippetTab === 'nextjs' ? 'typescript' : 'markup'}
                      showLineNumbers
                    />
                    <p className="text-xs text-muted-foreground">
                      {snippetTab === 'html'
                        ? 'Add this script tag before the closing </body> tag on every page where you want the widget.'
                        : snippetTab === 'html-identify'
                        ? 'Use this to identify logged-in users. Replace the placeholder values with real user data from your app.'
                        : snippetTab === 'react'
                        ? 'Install @helpin-ai/react from npm. Use useHelpin() hook to identify users and capture leads.'
                        : 'Install @helpin-ai/nextjs from npm. Works with both App Router and Pages Router.'}
                    </p>
                  </div>
                </>
              )}
            </div>
            </div>
          </div>
        </div>

        {/* Identity Capture */}
        <div className={cn("overflow-hidden rounded-lg border bg-background transition-colors", isExpanded('identity-capture') ? "border-primary/20" : "border-border/60")}>
          <button
            type="button"
            onClick={() => toggleSection('identity-capture')}
            className="flex w-full items-center gap-4 px-4 py-4 text-left transition-colors hover:bg-muted/40"
          >
            <div className="flex h-9 w-9 shrink-0 items-center justify-center rounded-lg bg-muted text-muted-foreground">
              <Message01Icon className="h-4 w-4" />
            </div>
            <div className="min-w-0 flex-1">
              <p className="text-sm font-medium">Identity Capture</p>
              <p className="text-sm text-muted-foreground">Control what information is collected before starting a chat</p>
            </div>
            <ArrowDown01Icon className={cn('h-4 w-4 shrink-0 text-muted-foreground transition-transform duration-200', isExpanded('identity-capture') && 'rotate-180')} />
          </button>
          <div className="accordion-animate" data-open={isExpanded('identity-capture')}>
            <div>
            <div className="border-t border-border px-6 py-6 space-y-4">
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
        <div className={cn("overflow-hidden rounded-lg border bg-background transition-colors", isExpanded('appearance') ? "border-primary/20" : "border-border/60")}>
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
                  <p className="text-xs text-muted-foreground mt-0.5">Display branding in the widget footer.</p>
                </div>
                <Switch checked={showBranding} onCheckedChange={setShowBranding} />
              </div>
            </div>
            </div>
          </div>
        </div>

        {/* Help Center */}
        <div className={cn("overflow-hidden rounded-lg border bg-background transition-colors", isExpanded('help-center') ? "border-primary/20" : "border-border/60")}>
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
          {/* AI Auto-Reply */}
          <div className={cn("overflow-hidden rounded-lg border bg-background transition-colors", isExpanded('ai-auto-reply') ? "border-primary/20" : "border-border/60")}>
            <button
              type="button"
              onClick={() => toggleSection('ai-auto-reply')}
              className="flex w-full items-center gap-4 px-4 py-4 text-left transition-colors hover:bg-muted/40"
            >
              <div className="flex h-9 w-9 shrink-0 items-center justify-center rounded-lg bg-muted text-muted-foreground">
                <BotIcon className="h-4 w-4" />
              </div>
              <div className="min-w-0 flex-1">
                <p className="text-sm font-medium">AI Auto-Reply</p>
                <p className="text-sm text-muted-foreground">Configure AI-powered automatic responses</p>
              </div>
              <ArrowDown01Icon className={cn('h-4 w-4 shrink-0 text-muted-foreground transition-transform duration-200', isExpanded('ai-auto-reply') && 'rotate-180')} />
            </button>
            <div className="accordion-animate" data-open={isExpanded('ai-auto-reply')}>
              <div>
              <div className="border-t border-border px-6 py-6 space-y-4">
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
                      <Label className="text-sm">Default Inbox</Label>
                      <Select value={defaultMailboxId ?? 'shared'} onValueChange={(value) => setDefaultMailboxId(value === 'shared' ? null : value)}>
                        <SelectTrigger>
                          <SelectValue />
                        </SelectTrigger>
                        <SelectContent>
                          <SelectItem value="shared">Shared Inbox</SelectItem>
                          {supportMailboxes.filter((mailbox) => mailbox.active).map((mailbox) => (
                            <SelectItem key={mailbox.id} value={mailbox.id}>{mailbox.name}</SelectItem>
                          ))}
                        </SelectContent>
                      </Select>
                    </div>
                    <div className="space-y-1.5">
                      <Label className="text-sm">AI Handoff Inbox</Label>
                      <Select value={aiHandoffMailboxId ?? 'shared'} onValueChange={(value) => setAiHandoffMailboxId(value === 'shared' ? null : value)}>
                        <SelectTrigger>
                          <SelectValue />
                        </SelectTrigger>
                        <SelectContent>
                          <SelectItem value="shared">Shared Inbox</SelectItem>
                          {supportMailboxes.filter((mailbox) => mailbox.active).map((mailbox) => (
                            <SelectItem key={mailbox.id} value={mailbox.id}>{mailbox.name}</SelectItem>
                          ))}
                        </SelectContent>
                      </Select>
                    </div>
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
          <div className={cn("overflow-hidden rounded-lg border bg-background transition-colors", isExpanded('business-hours') ? "border-primary/20" : "border-border/60")}>
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
              </div>
              </div>
            </div>
          </div>

          {/* Chat Features — merged CSAT, File Uploads, Email */}
          <div className={cn("overflow-hidden rounded-lg border bg-background transition-colors", isExpanded('chat-features') ? "border-primary/20" : "border-border/60")}>
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
