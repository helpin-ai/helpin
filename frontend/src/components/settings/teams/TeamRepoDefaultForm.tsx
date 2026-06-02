import { useEffect, useState } from 'react';
import { repositoryDefaultBranchLabel } from '@/lib/branchLabels';
import { RepositoryBranchPicker } from '@/components/git/RepositoryBranchPicker';
import { Button } from '@/components/ui/button';
import { DialogFooter } from '@/components/ui/dialog';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { Switch } from '@/components/ui/switch';
import type { TeamRepoDefault } from '@/lib/types';
import type { GitRepository, WorkflowState } from '@/lib/pmTypes';

export function TeamRepoDefaultForm({
  workspaceId,
  initial,
  repositories,
  workflowStates,
  loading,
  saving,
  onSave,
}: {
  workspaceId: string;
  initial: TeamRepoDefault | null;
  repositories: GitRepository[];
  workflowStates: WorkflowState[];
  loading: boolean;
  saving: boolean;
  onSave: (payload: {
    repository_id: string;
    base_branch?: string;
    branch_template?: string;
    auto_sync_states?: boolean;
    review_state_id?: string;
    done_state_id?: string;
    closed_state_id?: string;
  }) => void | Promise<void>;
}) {
  const [repositoryId, setRepositoryId] = useState(initial?.repository_id ?? '');
  const [baseBranch, setBaseBranch] = useState(initial?.base_branch ?? '');
  const [branchTemplate, setBranchTemplate] = useState(initial?.branch_template ?? '{task_key}-{slug}');
  const [autoSyncStates, setAutoSyncStates] = useState(initial?.auto_sync_states ?? true);
  const [reviewStateId, setReviewStateId] = useState(initial?.review_state_id ?? 'none');
  const [doneStateId, setDoneStateId] = useState(initial?.done_state_id ?? 'none');
  const [closedStateId, setClosedStateId] = useState(initial?.closed_state_id ?? 'none');

  useEffect(() => {
    setRepositoryId(initial?.repository_id ?? '');
    setBaseBranch(initial?.base_branch ?? '');
    setBranchTemplate(initial?.branch_template ?? '{task_key}-{slug}');
    setAutoSyncStates(initial?.auto_sync_states ?? true);
    setReviewStateId(initial?.review_state_id ?? 'none');
    setDoneStateId(initial?.done_state_id ?? 'none');
    setClosedStateId(initial?.closed_state_id ?? 'none');
  }, [initial]);

  const selectedRepository = repositories.find((repository) => repository.id === repositoryId) ?? null;

  return (
    <div className="space-y-5 py-2">
      <p className="text-sm text-muted-foreground">
        Tasks on this team inherit this repository and default base branch. The base branch is where task branches start and what pull requests target by default. Task details can still override either one.
      </p>
      <div className="space-y-2">
        <Label>Repository</Label>
        <Select value={repositoryId || undefined} onValueChange={setRepositoryId}>
          <SelectTrigger>
            <SelectValue placeholder={loading ? 'Loading repositories...' : 'Select repository'} />
          </SelectTrigger>
          <SelectContent>
            {repositories.map((repository) => (
              <SelectItem key={repository.id} value={repository.id}>
                {repository.full_name}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
        {selectedRepository && (
          <p className="text-xs text-muted-foreground">Repository default branch: {selectedRepository.default_branch}</p>
        )}
      </div>
      <div className="space-y-2">
        <Label>Default base branch</Label>
        <RepositoryBranchPicker
          workspaceId={workspaceId}
          repositoryId={repositoryId || undefined}
          value={baseBranch}
          onChange={setBaseBranch}
          placeholder={selectedRepository?.default_branch || 'main'}
          emptyLabel={repositoryDefaultBranchLabel(selectedRepository?.default_branch)}
          disabled={saving}
        />
      </div>
      <div className="space-y-2">
        <Label>Branch template</Label>
        <Input
          value={branchTemplate}
          onChange={(event) => setBranchTemplate(event.target.value)}
          placeholder="{task_key}-{slug}"
        />
        <p className="text-xs text-muted-foreground">Available tokens: {'{task_key}'}, {'{workspace_key}'}, {'{display_id}'}, {'{slug}'}, {'{task_type}'}.</p>
      </div>
      <div className="flex items-center justify-between rounded-lg border border-border/60 px-3 py-2">
        <div>
          <p className="text-sm font-medium">Auto-sync workflow state from PR events</p>
          <p className="text-xs text-muted-foreground">
            When enabled, pull request webhooks can move tasks forward automatically.
          </p>
        </div>
        <Switch checked={autoSyncStates} onCheckedChange={setAutoSyncStates} />
      </div>
      <div className="grid gap-4 sm:grid-cols-3">
        <div className="space-y-2">
          <Label>PR opened state</Label>
          <Select value={reviewStateId} onValueChange={setReviewStateId}>
            <SelectTrigger>
              <SelectValue placeholder="Choose review state" />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="none">Do not map</SelectItem>
              {workflowStates.map((state) => (
                <SelectItem key={state.id} value={state.id}>
                  {state.name}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        </div>
        <div className="space-y-2">
          <Label>PR closed state</Label>
          <Select value={closedStateId} onValueChange={setClosedStateId}>
            <SelectTrigger>
              <SelectValue placeholder="Choose closed state" />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="none">Do not map</SelectItem>
              {workflowStates.map((state) => (
                <SelectItem key={state.id} value={state.id}>
                  {state.name}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        </div>
        <div className="space-y-2">
          <Label>PR merged state</Label>
          <Select value={doneStateId} onValueChange={setDoneStateId}>
            <SelectTrigger>
              <SelectValue placeholder="Choose done state" />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="none">Do not map</SelectItem>
              {workflowStates.map((state) => (
                <SelectItem key={state.id} value={state.id}>
                  {state.name}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        </div>
      </div>
      {repositories.length === 0 && !loading && (
        <p className="text-xs text-muted-foreground">
          No workspace repositories are available yet. Add repositories in Workspace Settings before setting a team delivery default.
        </p>
      )}
      <DialogFooter>
        <Button
          disabled={saving || !repositoryId}
          onClick={() => onSave({
            repository_id: repositoryId,
            base_branch: baseBranch || selectedRepository?.default_branch || 'main',
            branch_template: branchTemplate || '{task_key}-{slug}',
            auto_sync_states: autoSyncStates,
            review_state_id: reviewStateId !== 'none' ? reviewStateId : undefined,
            done_state_id: doneStateId !== 'none' ? doneStateId : undefined,
            closed_state_id: closedStateId !== 'none' ? closedStateId : undefined,
          })}
        >
          {saving ? 'Saving...' : 'Save'}
        </Button>
      </DialogFooter>
    </div>
  );
}
