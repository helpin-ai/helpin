import { useId, useState } from 'react';
import { formatDistanceToNow } from 'date-fns';
import {
  Briefcase01Icon,
  Calendar01Icon,
  CheckListIcon,
  Delete01Icon,
  DollarCircleIcon,
  PencilEdit01Icon,
  Loading01Icon,
  Mail01Icon,
  Message01Icon,
  SparklesIcon,
  TelephoneIcon,
} from '@/lib/icons';
import { toast } from 'sonner';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Textarea } from '@/components/ui/textarea';
import { Tabs } from '@/components/ui/tabs';
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog';
import { ConfirmDialog } from '@/components/pm/ConfirmDialog';
import { getOptionalSectionActionClass } from '@/components/pm/optionalSectionActionPill';
import { useCreateCRMActivity, useDeleteCRMActivity, useUpdateCRMActivity } from '@/hooks/queries/useCRM';
import type {
  CRMActivity,
  CRMActivityType,
  CRMCompanyTimelineFilter,
  CRMCompanyTimelineItem,
} from '@/lib/crmTypes';
import { cn } from '@/lib/utils';

interface ActivityTimelineProps {
  activities?: CRMActivity[];
  timelineItems?: CRMCompanyTimelineItem[];
  timelineFilter?: CRMCompanyTimelineFilter;
  onTimelineFilterChange?: (filter: CRMCompanyTimelineFilter) => void;
  onTimelineItemOpen?: (item: CRMCompanyTimelineItem) => void;
  hasNextPage?: boolean;
  isFetchingNextPage?: boolean;
  isTimelineLoading?: boolean;
  onLoadMore?: () => void;
  workspaceId?: string;
  contactId?: string;
  companyId?: string;
  dealId?: string;
  onActivityCreated?: () => void;
  onActivityDeleted?: () => void;
  presentation?: 'default' | 'borderless';
}

const activityIcons: Record<CRMActivityType, typeof Mail01Icon> = {
  note: Message01Icon,
  call: TelephoneIcon,
  meeting: Calendar01Icon,
  email: Mail01Icon,
};

const activityTypes: { type: CRMActivityType; icon: typeof Message01Icon; label: string }[] = [
  { type: 'note', icon: Message01Icon, label: 'Note' },
  { type: 'call', icon: TelephoneIcon, label: 'Call' },
  { type: 'meeting', icon: Calendar01Icon, label: 'Meeting' },
];

const filterOptions = ['all', 'note', 'call', 'meeting', 'email'] as const;

const companyFilterOptions: { value: CRMCompanyTimelineFilter; label: string }[] = [
  { value: 'all', label: 'All' },
  { value: 'note', label: 'Notes' },
  { value: 'email', label: 'Emails' },
  { value: 'call', label: 'Calls' },
  { value: 'meeting', label: 'Meetings' },
  { value: 'task', label: 'Tasks' },
];

const timelineIcons: Record<CRMCompanyTimelineItem['kind'], typeof Message01Icon> = {
  note: Message01Icon,
  email: Mail01Icon,
  call: TelephoneIcon,
  meeting: Calendar01Icon,
  task: CheckListIcon,
  deal: DollarCircleIcon,
  support: Briefcase01Icon,
  enrichment: SparklesIcon,
};

