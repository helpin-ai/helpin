import { useId, useLayoutEffect, useRef, useState } from 'react';
import { formatDistanceToNow } from 'date-fns';
import { Briefcase01Icon, Calendar01Icon, CheckListIcon, Delete01Icon, DollarCircleIcon, PencilEdit01Icon, Loading01Icon, Mail01Icon, Message01Icon, SparklesIcon, TelephoneIcon } from '@/lib/icons';
import { toast } from 'sonner';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Textarea } from '@/components/ui/textarea';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog';
import { ConfirmDialog } from '@/components/pm/ConfirmDialog';
import { EmphasizedActivityLabel, UpdateActivityRow } from '@/components/pm/UpdateActivityRow';
import { compactUpdateTime } from '@/components/pm/updateActivityTime';
import { MarkdownContent } from '@/components/pm/CodingSession/MarkdownContent';
import { getOptionalSectionActionClass } from '@/components/pm/optionalSectionActionPill';
import { useCreateCRMActivity, useDeleteCRMActivity, useEmailAccounts, useUpdateCRMActivity } from '@/hooks/queries/useCRM';
import type { CRMActivity, CRMActivityType, CRMTimelineFilter, CRMTimelineItem } from '@/lib/crmTypes';
import { cn } from '@/lib/utils';
import { crmTimelinePresentation, dedupeCRMTimelineItems, type CRMTimelinePresentation } from './companyTimelinePresentation';
import { CRMEmailComposerDialog } from './CRMEmailComposerDialog';

interface ActivityTimelineProps {
  activities?: CRMActivity[];
  timelineItems?: CRMTimelineItem[];
  timelineFilter?: CRMTimelineFilter;
  onTimelineFilterChange?: (filter: CRMTimelineFilter) => void;
  onTimelineItemOpen?: (item: CRMTimelineItem) => void;
  hasNextPage?: boolean;
  isFetchingNextPage?: boolean;
  isTimelineLoading?: boolean;
  onLoadMore?: () => void;
  workspaceId?: string;
  contactId?: string;
  companyId?: string;
  dealId?: string;
  emailRecipient?: string;
  onActivityCreated?: () => void;
  onActivityDeleted?: () => void;
  presentation?: 'default' | 'borderless';
  filterControl?: 'tabs' | 'dropdown' | 'hidden';
  actionTypes?: Array<CRMActivityType | 'email'>;
  heading?: string;
}

const activityIcons: Record<CRMActivityType, typeof Mail01Icon> = {
  note: Message01Icon,
  call: TelephoneIcon,
  meeting: Calendar01Icon,
  email: Mail01Icon,
};

const activityTypes: {
  type: CRMActivityType;
  icon: typeof Message01Icon;
  label: string;
}[] = [
  { type: 'note', icon: Message01Icon, label: 'Note' },
  { type: 'call', icon: TelephoneIcon, label: 'Call' },
  { type: 'meeting', icon: Calendar01Icon, label: 'Meeting' },
];

const filterOptions = ['all', 'note', 'call', 'meeting', 'email'] as const;

const companyFilterOptions: { value: CRMTimelineFilter; label: string }[] = [
  { value: 'all', label: 'All' },
  { value: 'note', label: 'Notes' },
  { value: 'email', label: 'Emails' },
  { value: 'call', label: 'Calls' },
  { value: 'meeting', label: 'Meetings' },
  { value: 'task', label: 'Tasks' },
  { value: 'deal', label: 'Deals' },
  { value: 'support', label: 'Support' },
];

const dealFilterOptions: { value: CRMTimelineFilter; label: string }[] = [
  { value: 'all', label: 'All' },
  { value: 'note', label: 'Notes' },
  { value: 'email', label: 'Emails' },
  { value: 'call', label: 'Calls' },
  { value: 'meeting', label: 'Meetings' },
  { value: 'task', label: 'Tasks' },
  { value: 'deal', label: 'Deal changes' },
  { value: 'support', label: 'Support' },
];

