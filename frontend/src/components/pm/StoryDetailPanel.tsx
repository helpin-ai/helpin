import { useEffect, useState } from 'react';
import { format, parseISO } from 'date-fns';
import { Archive, Loader2, MessageSquare } from 'lucide-react';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { Sheet, SheetContent, SheetHeader, SheetTitle } from '@/components/ui/sheet';
import { Switch } from '@/components/ui/switch';
import { Textarea } from '@/components/ui/textarea';
import { Badge } from '@/components/ui/badge';
import { pmCommentService } from '@/lib/services/pmCommentService';
import { pmStoryService } from '@/lib/services/pmStoryService';
import type {
  ActivityLogEntry,
  CommentWithAuthor,
  Priority,
  Severity,
  StoryDetail,
  UpdateStoryRequest,
  WorkflowState,
} from '@/lib/pmTypes';

interface StoryDetailPanelProps {
  workspaceId: string;
  open: boolean;
  onOpenChange: (open: boolean) => void;
  storyDetail: StoryDetail | null;
  states: WorkflowState[];
  onStoryUpdated: (story: StoryDetail) => void;
  onStoryArchived: (storyId: string) => void;
}

interface FormState {
  name: string;
  description: string;
  workflow_state_id: string;
  priority: Priority;
  severity: Severity;
  estimate: string;
  deadline: string;
  epic_id: string;
  iteration_id: string;
  blocked: boolean;
  blocker: string;
}

const priorityOptions: Priority[] = ['none', 'low', 'medium', 'high', 'urgent'];
const severityOptions: Severity[] = ['none', 'minor', 'major', 'critical'];

const buildFormState = (story: StoryDetail): FormState => ({
  name: story.story.name,
  description: story.story.description ?? '',
  workflow_state_id: story.story.workflow_state_id,
  priority: story.story.priority,
  severity: story.story.severity,
  estimate: story.story.estimate === undefined || story.story.estimate === null ? '' : String(story.story.estimate),
  deadline: story.story.deadline ? story.story.deadline.slice(0, 10) : '',
  epic_id: story.story.epic_id ?? '',
  iteration_id: story.story.iteration_id ?? '',
  blocked: story.story.blocked,
  blocker: story.story.blocker ?? '',
});

