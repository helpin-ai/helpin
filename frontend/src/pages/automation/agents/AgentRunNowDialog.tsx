import { AgentRunDeliveryModePicker } from '@/components/pm/AgentRunDeliveryMode';
import { CodingCapacityNotice } from '@/components/agents/CodingCapacityNotice';
import type { AgentRunDeliveryMode, Agent, AgentTargetType, GitRepository } from '@/lib/pmTypes';
import { Loading01Icon, ZapIcon } from '@/lib/icons';
import { Button } from '@/components/ui/button';
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { Textarea } from '@/components/ui/textarea';
import { CRMRecordPicker } from '@/components/automation/CRMRecordPicker';
import { CRM_RECORD_TARGETS, isCRMRecordTarget } from '@/lib/agentCRMTargets';

const TARGET_ID_LABELS: Partial<Record<AgentTargetType, string>> = {
  task: 'Task ID',
  epic: 'Epic ID',
  sprint: 'Sprint ID',
  objective: 'Objective ID',
  support_conversation: 'Conversation ID',
};

const TARGET_ID_PLACEHOLDERS: Partial<Record<AgentTargetType, string>> = {
  task: 'Paste a task ID',
  epic: 'Paste an epic ID',
  sprint: 'Paste a sprint ID',
  objective: 'Paste an objective ID',
  support_conversation: 'Paste a conversation ID',
};

interface AgentRunNowDialogProps {
  deliveryMode: AgentRunDeliveryMode;
  onDeliveryModeChange: (value: AgentRunDeliveryMode) => void;
  open: boolean;
  onOpenChange: (open: boolean) => void;
  agent: Agent | null;
  targets: AgentTargetType[];
  targetLabel: (target: AgentTargetType) => string;
  targetType: AgentTargetType | '';
  onTargetTypeChange: (value: string) => void;
  workspaceName: string;
  workspaceId: string;
  repositories: GitRepository[];
  runnableRepositories: GitRepository[];
  repositoriesLoading: boolean;
  repositoriesSettingsHref?: string;
  targetId: string;
  onTargetIdChange: (value: string) => void;
  baseBranch: string;
  onBaseBranchChange: (value: string) => void;
  selectedRepository?: GitRepository;
  additionalContext: string;
  onAdditionalContextChange: (value: string) => void;
  submitting: boolean;
  canSubmit: boolean;
  onSubmit: () => void;
}

