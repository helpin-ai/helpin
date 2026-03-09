import { useState } from 'react';
import { formatDistanceToNow } from 'date-fns';
import { Mail, MessageSquare, Phone, Calendar, CheckSquare, Trash2 } from 'lucide-react';
import { toast } from 'sonner';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Textarea } from '@/components/ui/textarea';
import { ConfirmDialog } from '@/components/pm/ConfirmDialog';
import { useCreateCRMActivity, useDeleteCRMActivity } from '@/hooks/queries/useCRM';
import type { CRMActivity, CRMActivityType } from '@/lib/crmTypes';

interface ActivityTimelineProps {
  activities: CRMActivity[];
  workspaceId?: string;
  contactId?: string;
  companyId?: string;
  dealId?: string;
  onActivityCreated?: () => void;
  onActivityDeleted?: () => void;
}

const activityIcons: Record<CRMActivityType, typeof Mail> = {
  note: MessageSquare,
  call: Phone,
  meeting: Calendar,
  email: Mail,
  task: CheckSquare,
};

const activityTypes: { type: CRMActivityType; icon: typeof MessageSquare; label: string }[] = [
  { type: 'note', icon: MessageSquare, label: 'Note' },
  { type: 'call', icon: Phone, label: 'Call' },
  { type: 'meeting', icon: Calendar, label: 'Meeting' },
  { type: 'task', icon: CheckSquare, label: 'Task' },
];

const filterOptions = ['all', 'note', 'call', 'meeting', 'email', 'task'] as const;

export function ActivityTimeline({
  activities,
  workspaceId,
  contactId,
  companyId,
  dealId,
  onActivityCreated,
  onActivityDeleted,
}: ActivityTimelineProps) {
  const [filterType, setFilterType] = useState<CRMActivityType | 'all'>('all');
  const [creatingType, setCreatingType] = useState<CRMActivityType | null>(null);
  const [newSubject, setNewSubject] = useState('');
  const [newBody, setNewBody] = useState('');
  const [deleteId, setDeleteId] = useState<string | null>(null);

  const createActivity = useCreateCRMActivity(workspaceId ?? '');
  const deleteActivity = useDeleteCRMActivity(workspaceId ?? '');

  const filtered = filterType === 'all' ? activities : activities.filter((a) => a.activity_type === filterType);

  const handleCreate = async () => {
    if (!workspaceId || !newSubject.trim() || !creatingType) return;
    try {
      await createActivity.mutateAsync({
        workspace_id: workspaceId,
        activity_type: creatingType,
        contact_id: contactId,
        company_id: companyId,
        deal_id: dealId,
        subject: newSubject.trim(),
        body: newBody.trim() || undefined,
      });
      toast.success('Activity logged');
      setCreatingType(null);
      setNewSubject('');
      setNewBody('');
      onActivityCreated?.();
    } catch {
      toast.error('Failed to log activity');
    }
  };

  const handleDeleteConfirm = async () => {
    if (!deleteId || !workspaceId) return;
    try {
      await deleteActivity.mutateAsync(deleteId);
      toast.success('Activity deleted');
      setDeleteId(null);
      onActivityDeleted?.();
    } catch {
      toast.error('Failed to delete activity');
    }
  };

  const creationButtons = workspaceId ? (
    <div className="mb-4 flex gap-1.5">
      {activityTypes.map(({ type, icon: Icon, label }) => (
        <Button
          key={type}
          variant={creatingType === type ? 'default' : 'outline'}
          size="sm"
          onClick={() => {
            setCreatingType(creatingType === type ? null : type);
            setNewSubject('');
            setNewBody('');
          }}
        >
          <Icon className="mr-1 h-3.5 w-3.5" />
          {label}
        </Button>
      ))}
    </div>
  ) : null;

  return (
    <div>
      {creationButtons}

      {creatingType && (
        <div className="mb-4 space-y-2 rounded-md border p-3">
          <Input
            placeholder="Subject"
            value={newSubject}
            onChange={(e) => setNewSubject(e.target.value)}
            className="text-sm"
          />
          <Textarea
            placeholder="Details..."
            value={newBody}
            onChange={(e) => setNewBody(e.target.value)}
            className="min-h-[60px] text-sm"
            rows={2}
          />
          <div className="flex justify-end gap-2">
            <Button variant="ghost" size="sm" onClick={() => setCreatingType(null)}>
              Cancel
            </Button>
            <Button
              size="sm"
              disabled={!newSubject.trim() || createActivity.isPending}
              onClick={handleCreate}
            >
              {createActivity.isPending ? 'Saving...' : 'Save'}
            </Button>
          </div>
        </div>
      )}

      <div className="mb-3 flex gap-1 border-b pb-2">
        {filterOptions.map((type) => (
          <button
            key={type}
            type="button"
            className={`rounded-md px-2.5 py-1 text-xs transition-colors ${
              filterType === type
                ? 'bg-accent font-medium text-foreground'
                : 'text-muted-foreground hover:text-foreground'
            }`}
            onClick={() => setFilterType(type)}
          >
            {type === 'all' ? 'All' : `${type.charAt(0).toUpperCase()}${type.slice(1)}s`}
          </button>
        ))}
      </div>

      {filtered.length === 0 ? (
        <div className="flex flex-col items-center justify-center py-8 text-center">
          <div className="flex h-12 w-12 items-center justify-center rounded-full bg-muted">
            <MessageSquare className="h-6 w-6 text-muted-foreground/50" />
          </div>
          <p className="mt-3 text-sm font-medium">No activities yet</p>
          <p className="mt-1 text-xs text-muted-foreground">Log your first activity to track interactions</p>
        </div>
      ) : (
        <div className="space-y-4">
          {filtered.map((activity) => {
            const Icon = activityIcons[activity.activity_type] ?? MessageSquare;
            return (
              <div key={activity.id} className="group flex gap-3">
                <div className="flex h-8 w-8 shrink-0 items-center justify-center rounded-full bg-muted">
                  <Icon className="h-4 w-4 text-muted-foreground" />
                </div>
                <div className="min-w-0 flex-1">
                  <div className="flex items-center gap-2">
                    <span className="text-sm font-medium capitalize">{activity.activity_type}</span>
                    <span className="text-xs text-muted-foreground">
                      {formatDistanceToNow(new Date(activity.occurred_at), { addSuffix: true })}
                    </span>
                    {workspaceId && (
                      <div className="ml-auto flex gap-1 opacity-0 transition-opacity group-hover:opacity-100">
                        <Button
                          variant="ghost"
                          size="icon"
                          className="h-6 w-6"
                          onClick={() => setDeleteId(activity.id)}
                        >
                          <Trash2 className="h-3 w-3" />
                        </Button>
                      </div>
                    )}
                  </div>
                  {activity.subject && <p className="mt-0.5 text-sm">{activity.subject}</p>}
                  {activity.body && (
                    <p className="mt-1 line-clamp-2 text-sm text-muted-foreground">{activity.body}</p>
                  )}
                </div>
              </div>
            );
          })}
        </div>
      )}

      <ConfirmDialog
        open={!!deleteId}
        onOpenChange={(open) => !open && setDeleteId(null)}
        title="Delete activity"
        description="Are you sure you want to delete this activity?"
        confirmLabel="Delete"
        variant="destructive"
        onConfirm={handleDeleteConfirm}
      />
    </div>
  );
}
