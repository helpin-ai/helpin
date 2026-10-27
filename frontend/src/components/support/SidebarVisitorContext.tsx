import { useState } from 'react';
import { useWorkspaceAccess } from '@/hooks/queries/useSession';
import { ConversationMergeDialog } from './ConversationMergeDialog';
import { ConversationHistoryDialog } from './ConversationMergeHistory';
import { Button } from '@/components/ui/button';
import type { VisitorOtherConversation } from '@/lib/pmTypes';
import { Link } from '@tanstack/react-router';
import {
  CheckmarkCircle02Icon,
  CircleIcon,
  Clock03Icon,
  GitMergeIcon,
  GlobeIcon,
  OctagonXIcon,
  ChromeIcon,
  LaptopIcon,
  SmartPhone01Icon,
  Tablet01Icon,
  MapPinIcon,
  Clock01Icon,
  Link01Icon,
  LanguageCircleIcon,
} from '@/lib/icons';
import { quietRelatedItemTitleClassName } from '@/components/design-system/quiet';
import { CollapsibleSection } from '@/components/ui/collapsible-section';
import { QuickTooltip } from '@/components/ui/quick-tooltip';
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/tooltip';
import { STATUS_LABELS } from './constants';
import { useConversation, useVisitorContext } from '@/hooks/queries/useSupport';
import { useWorkspaceStore } from '@/stores/workspaceStore';
import { isSystemCRMCustomProperty } from '@/lib/crmCustomProperties';
import type { ConversationStatus } from '@/lib/pmTypes';
import { cn, isPlainLeftClick } from '@/lib/utils';

interface SidebarVisitorContextProps {
  workspaceId: string;
  conversationId: string;
}

function InfoRow({
  label,
  value,
  icon: Icon,
}: {
  label: string;
  value: string | null | undefined;
  icon?: React.ElementType;
}) {
  if (!value) return null;
  return (
    <div className="grid grid-cols-[88px_1fr] items-center gap-2 text-[12px]">
      <span className="text-muted-foreground flex items-center gap-1.5">
        {Icon && <Icon className="h-3 w-3" />}
        {label}
      </span>
      <span className="truncate font-medium text-foreground/90" title={value}>{value}</span>
    </div>
  );
}

function DeviceIcon({ type }: { type: string }) {
  switch (type) {
    case 'mobile': return <SmartPhone01Icon className="h-3 w-3" />;
    case 'tablet': return <Tablet01Icon className="h-3 w-3" />;
    default: return <LaptopIcon className="h-3 w-3" />;
  }
}

function BrowserIcon({ browser }: { browser: string }) {
  // Chrome icon exists in lucide; for others fall back to Globe
  switch (browser.toLowerCase()) {
    case 'chrome': return <ChromeIcon className="h-3 w-3" />;
    default: return <GlobeIcon className="h-3 w-3" />;
  }
}

function formatLocalTime(timezone?: string | null): string | null {
  if (!timezone) return null;
  const now = new Date();
  try {
    const time = new Intl.DateTimeFormat(undefined, {
      timeZone: timezone,
      hour: 'numeric',
      minute: '2-digit',
      hour12: true,
    }).format(now);
    const offsetPart = new Intl.DateTimeFormat(undefined, {
      timeZone: timezone,
      timeZoneName: 'shortOffset',
    })
      .formatToParts(now)
      .find((p) => p.type === 'timeZoneName')?.value;
    return offsetPart ? `${time} (${offsetPart})` : time;
  } catch {
    return null;
  }
}

function formatLocale(locale?: string | null): string | null {
  const normalized = locale?.trim();
  if (!normalized) return null;

  try {
    const parsed = new Intl.Locale(normalized);
    const languageName = parsed.language
      ? new Intl.DisplayNames(undefined, { type: 'language' }).of(parsed.language)
      : null;
    const regionName = parsed.region
      ? new Intl.DisplayNames(undefined, { type: 'region' }).of(parsed.region)
      : null;
    if (languageName && regionName) return `${languageName} (${regionName})`;
    if (languageName) return languageName;
  } catch {
    // Fall back to the raw locale code for malformed or unsupported values.
  }

  return normalized;
}

function DetailIcon({
  label,
  children,
}: {
  label: string;
  children: React.ReactNode;
}) {
  return (
    <Tooltip>
      <TooltipTrigger asChild>
        <span
          aria-label={label}
          className="flex h-4 w-4 shrink-0 items-center justify-center text-muted-foreground/70"
        >
          {children}
        </span>
      </TooltipTrigger>
      <TooltipContent side="left">
        <span className="text-xs">{label}</span>
      </TooltipContent>
    </Tooltip>
  );
}

