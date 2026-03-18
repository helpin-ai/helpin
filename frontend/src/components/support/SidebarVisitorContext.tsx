import { Link } from '@tanstack/react-router';
import { Globe, Monitor, MessageSquare, User } from 'lucide-react';
import { Badge } from '@/components/ui/badge';
import { CollapsibleSection } from './CollapsibleSection';
import { STATUS_COLORS, STATUS_LABELS } from './constants';
import { useVisitorContext } from '@/hooks/queries/useSupport';
import { useWorkspaceStore } from '@/stores/workspaceStore';
import type { ConversationStatus } from '@/lib/pmTypes';

interface SidebarVisitorContextProps {
  workspaceId: string;
  conversationId: string;
}

function InfoRow({ label, value }: { label: string; value: string | null | undefined }) {
  if (!value) return null;
  return (
    <div className="flex items-center justify-between gap-2 text-xs">
      <span className="text-muted-foreground shrink-0">{label}</span>
      <span className="truncate text-right" title={value}>{value}</span>
    </div>
  );
}

export function SidebarVisitorContext({ workspaceId, conversationId }: SidebarVisitorContextProps) {
  const { data, isLoading } = useVisitorContext(workspaceId, conversationId);
  const workspace = useWorkspaceStore((s) => s.currentWorkspace);

  if (isLoading || !data) return null;

  const { device, location, contact, other_conversations, total_conversations } = data;

  const hasDevice = device && device.browser !== 'Unknown';
  const hasLocation = location && (location.timezone || location.locale || location.last_page_url);
  const hasContact = !!contact;
  const hasOtherConvos = other_conversations.length > 0;

  if (!hasDevice && !hasLocation && !hasContact && !hasOtherConvos) return null;

  return (
    <div>
      {/* Device */}
      {hasDevice && (
        <CollapsibleSection title="Device" icon={Monitor} count={0} >
          <div className="space-y-1.5">
            <InfoRow label="Browser" value={device.browser_version ? `${device.browser} ${device.browser_version}` : device.browser} />
            <InfoRow label="OS" value={device.os_version ? `${device.os} ${device.os_version}` : device.os} />
            <InfoRow label="Device" value={device.device_type} />
          </div>
        </CollapsibleSection>
      )}

      {/* Location */}
      {hasLocation && (
        <CollapsibleSection title="Location" icon={Globe} count={0} >
          <div className="space-y-1.5">
            <InfoRow label="Timezone" value={location.timezone} />
            <InfoRow label="Locale" value={location.locale} />
            {location.last_page_url && (
              <div className="text-xs">
                <span className="text-muted-foreground block mb-0.5">Current page</span>
                <a
                  href={location.last_page_url}
                  target="_blank"
                  rel="noopener noreferrer"
                  className="text-blue-600 dark:text-blue-400 hover:underline truncate block"
                  title={location.last_page_url}
                >
                  {location.last_page_url.replace(/^https?:\/\//, '')}
                </a>
              </div>
            )}
          </div>
        </CollapsibleSection>
      )}

      {/* Contact Details */}
      {hasContact && (
        <CollapsibleSection title="Contact Details" icon={User} count={0} >
          <div className="space-y-1.5">
            <InfoRow label="Job title" value={contact.job_title} />
            <InfoRow label="Lifecycle" value={contact.lifecycle_stage} />
            <InfoRow label="Lead status" value={contact.lead_status} />
            <InfoRow label="Source" value={contact.source || undefined} />
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
        <CollapsibleSection title="Other Conversations" icon={MessageSquare} count={total_conversations - 1}>
          <div className="space-y-1.5">
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
