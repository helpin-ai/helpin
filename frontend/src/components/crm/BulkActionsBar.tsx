import { useCallback, useMemo, useState } from 'react';
import { toast } from 'sonner';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select';
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/tooltip';
import { ConfirmDialog } from '@/components/pm/ConfirmDialog';
import { MemberPickerPopover } from '@/components/pm/MemberPickerPopover';
import { AiMagicIcon, Cancel01Icon, Delete01Icon, Loading01Icon, UserAdd01Icon } from '@/lib/icons';
import { crmContactService } from '@/lib/services/crmService';
import type { CRMContact, LifecycleStage, LeadStatus } from '@/lib/crmTypes';
import type { DockEntityReference } from '@/lib/dockTypes';
import type { AssignableMember } from '@/lib/types';

const LIFECYCLE_STAGES: { value: LifecycleStage; label: string }[] = [
  { value: 'subscriber', label: 'Subscriber' },
  { value: 'lead', label: 'Lead' },
  { value: 'marketing_qualified', label: 'MQL' },
  { value: 'sales_qualified', label: 'SQL' },
  { value: 'opportunity', label: 'Opportunity' },
  { value: 'customer', label: 'Customer' },
  { value: 'evangelist', label: 'Evangelist' },
];

const LEAD_STATUSES: { value: LeadStatus; label: string }[] = [
  { value: 'new', label: 'New' },
  { value: 'open', label: 'Open' },
  { value: 'in_progress', label: 'In Progress' },
  { value: 'unqualified', label: 'Unqualified' },
];

/** The Ask Agent dock attaches at most this many references to a message. */
const ASK_AGENT_REFERENCE_LIMIT = 10;

type ContactPatch = Parameters<typeof crmContactService.update>[2];

function contactDisplayTitle(contact: Pick<CRMContact, 'first_name' | 'last_name' | 'email'>): string {
  return [contact.first_name, contact.last_name].filter(Boolean).join(' ') || contact.email || 'CRM contact';
}

/** Dock references for a selection, capped at the dock's per-message limit. */
function buildContactReferences(contacts: CRMContact[]): DockEntityReference[] {
  return contacts.slice(0, ASK_AGENT_REFERENCE_LIMIT).map((contact) => ({
    entity_type: 'crm_contact',
    entity_id: contact.id,
    display_id: contact.display_id,
    display_title: contactDisplayTitle(contact),
  }));
}

interface BulkActionsBarProps {
  selectedContacts: CRMContact[];
  workspaceId: string;
  assignableMembers: AssignableMember[];
  onComplete: () => void;
  onClearSelection: () => void;
}

/**
 * Floating action bar shown while contacts are selected. It sits over the
 * table so the available actions are visible right where the selection was made.
 */