function MainInfoRow({
  label,
  icon,
  value,
  href,
}: {
  label: string;
  icon: React.ReactNode;
  value: string;
  href?: string;
}) {
  return (
    <div className="flex items-center gap-2.5 text-[12px]">
      <DetailIcon label={label}>
        {icon}
      </DetailIcon>
      {href ? (
        <a
          href={href}
          target="_blank"
          rel="noopener noreferrer"
          className="truncate text-blue-600 hover:underline dark:text-blue-400"
          title={value}
        >
          {value}
        </a>
      ) : (
        <span className="truncate text-foreground/90" title={value}>{value}</span>
      )}
    </div>
  );
}

/** Contact details: the person's own CRM fields. Lifecycle lives in the identity card. */
export function SidebarVisitorContext({ workspaceId, conversationId }: SidebarVisitorContextProps) {
  const { data, isLoading } = useVisitorContext(workspaceId, conversationId);

  if (isLoading || !data?.contact) return null;

  const { contact } = data;
  const customProperties = Object.entries(contact.custom_properties ?? {})
    .filter(([key]) => !isSystemCRMCustomProperty(key))
    .map(([key, value]) => [key, formatUnknownValue(value)] as const)
    .filter(([, value]) => value !== null);
  if (!contact.job_title && !contact.phone && customProperties.length === 0) return null;

  return (
    <CollapsibleSection title="Contact details" count={0} persistKey="support-details.contact">
      <div className="space-y-2">
        {contact.job_title && <InfoRow label="Job title" value={contact.job_title} />}
        {contact.phone && <InfoRow label="Phone" value={contact.phone} />}
        {customProperties.map(([key, value]) => (
          <InfoRow key={key} label={key.replaceAll('_', ' ')} value={value} />
        ))}
      </div>
    </CollapsibleSection>
  );
}

