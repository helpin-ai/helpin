import { Link } from '@tanstack/react-router';
import {
  ArrowLeftRightIcon,
  GlobeIcon,
  Message01Icon,
  UserIcon,
  ChromeIcon,
  LaptopIcon,
  SmartPhone01Icon,
  Tablet01Icon,
  MapPinIcon,
  Clock01Icon,
  Link01Icon,
  LanguageCircleIcon,
} from '@/lib/icons';
import { Badge } from '@/components/ui/badge';
import { CollapsibleSection } from '@/components/ui/collapsible-section';
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/tooltip';
import { STATUS_COLORS, STATUS_LABELS } from './constants';
import { useConversation, useVisitorContext } from '@/hooks/queries/useSupport';
import { useWorkspaceStore } from '@/stores/workspaceStore';
import type { ConversationStatus } from '@/lib/pmTypes';

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

const LIFECYCLE_COLORS: Record<string, string> = {
  subscriber: 'bg-gray-100 text-gray-600 dark:bg-gray-800 dark:text-gray-400',
  lead: 'bg-blue-100 text-blue-700 dark:bg-blue-950/30 dark:text-blue-400',
  opportunity: 'bg-amber-100 text-amber-700 dark:bg-amber-950/30 dark:text-amber-400',
  customer: 'bg-green-100 text-green-700 dark:bg-green-950/30 dark:text-green-400',
  evangelist: 'bg-purple-100 text-purple-700 dark:bg-purple-950/30 dark:text-purple-400',
  other: 'bg-gray-100 text-gray-600 dark:bg-gray-800 dark:text-gray-400',
};

const CHANNEL_LABELS: Record<string, string> = {
  widget: 'Chat',
  email: 'Email',
  api: 'API',
  internal: 'Internal',
};

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

export function SidebarVisitorContext({ workspaceId, conversationId }: SidebarVisitorContextProps) {
  const { data, isLoading } = useVisitorContext(workspaceId, conversationId);
  const { data: conversation } = useConversation(workspaceId, conversationId);

  if (isLoading || !data) return null;

  const { device, location, contact } = data;

  const hasDevice = device && device.browser !== 'Unknown';
  const locationText = [location?.city_name, location?.region_name, location?.country_name].filter(Boolean).join(', ');
  const localTime = formatLocalTime(location?.timezone);
  const language = formatLocale(location?.locale);
  const channel = conversation?.source;
  const channelLabel = channel ? CHANNEL_LABELS[channel] ?? channel : null;
  const currentPage = location?.last_page_url;
  const deviceType = device?.device_type ? `${device.device_type.slice(0, 1).toUpperCase()}${device.device_type.slice(1)}` : null;
  const browserAndOS = device
    ? [
        [device.browser, device.browser_version].filter(Boolean).join(' '),
        [device.os, device.os_version].filter(Boolean).join(' '),
      ].filter(Boolean).join(' · ')
    : null;

  const hasMainInfo = !!(locationText || localTime || language || channelLabel || currentPage || hasDevice);
  const hasContact = !!contact;
  if (!hasDevice && !hasMainInfo && !hasContact) return null;

  return (
    <CollapsibleSection title="Contact Details" icon={UserIcon} count={0} defaultOpen>
      <div className="space-y-2">
        {contact?.job_title && <InfoRow label="Job title" value={contact.job_title} />}
        {contact?.lifecycle_stage && (
          <div className="grid grid-cols-[88px_1fr] items-center gap-2 text-[12px]">
            <span className="text-muted-foreground">Lifecycle</span>
            <span>
              <Badge variant="secondary" className={`h-4 rounded-full px-1.5 text-[10px] font-medium ${LIFECYCLE_COLORS[contact.lifecycle_stage] ?? LIFECYCLE_COLORS.other}`}>
                {contact.lifecycle_stage}
              </Badge>
            </span>
          </div>
        )}
        {contact?.lead_status && <InfoRow label="Lead status" value={contact.lead_status.replaceAll('_', ' ')} />}
        {contact?.phone && <InfoRow label="Phone" value={contact.phone} />}
        {contact?.custom_properties && Object.entries(contact.custom_properties).map(([key, value]) => (
          <InfoRow key={key} label={key.replaceAll('_', ' ')} value={formatUnknownValue(value)} />
        ))}

        {hasMainInfo && (
          <>
            <div className="pt-1 text-[10px] font-medium uppercase tracking-wider text-muted-foreground">Current visit</div>
            {channelLabel && (
              <MainInfoRow label="Channel" icon={<ArrowLeftRightIcon className="h-3.5 w-3.5" />} value={channelLabel} />
            )}
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
          </>
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
    <CollapsibleSection title="Other Conversations" icon={Message01Icon} count={Math.max(data.total_conversations - 1, data.other_conversations.length)}>
      <div className="max-h-[240px] space-y-1.5 overflow-y-auto pr-1">
        {data.other_conversations.map((conv) => (
          <Link
            key={conv.id}
            to="/w/$slug/support/$conversationId"
            params={{ slug: workspace?.slug ?? '', conversationId: conv.id }}
            className="flex items-center gap-2 rounded-md border px-2 py-1.5 text-[12px] transition-colors hover:bg-accent"
          >
            <span className="shrink-0 text-muted-foreground">#{conv.display_id}</span>
            <span className="flex-1 truncate font-medium">{conv.subject}</span>
            <Badge variant="secondary" className={`h-4 shrink-0 px-1 text-[9px] ${STATUS_COLORS[conv.status as ConversationStatus] ?? 'bg-gray-100 text-gray-600'}`}>
              {STATUS_LABELS[conv.status as ConversationStatus] ?? conv.status}
            </Badge>
          </Link>
        ))}
      </div>
    </CollapsibleSection>
  );
}