const timelineIcons: Record<CRMTimelineItem['kind'], typeof Message01Icon> = {
  note: Message01Icon,
  email: Mail01Icon,
  call: TelephoneIcon,
  meeting: Calendar01Icon,
  task: CheckListIcon,
  deal: DollarCircleIcon,
  support: Briefcase01Icon,
  enrichment: SparklesIcon,
};

function ContentTimelineEntry({
  item,
  presentation,
  icon: Icon,
  borderless,
  onOpen,
  onEdit,
  onDelete,
}: {
  item: CRMTimelineItem;
  presentation: CRMTimelinePresentation;
  icon: typeof Message01Icon;
  borderless: boolean;
  onOpen?: () => void;
  onEdit: () => void;
  onDelete: () => void;
}) {
  const [expanded, setExpanded] = useState(false);
  const [canExpand, setCanExpand] = useState(false);
  const contentRef = useRef<HTMLDivElement>(null);
  const isEmailPreview = item.kind === 'email';

  useLayoutEffect(() => {
    const content = contentRef.current;
    if (!content || expanded) return;
    const measure = () => setCanExpand(content.scrollHeight > content.clientHeight + 1);
    measure();
    if (typeof ResizeObserver === 'undefined') return;
    const observer = new ResizeObserver(measure);
    observer.observe(content);
    return () => observer.disconnect();
  }, [expanded, isEmailPreview, presentation.contentBody, presentation.contentTitle]);

  const interactive = Boolean(onOpen);

  return (
    <div className={cn('group grid grid-cols-[1.75rem_minmax(0,1fr)] gap-3 py-3.5', borderless && (isEmailPreview ? 'pl-4 pr-2 sm:pl-6 sm:pr-3 lg:pl-10 lg:pr-4' : 'px-4 sm:px-6 lg:px-10'))}>
      <span className="flex h-7 w-7 shrink-0 items-center justify-center">
        <span className="flex h-7 w-7 items-center justify-center rounded-full bg-muted text-muted-foreground">
          <Icon className="h-3.5 w-3.5" />
        </span>
      </span>

      <div className="min-w-0">
        <div className="flex min-w-0 items-center gap-2">
          {interactive ? (
            <button type="button" className="min-w-0 flex-1 truncate text-left text-[13px] text-foreground/70 transition-colors hover:text-foreground" onClick={onOpen}>
              <EmphasizedActivityLabel label={presentation.label} values={presentation.emphasizedValues} />
            </button>
          ) : (
            <span className="min-w-0 flex-1 truncate text-[13px] text-foreground/70">
              <EmphasizedActivityLabel label={presentation.label} values={presentation.emphasizedValues} />
            </span>
          )}
          {presentation.attribution && (
            <span className="shrink-0 whitespace-nowrap text-xs text-muted-foreground">
              by <span className="font-medium text-foreground/80">{presentation.attribution}</span>
            </span>
          )}
          {(item.can_edit || item.can_delete) && (
            <span className="flex shrink-0 gap-0.5 opacity-0 transition-opacity group-hover:opacity-100 group-focus-within:opacity-100">
              {item.can_edit && (
                <Button
                  variant="ghost"
                  size="icon"
                  className="h-6 w-6"
                  aria-label="Edit activity"
                  onClick={(event) => {
                    event.stopPropagation();
                    onEdit();
                  }}
                >
                  <PencilEdit01Icon className="h-3 w-3" />
                </Button>
              )}
              {item.can_delete && (
                <Button
                  variant="ghost"
                  size="icon"
                  className="h-6 w-6"
                  aria-label="Delete activity"
                  onClick={(event) => {
                    event.stopPropagation();
                    onDelete();
                  }}
                >
                  <Delete01Icon className="h-3 w-3" />
                </Button>
              )}
            </span>
          )}
          <time className="w-16 shrink-0 whitespace-nowrap text-right text-xs text-muted-foreground">{compactUpdateTime(item.occurred_at)}</time>
        </div>

        <div ref={contentRef} className={cn('mt-1.5', !expanded && (isEmailPreview ? 'max-h-10 overflow-hidden' : 'max-h-[6.75rem] overflow-hidden'))}>
          {presentation.contentTitle && <p className="text-sm font-medium leading-5 text-foreground/90">{presentation.contentTitle}</p>}
          {presentation.contentBody && presentation.contentFormat === 'markdown' ? (
            <MarkdownContent content={presentation.contentBody} className={cn('text-[13px] leading-5 text-foreground/75', presentation.contentTitle && 'mt-1')} />
          ) : presentation.contentBody ? (
            <p className={cn('whitespace-pre-wrap text-[13px] leading-5 text-foreground/75', presentation.contentTitle && 'mt-1')}>{presentation.contentBody}</p>
          ) : null}
        </div>

        {canExpand && (
          <button
            type="button"
            aria-expanded={expanded}
            className="mt-1 text-xs font-medium text-muted-foreground transition-colors hover:text-foreground"
            onClick={(event) => {
              event.stopPropagation();
              setExpanded((current) => !current);
            }}
          >
            {expanded ? 'Show less' : 'Show more'}
          </button>
        )}
      </div>
    </div>
  );
}

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
  emailRecipient,
  onActivityCreated,
  onActivityDeleted,
  presentation = 'default',
  filterControl = 'tabs',
  actionTypes,
  heading = 'Activity',
}: ActivityTimelineProps) {
  const borderless = presentation === 'borderless';
  const activityFormId = useId();
  const [filterType, setFilterType] = useState<CRMTimelineFilter>('all');
  const [activityDialogOpen, setActivityDialogOpen] = useState(false);
  const [draftType, setDraftType] = useState<CRMActivityType>('note');
  const [newSubject, setNewSubject] = useState('');
  const [newBody, setNewBody] = useState('');
  const [deleteId, setDeleteId] = useState<string | null>(null);
  const [editingSourceId, setEditingSourceId] = useState<string | null>(null);
  const [emailComposerOpen, setEmailComposerOpen] = useState(false);

  const createActivity = useCreateCRMActivity(workspaceId ?? '');
  const updateActivity = useUpdateCRMActivity(workspaceId ?? '');
  const deleteActivity = useDeleteCRMActivity(workspaceId ?? '');
  const emailEnabled = Boolean(timelineItems && (contactId || companyId || dealId));
  const emailAccounts = useEmailAccounts(emailEnabled ? (workspaceId ?? '') : '');
  const visibleActivityTypes = activityTypes.filter(({ type }) => !actionTypes || actionTypes.includes(type));
  const showEmailAction = emailEnabled && (!actionTypes || actionTypes.includes('email'));

  const selectedFilter = timelineItems ? (timelineFilter ?? filterType) : filterType;
  const filtered = filterType === 'all' ? activities : filterType === 'task' || filterType === 'deal' || filterType === 'support' ? [] : activities.filter((a) => a.activity_type === filterType);
  const displayedTimelineItems = timelineItems ? dedupeCRMTimelineItems(timelineItems) : undefined;
  const timelineFilterOptions = dealId ? dealFilterOptions : companyFilterOptions;

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

  const beginTimelineEdit = (item: CRMTimelineItem) => {
    setEditingSourceId(item.source_id);
    setDraftType(item.kind as CRMActivityType);
    setNewSubject(item.title);
    setNewBody(item.description ?? '');
    setActivityDialogOpen(true);
  };

  const creationButtons = workspaceId ? (
    <div className={cn('flex flex-wrap items-center gap-[18px]', !borderless && 'mb-4')}>
      {visibleActivityTypes.map(({ type, icon: Icon, label }) => (
        <button
          key={type}
          type="button"
          className={getOptionalSectionActionClass(activityDialogOpen && draftType === type ? 'open' : 'available', 'borderless')}
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
      {showEmailAction && (
        <button type="button" className={getOptionalSectionActionClass(emailComposerOpen ? 'open' : 'available', 'borderless')} onClick={() => setEmailComposerOpen(true)}>
          <Mail01Icon className="h-[15px] w-[15px]" />
          Email
        </button>
      )}
    </div>
  ) : null;

  return (
    <div>
      {borderless ? (
        <div className="flex flex-wrap items-center justify-between gap-3 px-4 pb-3 pt-5 sm:px-6 lg:px-10">
          <div className="flex items-center gap-3">
            <h3 className="text-xs font-semibold uppercase tracking-wide text-foreground/75">{heading}</h3>
            {filterControl === 'dropdown' && timelineItems && (
              <Select
                value={selectedFilter}
                onValueChange={(value) => {
                  setFilterType(value as CRMTimelineFilter);
                  onTimelineFilterChange?.(value as CRMTimelineFilter);
                }}
              >
                <SelectTrigger aria-label="Filter activity" className="h-7 w-[148px] border-0 bg-muted/50 px-2.5 text-xs shadow-none">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  {timelineFilterOptions.map((option) => (
                    <SelectItem key={option.value} value={option.value}>
                      {option.label}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            )}
          </div>
          {creationButtons}
        </div>
      ) : (
        creationButtons
      )}

      {filterControl === 'tabs' && (
        <div role="tablist" aria-label="Activity type" className={cn('flex items-center gap-0.5 overflow-x-auto border-b border-border/60', borderless ? 'px-4 sm:px-6 lg:px-10' : 'mb-3')}>
          {(timelineItems
            ? timelineFilterOptions
            : filterOptions.map((value) => ({
                value,
                label: value === 'all' ? 'All' : `${value.charAt(0).toUpperCase()}${value.slice(1)}s`,
              }))
          ).map(({ value, label }) => (
            <button
              key={value}
              type="button"
              role="tab"
              aria-selected={selectedFilter === value}
              className={cn(
                '-mb-px inline-flex items-center gap-1 whitespace-nowrap border-b-2 px-2.5 py-1.5 text-[13px] font-medium transition-colors',
                selectedFilter === value ? 'border-primary text-foreground' : 'border-transparent text-muted-foreground hover:text-foreground',
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
      )}

      {isTimelineLoading ? (
        <div className="flex items-center justify-center px-6 py-12">
          <Loading01Icon className="h-5 w-5 animate-spin text-muted-foreground" />
        </div>
      ) : (displayedTimelineItems ?? filtered).length === 0 ? (
        <div className={cn('flex flex-col items-center justify-center py-8 text-center', borderless && 'px-6 py-11')}>
          <div className={cn('flex h-12 w-12 items-center justify-center', !borderless && 'rounded-full bg-muted')}>
            <Message01Icon className={cn('h-6 w-6 text-muted-foreground/50', borderless && 'h-5 w-5')} />
          </div>
          <p className="mt-3 text-sm font-medium">No activities yet</p>
          <p className="mt-1 text-xs text-muted-foreground">Log your first activity to track interactions</p>
        </div>
      ) : (
        <div className={cn(borderless ? 'divide-y divide-border/50' : 'space-y-4')}>
          {displayedTimelineItems
            ? displayedTimelineItems.map((item) => {
                const Icon = timelineIcons[item.kind] ?? Message01Icon;
                const itemPresentation = crmTimelinePresentation(item);
                const interactive = !item.can_edit && !!onTimelineItemOpen;

                if (itemPresentation.mode === 'content') {
                  return (
                    <ContentTimelineEntry
                      key={item.id}
                      item={item}
                      presentation={itemPresentation}
                      icon={Icon}
                      borderless={borderless}
                      onOpen={interactive ? () => onTimelineItemOpen?.(item) : undefined}
                      onEdit={() => beginTimelineEdit(item)}
                      onDelete={() => setDeleteId(item.source_id)}
                    />
                  );
                }

                return (
                  <div key={item.id} className={cn(borderless && 'px-2 sm:px-4 lg:px-8')}>
                    <UpdateActivityRow
                      label={itemPresentation.label}
                      occurredAt={item.occurred_at}
                      emphasizedValues={itemPresentation.emphasizedValues}
                      fallbackIcon={<Icon className="h-3.5 w-3.5" />}
                      actionLabel={itemPresentation.attribution ? `by ${itemPresentation.attribution}` : undefined}
                      onClick={interactive ? () => onTimelineItemOpen?.(item) : undefined}
                    />
                  </div>
                );
              })
            : filtered.map((activity) => {
                const Icon = activityIcons[activity.activity_type] ?? Message01Icon;
                return (
                  <div key={activity.id} className={cn('group flex gap-3', borderless && 'px-4 py-4 sm:px-6 lg:px-10')}>
                    <div className={cn('flex h-8 w-8 shrink-0 items-center justify-center rounded-full bg-muted', borderless && 'h-7 w-7')}>
                      <Icon className={cn('h-4 w-4 text-muted-foreground', borderless && 'h-3.5 w-3.5')} />
                    </div>
                    <div className="min-w-0 flex-1">
                      <div className="flex items-center gap-2">
                        <span className="text-sm font-medium capitalize">{activity.activity_type}</span>
                        <span className="text-xs text-muted-foreground">
                          {formatDistanceToNow(new Date(activity.occurred_at), {
                            addSuffix: true,
                          })}
                        </span>
                        {workspaceId && (
                          <div className="ml-auto flex gap-1 opacity-0 transition-opacity group-hover:opacity-100 group-focus-within:opacity-100">
                            <Button variant="ghost" size="icon" className="h-6 w-6" onClick={() => setDeleteId(activity.id)}>
                              <Delete01Icon className="h-3 w-3" />
                            </Button>
                          </div>
                        )}
                      </div>
                      {activity.subject && <p className="mt-0.5 text-sm">{activity.subject}</p>}
                      {activity.body && <p className="mt-1 line-clamp-2 text-sm text-muted-foreground">{activity.body}</p>}
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
            <DialogDescription>{editingSourceId ? 'Update this manual activity.' : 'Add a note, call, or meeting to this record. Closing this dialog keeps your draft.'}</DialogDescription>
          </DialogHeader>

          <form
            className="space-y-5"
            onSubmit={(event) => {
              event.preventDefault();
              void handleCreate();
            }}
          >
            {visibleActivityTypes.length > 1 && (
              <fieldset className="space-y-2">
                <legend className="text-xs font-medium text-muted-foreground">Activity type</legend>
                <div className="grid grid-cols-3 gap-1 rounded-lg bg-muted/60 p-1" role="radiogroup" aria-label="Activity type">
                  {visibleActivityTypes.map(({ type, icon: Icon, label }) => (
                    <Button
                      key={type}
                      type="button"
                      role="radio"
                      aria-checked={draftType === type}
                      variant="ghost"
                      size="sm"
                      className={cn('gap-1.5 text-muted-foreground hover:text-foreground', draftType === type && 'bg-background text-foreground shadow-xs hover:bg-background')}
                      onClick={() => setDraftType(type)}
                    >
                      <Icon className="h-3.5 w-3.5" />
                      {label}
                    </Button>
                  ))}
                </div>
              </fieldset>
            )}

            <div className="space-y-2">
              <label htmlFor={`${activityFormId}-subject`} className="text-xs font-medium text-muted-foreground">
                Subject
              </label>
              <Input id={`${activityFormId}-subject`} autoFocus placeholder="What happened?" value={newSubject} onChange={(event) => setNewSubject(event.target.value)} />
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
              <Button type="button" variant="ghost" className="text-muted-foreground" disabled={!newSubject && !newBody} onClick={clearDraft}>
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

      {emailEnabled && (
        <CRMEmailComposerDialog
          workspaceId={workspaceId ?? ''}
          accounts={emailAccounts.data ?? []}
          open={emailComposerOpen}
          draft={{
            title: 'New email',
            to: emailRecipient ? [emailRecipient] : undefined,
          }}
          onOpenChange={setEmailComposerOpen}
        />
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