export function ActivityTimeline({
  activities = [],
  timelineItems,
  timelineFilter,
  onTimelineFilterChange,
  onTimelineItemOpen,
  hasNextPage,
  isFetchingNextPage,
  isTimelineLoading,
  onLoadMore,
  workspaceId,
  contactId,
  companyId,
  dealId,
  onActivityCreated,
  onActivityDeleted,
  presentation = 'default',
}: ActivityTimelineProps) {
  const borderless = presentation === 'borderless';
  const activityFormId = useId();
  const [filterType, setFilterType] = useState<CRMCompanyTimelineFilter>('all');
  const [activityDialogOpen, setActivityDialogOpen] = useState(false);
  const [draftType, setDraftType] = useState<CRMActivityType>('note');
  const [newSubject, setNewSubject] = useState('');
  const [newBody, setNewBody] = useState('');
  const [deleteId, setDeleteId] = useState<string | null>(null);
  const [editingSourceId, setEditingSourceId] = useState<string | null>(null);

  const createActivity = useCreateCRMActivity(workspaceId ?? '');
  const updateActivity = useUpdateCRMActivity(workspaceId ?? '');
  const deleteActivity = useDeleteCRMActivity(workspaceId ?? '');

  const selectedFilter = timelineItems ? (timelineFilter ?? filterType) : filterType;
  const filtered = filterType === 'all'
    ? activities
    : filterType === 'task'
      ? []
      : activities.filter((a) => a.activity_type === filterType);

  const handleCreate = async () => {
    if (!workspaceId || !newSubject.trim()) return;
    try {
      if (editingSourceId) {
        await updateActivity.mutateAsync({
          id: editingSourceId,
          activity_type: draftType,
          subject: newSubject.trim(),
          body: newBody.trim() || undefined,
        });
        toast.success('Activity updated');
      } else {
        await createActivity.mutateAsync({
          workspace_id: workspaceId,
          activity_type: draftType,
          contact_id: contactId,
          company_id: companyId,
          deal_id: dealId,
          subject: newSubject.trim(),
          body: newBody.trim() || undefined,
        });
        toast.success('Activity logged');
      }
      setActivityDialogOpen(false);
      setEditingSourceId(null);
      setNewSubject('');
      setNewBody('');
      onActivityCreated?.();
    } catch {
      toast.error('Failed to log activity');
    }
  };

  const clearDraft = () => {
    setNewSubject('');
    setNewBody('');
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

  const beginTimelineEdit = (item: CRMCompanyTimelineItem) => {
    setEditingSourceId(item.source_id);
    setDraftType(item.kind as CRMActivityType);
    setNewSubject(item.title);
    setNewBody(item.description ?? '');
    setActivityDialogOpen(true);
  };

  const creationButtons = workspaceId ? (
    <div className={cn('flex flex-wrap items-center gap-[18px]', !borderless && 'mb-4')}>
      {activityTypes.map(({ type, icon: Icon, label }) => (
        <button
          key={type}
          type="button"
          className={getOptionalSectionActionClass(
            activityDialogOpen && draftType === type ? 'open' : 'available',
            'borderless',
          )}
          onClick={() => {
            setEditingSourceId(null);
            setDraftType(type);
            setActivityDialogOpen(true);
          }}
        >
          <Icon className="h-[15px] w-[15px]" />
          {label}
        </button>
      ))}
    </div>
  ) : null;

  return (
    <div>
      {borderless ? (
        <div className="flex flex-wrap items-center justify-between gap-3 px-4 pb-3 pt-5 sm:px-6 lg:px-10">
          <h3 className="text-xs font-semibold uppercase tracking-wide text-foreground/75">Activity</h3>
          {creationButtons}
        </div>
      ) : creationButtons}

      <Tabs
        value={selectedFilter}
        onValueChange={(value) => setFilterType(value as CRMCompanyTimelineFilter)}
        className="gap-0"
      >
        <div
          role="tablist"
          aria-label="Activity type"
          className={cn(
            'flex items-center gap-0.5 overflow-x-auto border-b border-border/60',
            borderless ? 'px-4 sm:px-6 lg:px-10' : 'mb-3',
          )}
        >
          {(timelineItems
            ? companyFilterOptions
            : filterOptions.map((value) => ({
                value,
                label: value === 'all' ? 'All' : `${value.charAt(0).toUpperCase()}${value.slice(1)}s`,
              }))).map(({ value, label }) => (
            <button
              key={value}
              type="button"
              role="tab"
              aria-selected={selectedFilter === value}
              className={cn(
                '-mb-px inline-flex items-center gap-1 whitespace-nowrap border-b-2 px-2.5 py-1.5 text-[13px] font-medium transition-colors',
                selectedFilter === value
                  ? 'border-primary text-foreground'
                  : 'border-transparent text-muted-foreground hover:text-foreground',
              )}
              onClick={() => {
                setFilterType(value);
                onTimelineFilterChange?.(value);
              }}
            >
              {label}
            </button>
          ))}
        </div>
      </Tabs>

      {isTimelineLoading ? (
        <div className="flex items-center justify-center px-6 py-12">
          <Loading01Icon className="h-5 w-5 animate-spin text-muted-foreground" />
        </div>
      ) : (timelineItems ?? filtered).length === 0 ? (
        <div className={cn('flex flex-col items-center justify-center py-8 text-center', borderless && 'px-6 py-11')}>
          <div className={cn('flex h-12 w-12 items-center justify-center', !borderless && 'rounded-full bg-muted')}>
            <Message01Icon className={cn('h-6 w-6 text-muted-foreground/50', borderless && 'h-5 w-5')} />
          </div>
          <p className="mt-3 text-sm font-medium">No activities yet</p>
          <p className="mt-1 text-xs text-muted-foreground">Log your first activity to track interactions</p>
        </div>
      ) : (
        <div className={cn(borderless ? 'divide-y divide-border/50' : 'space-y-4')}>
          {timelineItems ? timelineItems.map((item) => {
            const Icon = timelineIcons[item.kind] ?? Message01Icon;
            const context = [item.actor?.name, item.contact?.name].filter(Boolean).join(' · ');
            const interactive = !item.can_edit && !!onTimelineItemOpen;
            return (
              <div
                key={item.id}
                className={cn(
                  'group flex gap-3 px-4 py-4 sm:px-6 lg:px-10',
                  interactive && 'cursor-pointer transition-colors hover:bg-muted/25',
                )}
                role={interactive ? 'button' : undefined}
                tabIndex={interactive ? 0 : undefined}
                onClick={() => interactive && onTimelineItemOpen?.(item)}
                onKeyDown={(event) => {
                  if (interactive && (event.key === 'Enter' || event.key === ' ')) onTimelineItemOpen?.(item);
                }}
              >
                <div className="flex h-7 w-7 shrink-0 items-center justify-center rounded-full bg-muted">
                  <Icon className="h-3.5 w-3.5 text-muted-foreground" />
                </div>
                <div className="min-w-0 flex-1">
                  <div className="flex min-w-0 items-center gap-2">
                    <span className="truncate text-sm font-medium">{item.title}</span>
                    <span className="shrink-0 text-xs text-muted-foreground">
                      {formatDistanceToNow(new Date(item.occurred_at), { addSuffix: true })}
                    </span>
                    {(item.can_edit || item.can_delete) && (
                      <div className="ml-auto flex gap-1 opacity-0 transition-opacity group-hover:opacity-100 group-focus-within:opacity-100">
                        {item.can_edit && (
                          <Button variant="ghost" size="icon" className="h-6 w-6" aria-label="Edit activity" onClick={(event) => { event.stopPropagation(); beginTimelineEdit(item); }}>
                            <PencilEdit01Icon className="h-3 w-3" />
                          </Button>
                        )}
                        {item.can_delete && (
                          <Button variant="ghost" size="icon" className="h-6 w-6" aria-label="Delete activity" onClick={(event) => { event.stopPropagation(); setDeleteId(item.source_id); }}>
                            <Delete01Icon className="h-3 w-3" />
                          </Button>
                        )}
                      </div>
                    )}
                  </div>
                  {item.description && <p className="mt-1 line-clamp-3 text-sm text-muted-foreground">{item.description}</p>}
                  {(context || item.entity?.display_id) && (
                    <p className="mt-1.5 text-xs text-muted-foreground/75">
                      {[context, item.entity?.display_id].filter(Boolean).join(' · ')}
                    </p>
                  )}
                </div>
              </div>
            );
          }) : filtered.map((activity) => {
            const Icon = activityIcons[activity.activity_type] ?? Message01Icon;
            return (
              <div key={activity.id} className={cn(
                'group flex gap-3',
                borderless && 'px-4 py-4 sm:px-6 lg:px-10',
              )}>
                <div className={cn(
                  'flex h-8 w-8 shrink-0 items-center justify-center rounded-full bg-muted',
                  borderless && 'h-7 w-7',
                )}>
                  <Icon className={cn('h-4 w-4 text-muted-foreground', borderless && 'h-3.5 w-3.5')} />
                </div>
                <div className="min-w-0 flex-1">
                  <div className="flex items-center gap-2">
                    <span className="text-sm font-medium capitalize">{activity.activity_type}</span>
                    <span className="text-xs text-muted-foreground">
                      {formatDistanceToNow(new Date(activity.occurred_at), { addSuffix: true })}
                    </span>
                    {workspaceId && (
                      <div className="ml-auto flex gap-1 opacity-0 transition-opacity group-hover:opacity-100 group-focus-within:opacity-100">
                        <Button
                          variant="ghost"
                          size="icon"
                          className="h-6 w-6"
                          onClick={() => setDeleteId(activity.id)}
                        >
                          <Delete01Icon className="h-3 w-3" />
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
          {timelineItems && hasNextPage && (
            <div className="flex justify-center px-6 py-4">
              <Button variant="outline" size="sm" disabled={isFetchingNextPage} onClick={onLoadMore}>
                {isFetchingNextPage && <Loading01Icon className="mr-1.5 h-3.5 w-3.5 animate-spin" />}
                Load more
              </Button>
            </div>
          )}
        </div>
      )}

      <Dialog open={activityDialogOpen} onOpenChange={setActivityDialogOpen}>
        <DialogContent className="sm:max-w-lg">
          <DialogHeader>
            <DialogTitle>{editingSourceId ? 'Edit activity' : 'Log activity'}</DialogTitle>
            <DialogDescription>
              {editingSourceId
                ? 'Update this manual activity.'
                : 'Add a note, call, or meeting to this record. Closing this dialog keeps your draft.'}
            </DialogDescription>
          </DialogHeader>

          <form
            className="space-y-5"
            onSubmit={(event) => {
              event.preventDefault();
              void handleCreate();
            }}
          >
            <fieldset className="space-y-2">
              <legend className="text-xs font-medium text-muted-foreground">Activity type</legend>
              <div className="grid grid-cols-3 gap-1 rounded-lg bg-muted/60 p-1" role="radiogroup" aria-label="Activity type">
                {activityTypes.map(({ type, icon: Icon, label }) => (
                  <Button
                    key={type}
                    type="button"
                    role="radio"
                    aria-checked={draftType === type}
                    variant="ghost"
                    size="sm"
                    className={cn(
                      'gap-1.5 text-muted-foreground hover:text-foreground',
                      draftType === type && 'bg-background text-foreground shadow-xs hover:bg-background',
                    )}
                    onClick={() => setDraftType(type)}
                  >
                    <Icon className="h-3.5 w-3.5" />
                    {label}
                  </Button>
                ))}
              </div>
            </fieldset>

            <div className="space-y-2">
              <label htmlFor={`${activityFormId}-subject`} className="text-xs font-medium text-muted-foreground">
                Subject
              </label>
              <Input
                id={`${activityFormId}-subject`}
                autoFocus
                placeholder="What happened?"
                value={newSubject}
                onChange={(event) => setNewSubject(event.target.value)}
              />
            </div>

            <div className="space-y-2">
              <label htmlFor={`${activityFormId}-details`} className="text-xs font-medium text-muted-foreground">
                Details <span className="font-normal text-muted-foreground/70">(optional)</span>
              </label>
              <Textarea
                id={`${activityFormId}-details`}
                placeholder="Add context, outcomes, or next steps..."
                value={newBody}
                onChange={(event) => setNewBody(event.target.value)}
                className="min-h-28 resize-y"
                rows={4}
              />
            </div>

            <DialogFooter className="gap-3 sm:items-center sm:justify-between">
              <Button
                type="button"
                variant="ghost"
                className="text-muted-foreground"
                disabled={!newSubject && !newBody}
                onClick={clearDraft}
              >
                Clear draft
              </Button>
              <div className="flex flex-col-reverse gap-2 sm:flex-row">
                <Button type="button" variant="outline" onClick={() => setActivityDialogOpen(false)}>
                  Close
                </Button>
                <Button type="submit" disabled={!newSubject.trim() || createActivity.isPending || updateActivity.isPending}>
                  {createActivity.isPending || updateActivity.isPending ? 'Saving...' : editingSourceId ? 'Save changes' : `Save ${draftType}`}
                </Button>
              </div>
            </DialogFooter>
          </form>
        </DialogContent>
      </Dialog>

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