/** Session details: the visitor's latest browsing session for this conversation. */
export function SidebarSessionDetails({ workspaceId, conversationId }: SidebarVisitorContextProps) {
  const { data, isLoading } = useVisitorContext(workspaceId, conversationId);

  if (isLoading || !data) return null;

  const { device, location } = data;
  const hasDevice = device && device.browser !== 'Unknown';
  const locationText = [location?.city_name, location?.region_name, location?.country_name].filter(Boolean).join(', ');
  const localTime = formatLocalTime(location?.timezone);
  const language = formatLocale(location?.locale);
  const currentPage = location?.last_page_url;
  const deviceType = device?.device_type ? `${device.device_type.slice(0, 1).toUpperCase()}${device.device_type.slice(1)}` : null;
  const browserAndOS = device
    ? [
        [device.browser, device.browser_version].filter(Boolean).join(' '),
        [device.os, device.os_version].filter(Boolean).join(' '),
      ].filter(Boolean).join(' · ')
    : null;

  if (!locationText && !localTime && !language && !currentPage && !hasDevice) return null;

  return (
    <CollapsibleSection title="Session details" count={0} persistKey="support-details.session">
      <div className="space-y-2">
        {currentPage && (
          <MainInfoRow
            label="Current page"
            icon={<Link01Icon className="h-3.5 w-3.5" />}
            value={currentPage.replace(/^https?:\/\//, '')}
            href={currentPage}
          />
        )}
        {locationText && (
          <MainInfoRow label="Location" icon={<MapPinIcon className="h-3.5 w-3.5" />} value={locationText} />
        )}
        {localTime && (
          <MainInfoRow label="Local time" icon={<Clock01Icon className="h-3.5 w-3.5" />} value={localTime} />
        )}
        {language && (
          <MainInfoRow label="Language" icon={<LanguageCircleIcon className="h-3.5 w-3.5" />} value={language} />
        )}
        {hasDevice && deviceType && (
          <MainInfoRow label="Device" icon={<DeviceIcon type={device.device_type} />} value={deviceType} />
        )}
        {hasDevice && browserAndOS && (
          <MainInfoRow label="Browser and OS" icon={<BrowserIcon browser={device.browser} />} value={browserAndOS} />
        )}
      </div>
    </CollapsibleSection>
  );
}

function formatUnknownValue(value: unknown): string | null {
  if (value === null || value === undefined || value === '') return null;
  if (typeof value === 'boolean') return value ? 'Yes' : 'No';
  if (typeof value === 'string' || typeof value === 'number') return String(value);
  return JSON.stringify(value);
}

export function SidebarOtherConversations({ workspaceId, conversationId }: SidebarVisitorContextProps) {
  const { data, isLoading } = useVisitorContext(workspaceId, conversationId);
  const workspace = useWorkspaceStore((s) => s.currentWorkspace);
  if (isLoading || !data || data.other_conversations.length === 0) return null;
  return (
    <CollapsibleSection key={`${workspaceId}:${conversationId}`} autoOpenWhenPopulated persistKey="support-details.recent-conversations" title="Recent conversations" count={Math.max(data.total_conversations - 1, data.other_conversations.length)}>
      <div className="max-h-[240px] space-y-0.5 overflow-y-auto">
        {data.other_conversations.map((conv) => <RelatedConversationRow key={conv.id} workspaceId={workspaceId} currentId={conversationId} conversation={conv} slug={workspace?.slug ?? ''} />)}
      </div>
    </CollapsibleSection>
  );
}

type RelatedConversationState = ConversationStatus | 'merged';

const RELATED_CONVERSATION_STATUS: Record<RelatedConversationState, { icon: React.ElementType; className: string }> = {
  open: { icon: CircleIcon, className: 'text-blue-500 dark:text-blue-400' },
  waiting_on_customer: { icon: Clock03Icon, className: 'text-purple-500 dark:text-purple-400' },
  resolved: { icon: CheckmarkCircle02Icon, className: 'text-emerald-600 dark:text-emerald-400' },
  spam: { icon: OctagonXIcon, className: 'text-red-500 dark:text-red-400' },
  merged: { icon: GitMergeIcon, className: 'text-muted-foreground' },
};

// A row opens the conversation in a read-only preview with Open and Merge, so
// the open thread stays put. Modified clicks still follow the link.
function RelatedConversationRow({ workspaceId, currentId, conversation: c, slug }: { workspaceId: string; currentId: string; conversation: VisitorOtherConversation; slug: string }) {
  const [action, setAction] = useState<'preview' | 'merge' | null>(null);
  const { data: access } = useWorkspaceAccess(workspaceId);
  const { data: source, error: sourceError } = useConversation(workspaceId, action ? c.id : null);
  const { data: target, error: targetError } = useConversation(workspaceId, action ? currentId : null);
  const state: RelatedConversationState = c.merged_into_id ? 'merged' : c.status as ConversationStatus;
  const status = RELATED_CONVERSATION_STATUS[state] ?? RELATED_CONVERSATION_STATUS.open;
  const statusLabel = c.merged_into_id ? 'Merged' : STATUS_LABELS[c.status as ConversationStatus] ?? c.status;
  const StatusIcon = status.icon;
  const canMerge = !c.merged_into_id && access?.permissions.includes('support.merge');
  const rowContentClassName = 'flex min-w-0 flex-1 items-center gap-2 px-1 py-1.5 text-left';
  const content = <>
    <QuickTooltip label={statusLabel}>
      <span aria-label={statusLabel} className={cn('flex h-3.5 w-3.5 shrink-0 items-center justify-center', status.className)}>
        <StatusIcon className="h-3.5 w-3.5" />
      </span>
    </QuickTooltip>
    <span className={cn('min-w-0 flex-1 truncate', quietRelatedItemTitleClassName)}>{c.subject}</span>
    <span className="shrink-0 text-[10px] tabular-nums text-muted-foreground">#{c.display_id}</span>
  </>;
  const close = (open: boolean) => { if (!open) setAction(null); };
  const actions = c.merged_into_id ? undefined : <>
    {canMerge && target && <Button variant="outline" onClick={() => setAction('merge')}>Merge</Button>}
    <Button asChild><Link to="/w/$slug/support/$conversationId" params={{ slug, conversationId: c.id }} onClick={() => setAction(null)}>Open conversation</Link></Button>
  </>;
  return <div className="group relative flex items-center rounded-md text-xs transition-colors hover:bg-quiet-hover focus-within:bg-quiet-hover">
    {c.merged_into_id ? <button type="button" className={rowContentClassName} onClick={() => setAction('preview')}>{content}</button> : <Link to="/w/$slug/support/$conversationId" params={{ slug, conversationId: c.id }} className={rowContentClassName} onClick={(event) => { if (!isPlainLeftClick(event)) return; event.preventDefault(); setAction('preview'); }}>{content}</Link>}
    {action && (sourceError || targetError) && <span role="alert" className="text-xs text-destructive">Could not open. <button onClick={() => setAction(null)}>Dismiss</button></span>}
    {action && !sourceError && !targetError && (!source || !target) && <span role="status" className="pr-2 text-muted-foreground">Loading…</span>}
    {source && target && action === 'merge' && <ConversationMergeDialog workspaceId={workspaceId} conversations={[source]} initialTarget={target} open onOpenChange={close} />}
    {source && target && action === 'preview' && <ConversationHistoryDialog workspaceId={workspaceId} conversation={source} actions={actions} open onOpenChange={close} />}
  </div>;
}
