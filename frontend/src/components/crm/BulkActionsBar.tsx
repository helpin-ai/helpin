import { useState, useCallback } from 'react';
import { Button } from '@/components/ui/button';
import { Badge } from '@/components/ui/badge';
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select';
import { MemberPickerPopover } from '@/components/pm/MemberPickerPopover';
import { Delete01Icon, UserAdd01Icon } from '@/lib/icons';
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

  if (count === 0) return null;

  return (
    <div className="animate-in slide-in-from-bottom-2 absolute bottom-4 left-1/2 z-20 flex -translate-x-1/2 items-center gap-2 rounded-lg border bg-card px-3 py-2 shadow-lg">
      <Badge variant="secondary" className="text-xs">
        {count} selected
      </Badge>

      {/* Change Stage */}
      <Select
        size="sm"
        disabled={loading}
        onValueChange={(v) => bulkUpdate({ lifecycle_stage: v })}
      >
        <SelectTrigger className="h-7 w-[110px] text-xs">
          <SelectValue placeholder="Stage" />
        </SelectTrigger>
        <SelectContent>
          {LIFECYCLE_STAGES.map((s) => (
            <SelectItem key={s.value} value={s.value}>
              {s.label}
            </SelectItem>
          ))}
        </SelectContent>
      </Select>

      {/* Change Status */}
      <Select
        size="sm"
        disabled={loading}
        onValueChange={(v) => bulkUpdate({ lead_status: v })}
      >
        <SelectTrigger className="h-7 w-[110px] text-xs">
          <SelectValue placeholder="Status" />
        </SelectTrigger>
        <SelectContent>
          {LEAD_STATUSES.map((s) => (
            <SelectItem key={s.value} value={s.value}>
              {s.label}
            </SelectItem>
          ))}
        </SelectContent>
      </Select>

      {/* Assign Owner */}
      <MemberPickerPopover
        value="__none__"
        members={assignableMembers}
        noneLabel="Unassigned"
        onChange={(value) => {
          bulkUpdate({ owner_member_id: value === '__none__' ? '' : value });
        }}
        triggerClassName="flex h-7 items-center gap-1 rounded-md border border-input bg-transparent px-2 text-xs"
        contentClassName="w-[220px]"
        renderTrigger={() => (
          <span className="flex items-center gap-1 text-muted-foreground">
            <UserAdd01Icon className="h-3 w-3" /> Assign
          </span>
        )}
      />

      {/* Delete */}
      <Button
        variant="destructive"
        size="sm"
        className="h-7 text-xs"
        disabled={loading}
        onClick={bulkDelete}
      >
        <Delete01Icon className="mr-1 h-3 w-3" />
        Delete
      </Button>

      {/* Clear selection */}
      <Button
        variant="ghost"
        size="sm"
        className="h-7 px-2 text-xs text-muted-foreground"
        onClick={onClearSelection}
      >
        Cancel
      </Button>
    </div>
  );
}
