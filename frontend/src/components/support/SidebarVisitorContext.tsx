import { Link } from '@tanstack/react-router';
import * as Flags from 'country-flag-icons/react/3x2';
import type { JSX, SVGProps } from 'react';
import {
  GlobeIcon,
  Mail01Icon,
  Message01Icon,
  ComputerIcon,
  UserIcon,
  ChromeIcon,
  LaptopIcon,
  SmartPhone01Icon,
  Tablet01Icon,
  MapPinIcon,
  Clock01Icon,
  Link01Icon,
  InformationCircleIcon,
} from '@/lib/icons';
import { Badge } from '@/components/ui/badge';
import { CollapsibleSection } from '@/components/ui/collapsible-section';
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
    <div className="flex items-center justify-between gap-2 text-xs">
      <span className="text-muted-foreground shrink-0 flex items-center gap-1.5">
        {Icon && <Icon className="h-3 w-3" />}
        {label}
      </span>
      <span className="truncate text-right" title={value}>{value}</span>
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

function OSIcon({ os }: { os: string }) {
  // Use Monitor for all OS — lucide doesn't have Apple/Windows/Linux icons
  switch (os.toLowerCase()) {
    case 'macos': return <ComputerIcon className="h-3 w-3" />;
    case 'windows': return <ComputerIcon className="h-3 w-3" />;
    case 'linux': return <ComputerIcon className="h-3 w-3" />;
    case 'ios': return <SmartPhone01Icon className="h-3 w-3" />;
    case 'android': return <SmartPhone01Icon className="h-3 w-3" />;
    default: return <ComputerIcon className="h-3 w-3" />;
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

const SOURCE_ICONS: Record<string, React.ElementType> = {
  live_chat: Message01Icon,
  support: Message01Icon,
  email: Mail01Icon,
  widget: Message01Icon,
  api: GlobeIcon,
  manual: UserIcon,
};

const SOURCE_LABELS: Record<string, string> = {
  live_chat: 'Live Chat',
  support: 'Support',
  email: 'Email',
  widget: 'Widget',
  api: 'API',
  manual: 'Manual',
};

const CHANNEL_LABELS: Record<string, string> = {
  widget: 'Chat',
  email: 'Email',
  api: 'API',
  internal: 'Internal',
};

const CHANNEL_ICONS: Record<string, React.ElementType> = {
  widget: Message01Icon,
  email: Mail01Icon,
  api: GlobeIcon,
  internal: UserIcon,
};

function normalizeCountryCode(code?: string | null): keyof typeof Flags | null {
  const normalized = code?.trim().toUpperCase().replace(/-/g, '_');
  if (!normalized || !/^[A-Z]{2,3}(?:_[A-Z]{2,3})?$/.test(normalized)) {
    return null;
  }
  return normalized as keyof typeof Flags;
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

function MainInfoRow({
  icon,
  value,
  href,
}: {
  icon: React.ReactNode;
  value: string;
  href?: string;
}) {
  return (
    <div className="flex items-center gap-2 text-xs">
      <span className="flex h-4 w-4 shrink-0 items-center justify-center text-muted-foreground">
        {icon}
      </span>
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
        <span className="truncate" title={value}>{value}</span>
      )}
    </div>
  );
}

export function SidebarVisitorContext({ workspaceId, conversationId }: SidebarVisitorContextProps) {
  const { data, isLoading } = useVisitorContext(workspaceId, conversationId);
  const { data: conversation } = useConversation(workspaceId, conversationId);
  const workspace = useWorkspaceStore((s) => s.currentWorkspace);

  if (isLoading || !data) return null;

  const { device, location, contact, other_conversations, total_conversations } = data;

  const hasDevice = device && device.browser !== 'Unknown';
  const locationText = [location?.city_name, location?.region_name, location?.country_name].filter(Boolean).join(', ');
  const localTime = formatLocalTime(location?.timezone);
  const flagKey = normalizeCountryCode(location?.country_code);
  const Flag = flagKey ? (Flags[flagKey] as ((p: SVGProps<SVGSVGElement>) => JSX.Element) | undefined) : undefined;
  const channel = conversation?.source;
  const channelLabel = channel ? CHANNEL_LABELS[channel] ?? channel : null;
  const ChannelIcon = channel ? CHANNEL_ICONS[channel] : undefined;
  const currentPage = location?.last_page_url;

  const hasMainInfo = !!(locationText || localTime || Flag || conversation?.customer_email || channelLabel || currentPage);
  const hasContact = !!contact;
  const hasOtherConvos = other_conversations.length > 0;

  if (!hasDevice && !hasMainInfo && !hasContact && !hasOtherConvos) return null;

  return (
    <div>
      {/* Main information */}
      {hasMainInfo && (
        <CollapsibleSection title="Main information" icon={InformationCircleIcon} count={0} defaultOpen>
          <div className="space-y-2">
            {locationText && (
              <MainInfoRow icon={<MapPinIcon className="h-3.5 w-3.5" />} value={locationText} />
            )}
            {localTime && (
              <MainInfoRow icon={<Clock01Icon className="h-3.5 w-3.5" />} value={localTime} />
            )}
            {Flag && location?.country_name && (
              <div className="flex items-center gap-2 text-xs">
                <span className="flex h-4 w-4 shrink-0 items-center justify-center">
                  <span className="flex h-[11px] w-4 items-center justify-center overflow-hidden rounded-[2px] border border-border/60">
                    <Flag className="h-full w-full object-cover" />
                  </span>
                </span>
                <span className="truncate" title={location.country_name}>{location.country_name}</span>
              </div>
            )}
            {conversation?.customer_email && (
              <MainInfoRow icon={<Mail01Icon className="h-3.5 w-3.5" />} value={conversation.customer_email} />
            )}
            {channelLabel && ChannelIcon && (
              <MainInfoRow icon={<ChannelIcon className="h-3.5 w-3.5" />} value={channelLabel} />
            )}
            {currentPage && (
              <MainInfoRow
                icon={<Link01Icon className="h-3.5 w-3.5" />}
                value={currentPage.replace(/^https?:\/\//, '')}
                href={currentPage}
              />
            )}
          </div>
        </CollapsibleSection>
      )}

      {/* Visitor device */}
      {hasDevice && (
        <CollapsibleSection title="Visitor device" icon={ComputerIcon} count={0} defaultOpen>
          <div className="space-y-2">
            <div className="flex items-center gap-2 text-xs">
              <BrowserIcon browser={device.browser} />
              <span>{device.browser}{device.browser_version ? ` ${device.browser_version}` : ''}</span>
            </div>
            <div className="flex items-center gap-2 text-xs">
              <OSIcon os={device.os} />
              <span>{device.os}{device.os_version ? ` ${device.os_version}` : ''}</span>
            </div>
            <div className="flex items-center gap-2 text-xs">
              <DeviceIcon type={device.device_type} />
              <span className="capitalize">{device.device_type}</span>
            </div>
          </div>
        </CollapsibleSection>
      )}

      {/* Contact Details */}
      {hasContact && (
        <CollapsibleSection title="Contact Details" icon={UserIcon} count={0} defaultOpen>
          <div className="space-y-2">
            {contact.job_title && (
              <InfoRow label="Job title" value={contact.job_title} />
            )}
            {contact.lifecycle_stage && (
              <div className="flex items-center justify-between gap-2 text-xs">
                <span className="text-muted-foreground shrink-0">Lifecycle</span>
                <Badge variant="secondary" className={`h-4 px-1.5 text-[10px] font-medium ${LIFECYCLE_COLORS[contact.lifecycle_stage] ?? LIFECYCLE_COLORS.other}`}>
                  {contact.lifecycle_stage}
                </Badge>
              </div>
            )}
            {contact.source && (
              <div className="flex items-center justify-between gap-2 text-xs">
                <span className="text-muted-foreground shrink-0">Source</span>
                <span className="flex items-center gap-1 truncate text-right capitalize">
                  {(() => {
                    const SourceIcon = SOURCE_ICONS[contact.source] ?? GlobeIcon;
                    return <SourceIcon className="h-3 w-3 text-muted-foreground shrink-0" />;
                  })()}
                  {SOURCE_LABELS[contact.source] ?? contact.source.replace(/_/g, ' ')}
                </span>
              </div>
            )}
            {contact.custom_properties && Object.keys(contact.custom_properties).length > 0 && (
              <>
                <div className="text-[10px] font-medium text-muted-foreground uppercase tracking-wider pt-1">Custom data</div>
                {Object.entries(contact.custom_properties).map(([key, value]) => (
                  <InfoRow key={key} label={key} value={value} />
                ))}
              </>
            )}
          </div>
        </CollapsibleSection>
      )}

      {/* Other Conversations */}
      {hasOtherConvos && (
        <CollapsibleSection title="Other Conversations" icon={Message01Icon} count={Math.max(total_conversations - 1, other_conversations.length)}>
          <div className="max-h-[240px] space-y-1.5 overflow-y-auto pr-1">
            {other_conversations.map((conv) => (
              <Link
                key={conv.id}
                to="/w/$slug/support/$conversationId"
                params={{ slug: workspace?.slug ?? '', conversationId: conv.id }}
                className="flex items-center gap-2 rounded-md border px-2 py-1.5 text-xs hover:bg-accent transition-colors"
              >
                <span className="text-muted-foreground shrink-0">#{conv.display_id}</span>
                <span className="truncate flex-1 font-medium">{conv.subject}</span>
                <Badge variant="secondary" className={`h-4 px-1 text-[9px] shrink-0 ${STATUS_COLORS[conv.status as ConversationStatus] ?? 'bg-gray-100 text-gray-600'}`}>
                  {STATUS_LABELS[conv.status as ConversationStatus] ?? conv.status}
                </Badge>
              </Link>
            ))}
          </div>
        </CollapsibleSection>
      )}
    </div>
  );
}