export function BulkActionsBar({
  selectedContacts,
  workspaceId,
  assignableMembers,
  onComplete,
  onClearSelection,
}: BulkActionsBarProps) {
  const [loading, setLoading] = useState(false);
  const [deleteConfirmOpen, setDeleteConfirmOpen] = useState(false);
  const count = selectedContacts.length;
  const noun = count === 1 ? 'contact' : 'contacts';
  const selectedIds = useMemo(() => selectedContacts.map((contact) => contact.id), [selectedContacts]);

  const runBulk = useCallback(
    async (verb: string, operation: (id: string) => Promise<{ error?: string | null } | null | undefined | void>) => {
      if (loading || count === 0) return;
      setLoading(true);
      try {
        const results = await Promise.allSettled(selectedIds.map((id) => operation(id)));
        const failureCount = results.filter((result) => (
          result.status === 'rejected' || !!result.value?.error
        )).length;
        const successCount = results.length - failureCount;
        if (failureCount === 0) {
          toast.success(`${verb} ${successCount} ${successCount === 1 ? 'contact' : 'contacts'}`);
        } else if (successCount > 0) {
          toast.warning(`${verb} ${successCount} of ${results.length} contacts`, {
            description: `${failureCount} ${failureCount === 1 ? 'contact failed' : 'contacts failed'}.`,
          });
        } else {
          toast.error('Bulk operation failed', {
            description: `${failureCount} ${failureCount === 1 ? 'request failed' : 'requests failed'}.`,
          });
        }
        onComplete();
        if (successCount > 0) onClearSelection();
      } finally {
        setLoading(false);
      }
    },
    [count, loading, onClearSelection, onComplete, selectedIds],
  );

  const bulkUpdate = useCallback(
    (patch: ContactPatch) => runBulk('Updated', (id) => crmContactService.update(workspaceId, id, patch)),
    [runBulk, workspaceId],
  );

  const bulkDelete = useCallback(
    () => runBulk('Deleted', (id) => crmContactService.remove(workspaceId, id)),
    [runBulk, workspaceId],
  );

  const askAgent = useCallback(() => {
    const references = buildContactReferences(selectedContacts);
    if (count > ASK_AGENT_REFERENCE_LIMIT) {
      toast.info(`Attached the first ${ASK_AGENT_REFERENCE_LIMIT} of ${count} contacts to Ask Agent.`);
    }
    window.dispatchEvent(new CustomEvent('helpin:ask-agents', {
      detail: { intent: 'new_chat', references },
    }));
  }, [count, selectedContacts]);

  if (count === 0) return null;

  const divider = <span aria-hidden="true" className="mx-0.5 h-4 w-px shrink-0 bg-border" />;

  return (
    <>
      <div
        role="toolbar"
        aria-label={`Actions for ${count} selected ${noun}`}
        className="pointer-events-auto flex max-w-full flex-wrap items-center gap-1 rounded-lg border border-border bg-popover px-2 py-1.5 text-xs text-popover-foreground shadow-lg"
      >
        <Badge variant="secondary" className="mr-1 shrink-0 gap-1 text-xs">
          {loading ? <Loading01Icon className="h-3 w-3 animate-spin" /> : null}
          {count} selected
        </Badge>
        {divider}

        <Select value="" disabled={loading} onValueChange={(value) => void bulkUpdate({ lifecycle_stage: value as LifecycleStage })}>
          <SelectTrigger size="sm" className="h-7 min-w-[96px] gap-1 border-0 bg-transparent px-2 text-xs shadow-none hover:bg-accent" aria-label="Set lifecycle stage">
            <SelectValue placeholder="Stage" />
          </SelectTrigger>
          <SelectContent>
            {LIFECYCLE_STAGES.map((stage) => (
              <SelectItem key={stage.value} value={stage.value}>{stage.label}</SelectItem>
            ))}
          </SelectContent>
        </Select>

        <Select value="" disabled={loading} onValueChange={(value) => void bulkUpdate({ lead_status: value as LeadStatus })}>
          <SelectTrigger size="sm" className="h-7 min-w-[96px] gap-1 border-0 bg-transparent px-2 text-xs shadow-none hover:bg-accent" aria-label="Set lead status">
            <SelectValue placeholder="Status" />
          </SelectTrigger>
          <SelectContent>
            {LEAD_STATUSES.map((status) => (
              <SelectItem key={status.value} value={status.value}>{status.label}</SelectItem>
            ))}
          </SelectContent>
        </Select>

        <MemberPickerPopover
          value="__none__"
          members={assignableMembers}
          noneLabel="Unassigned"
          onChange={(value) => {
            void bulkUpdate({ owner_member_id: value === '__none__' ? '' : value });
          }}
          triggerClassName="flex h-7 items-center gap-1.5 rounded-md px-2 text-xs hover:bg-accent"
          contentClassName="w-[260px]"
          renderTrigger={() => (
            <span className="flex items-center gap-1.5">
              <UserAdd01Icon className="h-3.5 w-3.5 text-muted-foreground" /> Assign owner
            </span>
          )}
        />
        {divider}

        <Tooltip>
          <TooltipTrigger asChild>
            <Button
              type="button"
              variant="ghost"
              size="sm"
              className="h-7 gap-1.5 px-2 text-xs text-primary hover:bg-primary/10 hover:text-primary"
              disabled={loading}
              onClick={askAgent}
            >
              <AiMagicIcon className="h-3.5 w-3.5" />
              Ask Agent
            </Button>
          </TooltipTrigger>
          <TooltipContent side="top">Start an Ask Agent chat with these {noun} attached</TooltipContent>
        </Tooltip>

        <Button
          type="button"
          variant="ghost"
          size="sm"
          className="h-7 gap-1.5 px-2 text-xs text-destructive hover:bg-destructive/10 hover:text-destructive"
          disabled={loading}
          onClick={() => setDeleteConfirmOpen(true)}
        >
          <Delete01Icon className="h-3.5 w-3.5" />
          Delete
        </Button>
        {divider}

        <Button
          type="button"
          variant="ghost"
          size="icon"
          className="h-7 w-7 text-muted-foreground"
          aria-label="Clear selection"
          disabled={loading}
          onClick={onClearSelection}
        >
          <Cancel01Icon className="h-3.5 w-3.5" />
        </Button>
      </div>

      <ConfirmDialog
        open={deleteConfirmOpen}
        onOpenChange={setDeleteConfirmOpen}
        title={`Delete ${count} ${noun}?`}
        description="Deleted contacts are removed from the workspace along with their activity history. This cannot be undone."
        confirmLabel="Delete"
        variant="destructive"
        onConfirm={() => {
          setDeleteConfirmOpen(false);
          void bulkDelete();
        }}
      />
    </>
  );
}
