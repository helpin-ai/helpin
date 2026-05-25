import { useState, useCallback } from 'react';
import { Button } from '@/components/ui/button';
import { Badge } from '@/components/ui/badge';
import { Popover, PopoverContent, PopoverTrigger } from '@/components/ui/popover';
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select';
import { MemberPickerPopover } from '@/components/pm/MemberPickerPopover';
import { ArrowDown01Icon, Delete01Icon, PencilEdit01Icon, UserAdd01Icon } from '@/lib/icons';
import { crmContactService } from '@/lib/services/crmService';
import type { LifecycleStage, LeadStatus } from '@/lib/crmTypes';
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

interface BulkActionsBarProps {
  selectedIds: string[];
  workspaceId: string;
  assignableMembers: AssignableMember[];
  onComplete: () => void;
  onClearSelection: () => void;
}

export function BulkActionsBar({
  selectedIds,
  workspaceId,
  assignableMembers,
  onComplete,
  onClearSelection,
}: BulkActionsBarProps) {
  const [loading, setLoading] = useState(false);
  const count = selectedIds.length;

  const bulkUpdate = useCallback(
    async (patch: Record<string, unknown>) => {
      setLoading(true);
      try {
        await Promise.all(
          selectedIds.map((id) => crmContactService.update(workspaceId, id, patch)),
        );
        onComplete();
        onClearSelection();
      } finally {
        setLoading(false);
      }
    },
    [selectedIds, workspaceId, onComplete, onClearSelection],
  );

  const bulkDelete = useCallback(async () => {
    setLoading(true);
    try {
      await Promise.all(
        selectedIds.map((id) => crmContactService.remove(workspaceId, id)),
      );
      onComplete();
      onClearSelection();
    } finally {
      setLoading(false);
    }
  }, [selectedIds, workspaceId, onComplete, onClearSelection]);

  const [open, setOpen] = useState(false);

  if (count === 0) return null;

  const fieldRow = 'flex items-center gap-2';
  const fieldLabel = 'w-20 shrink-0 text-xs text-muted-foreground';

  return (
    <Popover open={open} onOpenChange={setOpen}>
      <PopoverTrigger asChild>
        <Button
          type="button"
          variant="default"
          size="sm"
          className="h-7 shrink-0 gap-1.5 px-2.5 text-xs"
        >
          <PencilEdit01Icon className="h-3.5 w-3.5" />
          Edit {count} {count === 1 ? 'contact' : 'contacts'}
          <ArrowDown01Icon className="h-3 w-3 opacity-70" />
        </Button>
      </PopoverTrigger>
      <PopoverContent
        align="start"
        sideOffset={6}
        className="w-[340px] p-3"
        onClick={(event) => event.stopPropagation()}
      >
        <div className="mb-2 flex items-center justify-between gap-2">
          <Badge variant="secondary" className="shrink-0 text-xs">
            {count} selected
          </Badge>
          <Button
            type="button"
            variant="ghost"
            size="sm"
            className="h-6 px-2 text-xs text-muted-foreground"
            disabled={loading}
            onClick={() => {
              onClearSelection();
              setOpen(false);
            }}
          >
            Cancel
          </Button>
        </div>

        <div className="flex flex-col gap-2">
          <div className={fieldRow}>
            <span className={fieldLabel}>Stage</span>
            <Select
              size="sm"
              disabled={loading}
              onValueChange={(v) => bulkUpdate({ lifecycle_stage: v })}
            >
              <SelectTrigger className="h-7 flex-1 text-xs">
                <SelectValue placeholder="No change" />
              </SelectTrigger>
              <SelectContent>
                {LIFECYCLE_STAGES.map((s) => (
                  <SelectItem key={s.value} value={s.value}>
                    {s.label}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>

          <div className={fieldRow}>
            <span className={fieldLabel}>Status</span>
            <Select
              size="sm"
              disabled={loading}
              onValueChange={(v) => bulkUpdate({ lead_status: v })}
            >
              <SelectTrigger className="h-7 flex-1 text-xs">
                <SelectValue placeholder="No change" />
              </SelectTrigger>
              <SelectContent>
                {LEAD_STATUSES.map((s) => (
                  <SelectItem key={s.value} value={s.value}>
                    {s.label}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>

          <div className={fieldRow}>
            <span className={fieldLabel}>Owner</span>
            <MemberPickerPopover
              value="__none__"
              members={assignableMembers}
              noneLabel="Unassigned"
              onChange={(value) => {
                bulkUpdate({ owner_member_id: value === '__none__' ? '' : value });
              }}
              triggerClassName="flex h-7 flex-1 items-center gap-1 rounded-md border border-input bg-transparent px-2 text-xs"
              contentClassName="w-[260px]"
              renderTrigger={() => (
                <span className="flex items-center gap-1 text-muted-foreground">
                  <UserAdd01Icon className="h-3 w-3" /> Assign owner
                </span>
              )}
            />
          </div>

          <div className="mt-2 flex items-center gap-2 border-t border-border/60 pt-2">
            <Button
              type="button"
              variant="destructive"
              size="sm"
              className="h-7 flex-1 px-2 text-xs"
              disabled={loading}
              onClick={bulkDelete}
            >
              <Delete01Icon className="mr-1 h-3 w-3" />
              Delete
            </Button>
          </div>
        </div>
      </PopoverContent>
    </Popover>
  );
}