export function AgentRunNowDialog({
  deliveryMode, onDeliveryModeChange,
  open,
  onOpenChange,
  agent,
  targets,
  targetLabel,
  targetType,
  onTargetTypeChange,
  workspaceName,
  workspaceId,
  repositories,
  runnableRepositories,
  repositoriesLoading,
  repositoriesSettingsHref,
  targetId,
  onTargetIdChange,
  baseBranch,
  onBaseBranchChange,
  selectedRepository,
  additionalContext,
  onAdditionalContextChange,
  submitting,
  canSubmit,
  onSubmit,
}: AgentRunNowDialogProps) {
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-[560px]">
        <DialogHeader>
          <DialogTitle>Run agent now</DialogTitle>
          <DialogDescription>
            {agent ? `Start ${agent.name} manually with a concrete target and optional instructions.` : 'Start this agent manually.'}
          </DialogDescription>
        </DialogHeader>

        <div className="space-y-4">
          <CodingCapacityNotice agent={agent} />
          {['task', 'epic', 'repository'].includes(targetType) ? <AgentRunDeliveryModePicker value={deliveryMode} onChange={onDeliveryModeChange} /> : null}
          {targets.length === 0 ? (
            <div className="rounded-lg border border-dashed border-border/70 px-4 py-3 text-sm text-muted-foreground">
              This agent does not have a manually runnable target enabled.
            </div>
          ) : (
            <>
              <div className="space-y-2">
                <Label htmlFor="run-now-target-type">Target</Label>
                <Select value={targetType} onValueChange={onTargetTypeChange}>
                  <SelectTrigger id="run-now-target-type">
                    <SelectValue placeholder="Choose a target" />
                  </SelectTrigger>
                  <SelectContent>
                    {targets.map((target) => (
                      <SelectItem key={target} value={target}>{targetLabel(target)}</SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              </div>

              {targetType === 'workspace' ? (
                <div className="rounded-lg border border-border/70 bg-muted/20 px-3 py-2">
                  <p className="text-xs font-medium uppercase tracking-[0.14em] text-muted-foreground">Workspace</p>
                  <p className="mt-1 text-sm">{workspaceName}</p>
                </div>
              ) : null}

              {targetType === 'repository' ? (
                <div className="space-y-3">
                  <div className="space-y-2">
                    <Label htmlFor="run-now-repository">Repository</Label>
                    <Select
                      value={targetId}
                      onValueChange={(repositoryId) => {
                        onTargetIdChange(repositoryId);
                        const repository = repositories.find((item) => item.id === repositoryId);
                        onBaseBranchChange(repository?.default_branch ?? '');
                      }}
                      disabled={repositoriesLoading || runnableRepositories.length === 0}
                    >
                      <SelectTrigger id="run-now-repository">
                        <SelectValue placeholder={repositoriesLoading ? 'Loading repositories...' : 'Choose a repository'} />
                      </SelectTrigger>
                      <SelectContent>
                        {runnableRepositories.map((repository) => (
                          <SelectItem key={repository.id} value={repository.id}>{repository.full_name}</SelectItem>
                        ))}
                      </SelectContent>
                    </Select>
                    {!repositoriesLoading && runnableRepositories.length === 0 ? (
                      <p className="text-xs text-muted-foreground">
                        No workspace repositories are available for agent runs.
                        {repositoriesSettingsHref ? (
                          <> <a href={repositoriesSettingsHref} className="underline underline-offset-2 hover:text-foreground">Manage repositories</a></>
                        ) : null}
                      </p>
                    ) : null}
                  </div>
                  <div className="space-y-2">
                    <Label htmlFor="run-now-base-branch">Base branch</Label>
                    <Input
                      id="run-now-base-branch"
                      value={baseBranch}
                      onChange={(event) => onBaseBranchChange(event.target.value)}
                      placeholder={selectedRepository?.default_branch || 'Repository default branch'}
                    />
                  </div>
                </div>
              ) : null}

              {isCRMRecordTarget(targetType) ? (
                <div className="space-y-2">
                  <Label htmlFor="run-now-crm-record">{CRM_RECORD_TARGETS[targetType].singular}</Label>
                  <CRMRecordPicker
                    id="run-now-crm-record"
                    workspaceId={workspaceId}
                    targetType={targetType}
                    value={targetId}
                    onChange={onTargetIdChange}
                    disabled={submitting}
                  />
                </div>
              ) : null}

              {targetType && !['workspace', 'repository'].includes(targetType) && !isCRMRecordTarget(targetType) ? (
                <div className="space-y-2">
                  <Label htmlFor="run-now-target-id">{TARGET_ID_LABELS[targetType] ?? 'Target ID'}</Label>
                  <Input
                    id="run-now-target-id"
                    value={targetId}
                    onChange={(event) => onTargetIdChange(event.target.value)}
                    placeholder={TARGET_ID_PLACEHOLDERS[targetType] ?? 'Paste a target ID'}
                  />
                </div>
              ) : null}

              <div className="space-y-2">
                <Label htmlFor="run-now-context">Run instructions</Label>
                <Textarea
                  id="run-now-context"
                  value={additionalContext}
                  onChange={(event) => onAdditionalContextChange(event.target.value)}
                  placeholder="Add anything this run should focus on."
                  rows={4}
                />
              </div>
            </>
          )}
        </div>

        <DialogFooter>
          <Button type="button" variant="outline" onClick={() => onOpenChange(false)} disabled={submitting}>Cancel</Button>
          <Button type="button" onClick={onSubmit} disabled={!canSubmit || submitting}>
            {submitting ? <Loading01Icon className="mr-1.5 h-3.5 w-3.5 animate-spin" /> : <ZapIcon className="mr-1.5 h-3.5 w-3.5" />}
            Run now
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