function StoryDetailPanelBody({
  workspaceId,
  storyDetail,
  states,
  onOpenChange,
  onStoryUpdated,
  onStoryArchived,
}: {
  workspaceId: string;
  storyDetail: StoryDetail;
  states: WorkflowState[];
  onOpenChange: (open: boolean) => void;
  onStoryUpdated: (story: StoryDetail) => void;
  onStoryArchived: (storyId: string) => void;
}) {
  const [form, setForm] = useState<FormState>(() => buildFormState(storyDetail));
  const [pendingPatch, setPendingPatch] = useState<UpdateStoryRequest>({});
  const [saveError, setSaveError] = useState<string | null>(null);
  const [saving, setSaving] = useState(false);

  const [comments, setComments] = useState<CommentWithAuthor[]>([]);
  const [newComment, setNewComment] = useState('');
  const [commentLoading, setCommentLoading] = useState(false);

  const [activity, setActivity] = useState<ActivityLogEntry[]>([]);

  useEffect(() => {
    const url = new URL(window.location.href);
    url.searchParams.set('story', `TP-${storyDetail.story.display_id}`);
    window.history.replaceState({}, '', url.toString());

    return () => {
      const cleanupUrl = new URL(window.location.href);
      cleanupUrl.searchParams.delete('story');
      window.history.replaceState({}, '', cleanupUrl.toString());
    };
  }, [storyDetail]);

  useEffect(() => {
    (async () => {
      const [commentsRes, activityRes] = await Promise.all([
        pmCommentService.list(workspaceId, 'story', storyDetail.story.id),
        pmStoryService.listActivity(workspaceId, storyDetail.story.id, 1, 30),
      ]);
      setComments(commentsRes.data ?? []);
      setActivity(activityRes.data?.data ?? []);
    })();
  }, [workspaceId, storyDetail]);

  useEffect(() => {
    if (saving || Object.keys(pendingPatch).length === 0) return;
    const timer = window.setTimeout(async () => {
      const patch = pendingPatch;
      setPendingPatch({});
      setSaving(true);
      const { data, error } = await pmStoryService.update(workspaceId, storyDetail.story.id, patch);
      if (error || !data) {
        setSaveError(error ?? 'Failed to save changes');
        setPendingPatch((current) => ({ ...patch, ...current }));
      } else {
        setSaveError(null);
        onStoryUpdated(data);
      }
      setSaving(false);
    }, 650);

    return () => window.clearTimeout(timer);
  }, [workspaceId, storyDetail, pendingPatch, saving, onStoryUpdated]);

  const queuePatch = (patch: UpdateStoryRequest) => {
    setPendingPatch((current) => ({ ...current, ...patch }));
  };

  const updateField = <K extends keyof FormState>(key: K, value: FormState[K], patch: UpdateStoryRequest) => {
    setForm((current) => ({ ...current, [key]: value }));
    queuePatch(patch);
  };

  const addComment = async () => {
    if (!newComment.trim()) return;
    setCommentLoading(true);
    const { data, error } = await pmCommentService.create(workspaceId, {
      entity_type: 'story',
      entity_id: storyDetail.story.id,
      body: newComment.trim(),
    });
    setCommentLoading(false);
    if (error || !data) return;
    setComments((current) => [...current, data]);
    setNewComment('');
  };

  const archiveStory = async () => {
    const { error } = await pmStoryService.remove(workspaceId, storyDetail.story.id);
    if (error) {
      setSaveError(error);
      return;
    }
    onStoryArchived(storyDetail.story.id);
    onOpenChange(false);
  };

  const dueLabel = storyDetail.story.deadline
    ? format(parseISO(storyDetail.story.deadline), 'MMM d, yyyy')
    : null;

  return (
    <div className="flex h-full flex-col">
      <SheetHeader className="border-b border-border/70 px-5 py-4">
        <div className="flex items-center justify-between gap-3">
          <div className="flex items-center gap-2">
            <Badge variant="secondary" className="rounded-full px-2 py-0.5 text-xs">
              TP-{storyDetail.story.display_id}
            </Badge>
            <Badge variant="outline" className="rounded-full px-2 py-0.5 text-xs">
              {storyDetail.story.story_type}
            </Badge>
            {dueLabel ? <Badge variant="outline">Due {dueLabel}</Badge> : null}
          </div>
          <Button variant="outline" size="sm" onClick={archiveStory}>
            <Archive className="h-4 w-4" />
            Archive
          </Button>
        </div>
        <SheetTitle className="text-left text-2xl font-semibold">Story Detail</SheetTitle>
      </SheetHeader>

      <div className="grid min-h-0 flex-1 grid-cols-1 gap-0 overflow-hidden lg:grid-cols-[1fr_320px]">
        <div className="min-h-0 overflow-y-auto border-r border-border/60 p-5">
          <div className="space-y-4">
            <div className="space-y-2">
              <Label>Title</Label>
              <Input
                value={form.name}
                onChange={(event) =>
                  updateField('name', event.target.value, { name: event.target.value })
                }
              />
            </div>

            <div className="space-y-2">
              <Label>Description</Label>
              <Textarea
                value={form.description}
                rows={8}
                onChange={(event) =>
                  updateField('description', event.target.value, { description: event.target.value })
                }
              />
            </div>

            <div className="grid grid-cols-1 gap-3 md:grid-cols-2">
              <div className="space-y-2">
                <Label>State</Label>
                <Select
                  value={form.workflow_state_id}
                  onValueChange={(value) =>
                    updateField('workflow_state_id', value, { workflow_state_id: value })
                  }
                >
                  <SelectTrigger>
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    {states.map((state) => (
                      <SelectItem key={state.id} value={state.id}>
                        {state.name}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              </div>

              <div className="space-y-2">
                <Label>Priority</Label>
                <Select
                  value={form.priority}
                  onValueChange={(value) =>
                    updateField('priority', value as Priority, { priority: value as Priority })
                  }
                >
                  <SelectTrigger>
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    {priorityOptions.map((option) => (
                      <SelectItem key={option} value={option}>
                        {option}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              </div>

              <div className="space-y-2">
                <Label>Severity</Label>
                <Select
                  value={form.severity}
                  onValueChange={(value) =>
                    updateField('severity', value as Severity, { severity: value as Severity })
                  }
                >
                  <SelectTrigger>
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    {severityOptions.map((option) => (
                      <SelectItem key={option} value={option}>
                        {option}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              </div>

              <div className="space-y-2">
                <Label>Estimate</Label>
                <Input
                  type="number"
                  min={0}
                  value={form.estimate}
                  onChange={(event) => {
                    const next = event.target.value;
                    updateField('estimate', next, {
                      estimate: next === '' ? undefined : Number(next),
                    });
                  }}
                />
              </div>

              <div className="space-y-2">
                <Label>Epic ID</Label>
                <Input
                  value={form.epic_id}
                  placeholder="Optional"
                  onChange={(event) =>
                    updateField('epic_id', event.target.value, {
                      epic_id: event.target.value.trim() || undefined,
                    })
                  }
                />
              </div>

              <div className="space-y-2">
                <Label>Iteration ID</Label>
                <Input
                  value={form.iteration_id}
                  placeholder="Optional"
                  onChange={(event) =>
                    updateField('iteration_id', event.target.value, {
                      iteration_id: event.target.value.trim() || undefined,
                    })
                  }
                />
              </div>

              <div className="space-y-2">
                <Label>Due date</Label>
                <Input
                  type="date"
                  value={form.deadline}
                  onChange={(event) =>
                    updateField('deadline', event.target.value, {
                      deadline: event.target.value || undefined,
                    })
                  }
                />
              </div>

              <div className="space-y-2">
                <Label>Blocked</Label>
                <div className="flex h-10 items-center gap-3 rounded-md border border-input bg-background px-3">
                  <Switch
                    checked={form.blocked}
                    onCheckedChange={(checked) =>
                      updateField('blocked', checked, {
                        blocked: checked,
                      })
                    }
                  />
                  <span className="text-sm text-muted-foreground">
                    {form.blocked ? 'Blocked' : 'Not blocked'}
                  </span>
                </div>
              </div>
            </div>

            <div className="space-y-2">
              <Label>Blocker</Label>
              <Input
                value={form.blocker}
                placeholder="Why is this blocked?"
                onChange={(event) =>
                  updateField('blocker', event.target.value, {
                    blocker: event.target.value.trim() || undefined,
                    blocked: event.target.value.trim().length > 0 ? true : form.blocked,
                  })
                }
              />
            </div>
          </div>

          <section className="mt-8 space-y-3">
            <div className="flex items-center gap-2">
              <MessageSquare className="h-4 w-4" />
              <h3 className="text-sm font-semibold">Comments</h3>
            </div>
            <div className="space-y-2">
              {comments.length === 0 ? (
                <p className="text-sm text-muted-foreground">No comments yet.</p>
              ) : (
                comments.map((entry) => (
                  <article key={entry.comment.id} className="rounded-md border border-border/70 px-3 py-2">
                    <p className="text-xs text-muted-foreground">
                      {entry.author.full_name || entry.author.email}
                    </p>
                    <p className="mt-1 text-sm">{entry.comment.body}</p>
                  </article>
                ))
              )}
            </div>
            <div className="flex gap-2">
              <Input
                value={newComment}
                placeholder="Write a comment..."
                onChange={(event) => setNewComment(event.target.value)}
              />
              <Button onClick={addComment} disabled={commentLoading || !newComment.trim()}>
                {commentLoading ? 'Posting...' : 'Post'}
              </Button>
            </div>
          </section>
        </div>

        <aside className="min-h-0 overflow-y-auto p-5">
          <h3 className="text-sm font-semibold">Activity</h3>
          <div className="mt-3 space-y-2">
            {activity.length === 0 ? (
              <p className="text-sm text-muted-foreground">No activity yet.</p>
            ) : (
              activity.map((entry) => (
                <article key={entry.activity.id} className="rounded-md border border-border/70 px-3 py-2 text-sm">
                  <p className="font-medium">
                    {entry.actor?.full_name || entry.actor?.email || 'System'}
                  </p>
                  <p className="text-muted-foreground">{entry.activity.action}</p>
                  <p className="text-xs text-muted-foreground">
                    {format(parseISO(entry.activity.created_at), 'MMM d, HH:mm')}
                  </p>
                </article>
              ))
            )}
          </div>
        </aside>
      </div>

      <div className="border-t border-border/70 px-5 py-2.5 text-xs text-muted-foreground">
        {saving ? (
          <span className="inline-flex items-center gap-1">
            <Loader2 className="h-3 w-3 animate-spin" />
            Saving changes...
          </span>
        ) : (
          <span>All changes are autosaved.</span>
        )}
        {saveError ? <span className="ml-3 text-destructive">{saveError}</span> : null}
      </div>
    </div>
  );
}

export function StoryDetailPanel({
  workspaceId,
  open,
  onOpenChange,
  storyDetail,
  states,
  onStoryUpdated,
  onStoryArchived,
}: StoryDetailPanelProps) {
  return (
    <Sheet open={open} onOpenChange={onOpenChange}>
      <SheetContent side="right" className="w-[95vw] p-0 sm:max-w-[900px]">
        {storyDetail ? (
          <StoryDetailPanelBody
            key={storyDetail.story.id}
            workspaceId={workspaceId}
            storyDetail={storyDetail}
            states={states}
            onOpenChange={onOpenChange}
            onStoryUpdated={onStoryUpdated}
            onStoryArchived={onStoryArchived}
          />
        ) : null}
      </SheetContent>
    </Sheet>
  );
}
