import { useEffect, useMemo, useState } from 'react';
import { Plus } from 'lucide-react';
import { Button } from '@/components/ui/button';
import { Dialog, DialogContent, DialogHeader, DialogTitle } from '@/components/ui/dialog';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Textarea } from '@/components/ui/textarea';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import type { CreateStoryRequest, IterationWithStats, Priority, StoryType, WorkflowWithStates, EpicWithStats } from '@/lib/pmTypes';
import { pmEpicService } from '@/lib/services/pmEpicService';
import { pmIterationService } from '@/lib/services/pmIterationService';

interface CreateStoryModalProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  workspaceId: string;
  workflow: WorkflowWithStates;
  initialStateId: string;
  onCreate: (payload: CreateStoryRequest) => Promise<void>;
}

const priorityOptions: Priority[] = ['none', 'low', 'medium', 'high', 'urgent'];
const storyTypeOptions: StoryType[] = ['feature', 'bug', 'chore'];

const defaultState = {
  name: '',
  story_type: 'feature' as StoryType,
  description: '',
  priority: 'none' as Priority,
  estimate: '',
  epic_id: '',
  iteration_id: '',
  deadline: '',
};

export function CreateStoryModal({
  open,
  onOpenChange,
  workspaceId,
  workflow,
  initialStateId,
  onCreate,
}: CreateStoryModalProps) {
  const [form, setForm] = useState(defaultState);
  const [stateId, setStateId] = useState(initialStateId);
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [epics, setEpics] = useState<EpicWithStats[]>([]);
  const [iterations, setIterations] = useState<IterationWithStats[]>([]);

  useEffect(() => {
    if (!open) return;
    setForm(defaultState);
    setStateId(initialStateId);
    setError(null);
  }, [open, initialStateId]);

  useEffect(() => {
    if (!open) return;
    (async () => {
      const [epicsRes, iterationsRes] = await Promise.all([
        pmEpicService.list(workspaceId, { archived: false }),
        pmIterationService.list(workspaceId, { archived: false }),
      ]);
      setEpics(epicsRes.data ?? []);
      setIterations(iterationsRes.data ?? []);
    })();
  }, [open, workspaceId]);

  const canSubmit = useMemo(
    () => form.name.trim().length > 0 && stateId.trim().length > 0,
    [form.name, stateId]
  );

  const submit = async () => {
    if (!canSubmit || submitting) return;
    setSubmitting(true);
    setError(null);
    try {
      await onCreate({
        workspace_id: workspaceId,
        name: form.name.trim(),
        description: form.description.trim() || undefined,
        story_type: form.story_type,
        workflow_id: workflow.workflow.id,
        workflow_state_id: stateId,
        priority: form.priority,
        estimate: form.estimate ? Number(form.estimate) : undefined,
        epic_id: form.epic_id || undefined,
        iteration_id: form.iteration_id || undefined,
        deadline: form.deadline || undefined,
      });
      onOpenChange(false);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to create story');
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-w-2xl">
        <DialogHeader>
          <DialogTitle>Create Story</DialogTitle>
        </DialogHeader>

        <div className="space-y-4">
          <div className="space-y-2">
            <Label htmlFor="story-title">Title</Label>
            <Input
              id="story-title"
              placeholder="What needs to be done?"
              value={form.name}
              onChange={(event) => setForm((prev) => ({ ...prev, name: event.target.value }))}
            />
          </div>

          <div className="space-y-2">
            <Label htmlFor="story-description">Description</Label>
            <Textarea
              id="story-description"
              placeholder="Details, acceptance criteria, links..."
              value={form.description}
              onChange={(event) => setForm((prev) => ({ ...prev, description: event.target.value }))}
            />
          </div>

          <div className="grid grid-cols-1 gap-3 md:grid-cols-2">
            <div className="space-y-2">
              <Label>Story Type</Label>
              <Select
                value={form.story_type}
                onValueChange={(value) => setForm((prev) => ({ ...prev, story_type: value as StoryType }))}
              >
                <SelectTrigger>
                  <SelectValue placeholder="Select type" />
                </SelectTrigger>
                <SelectContent>
                  {storyTypeOptions.map((option) => (
                    <SelectItem key={option} value={option}>
                      {option}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>

            <div className="space-y-2">
              <Label>State</Label>
              <Select value={stateId} onValueChange={setStateId}>
                <SelectTrigger>
                  <SelectValue placeholder="Select state" />
                </SelectTrigger>
                <SelectContent>
                  {workflow.states.map((state) => (
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
                onValueChange={(value) => setForm((prev) => ({ ...prev, priority: value as Priority }))}
              >
                <SelectTrigger>
                  <SelectValue placeholder="Select priority" />
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
              <Label htmlFor="story-estimate">Estimate</Label>
              <Input
                id="story-estimate"
                type="number"
                min={0}
                placeholder="Story points"
                value={form.estimate}
                onChange={(event) => setForm((prev) => ({ ...prev, estimate: event.target.value }))}
              />
            </div>

            <div className="space-y-2">
              <Label>Epic</Label>
              <Select
                value={form.epic_id || '__none__'}
                onValueChange={(value) => setForm((prev) => ({ ...prev, epic_id: value === '__none__' ? '' : value }))}
              >
                <SelectTrigger>
                  <SelectValue placeholder="No epic" />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="__none__">No epic</SelectItem>
                  {epics.map((epic) => (
                    <SelectItem key={epic.epic.id} value={epic.epic.id}>
                      {epic.epic.name}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>

            <div className="space-y-2">
              <Label>Iteration</Label>
              <Select
                value={form.iteration_id || '__none__'}
                onValueChange={(value) => setForm((prev) => ({ ...prev, iteration_id: value === '__none__' ? '' : value }))}
              >
                <SelectTrigger>
                  <SelectValue placeholder="No iteration" />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="__none__">No iteration</SelectItem>
                  {iterations.map((iteration) => (
                    <SelectItem key={iteration.iteration.id} value={iteration.iteration.id}>
                      {iteration.iteration.name}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>

            <div className="space-y-2">
              <Label htmlFor="story-deadline">Due date</Label>
              <Input
                id="story-deadline"
                type="date"
                value={form.deadline}
                onChange={(event) => setForm((prev) => ({ ...prev, deadline: event.target.value }))}
              />
            </div>
          </div>

          {error ? <p className="text-sm text-destructive">{error}</p> : null}

          <div className="flex items-center justify-end gap-2">
            <Button variant="outline" onClick={() => onOpenChange(false)} disabled={submitting}>
              Cancel
            </Button>
            <Button onClick={submit} disabled={!canSubmit || submitting}>
              <Plus className="h-4 w-4" />
              {submitting ? 'Creating...' : 'Create story'}
            </Button>
          </div>
        </div>
      </DialogContent>
    </Dialog>
  );
}
