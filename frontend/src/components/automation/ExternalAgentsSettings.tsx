import { useState, type FormEvent } from 'react';
import { toast } from 'sonner';
import {
  QuietEmptyState,
  QuietIconAction,
  QuietMetaLine,
  QuietPrimaryAction,
  QuietStatusText,
  QuietTextAction,
  QuietUnderlineInput,
} from '@/components/design-system/quiet';
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert';
import { useConfirm } from '@/components/ui/confirm-dialog';
import { Dialog, DialogContent, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog';
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu';
import { Skeleton } from '@/components/ui/skeleton';
import {
  useDeleteExternalAgent,
  useExternalAgents,
  useRefreshExternalAgentCard,
  useUpdateExternalAgent,
} from '@/hooks/queries/useExternalAgents';
import { useWorkspaceTeams } from '@/hooks/useWorkspaceTeams';
import type { ExternalAgent, ExternalAgentStatus } from '@/lib/externalAgentTypes';
import {
  ArrowReloadHorizontalIcon,
  Delete01Icon,
  Key01Icon,
  MoreHorizontalIcon,
  PauseIcon,
  PencilEdit02Icon,
  PlayIcon,
  Shield01Icon,
} from '@/lib/icons';
import { cn, timeAgo } from '@/lib/utils';
import { ExternalAgentAddDialog } from './ExternalAgentAddDialog';
import {
  A2A_LEARN_MORE_URL,
  externalAgentHost,
  externalAgentProtocolLabel,
  externalAgentTeamSummary,
} from '@/lib/externalAgents';
import { ExternalAgentField, ExternalAgentTeamPicker } from './externalAgentParts';

type ExternalAgentsSettingsProps = {
  workspaceId: string;
  canManageSettings: boolean;
  createOpen: boolean;
  onCreateOpenChange: (open: boolean) => void;
};

const STATUS_COPY: Record<ExternalAgentStatus, { label: string; tone: 'positive' | 'neutral' | 'blocker' }> = {
  active: { label: 'Active', tone: 'positive' },
  disabled: { label: 'Disabled', tone: 'neutral' },
  error: { label: 'Error', tone: 'blocker' },
};

const VISIBLE_SKILL_NAMES = 4;

export function ExternalAgentsSettings({ workspaceId, canManageSettings, createOpen, onCreateOpenChange }: ExternalAgentsSettingsProps) {
  const agentsQuery = useExternalAgents(workspaceId);
  const configured = agentsQuery.data?.configured !== false;
  const agents = agentsQuery.data?.items ?? [];

  if (agentsQuery.isLoading) {
    return <div className="space-y-3"><Skeleton className="h-24" /><Skeleton className="h-24" /></div>;
  }
  if (agentsQuery.isError) {
    return (
      <Alert variant="destructive">
        <AlertTitle>Could not load external agents</AlertTitle>
        <AlertDescription>{agentsQuery.error.message}</AlertDescription>
      </Alert>
    );
  }
  if (!configured) {
    return (
      <Alert data-external-agents-not-configured>
        <Shield01Icon className="h-4 w-4" />
        <AlertTitle>External agents are not configured on this server</AlertTitle>
        <AlertDescription>
          A server operator must set EXTERNAL_A2A_ENCRYPTION_KEY so Helpin can store agent access tokens encrypted.
        </AlertDescription>
      </Alert>
    );
  }

  return (
    <div className="max-w-4xl space-y-4">
      {!canManageSettings ? (
        <Alert>
          <Shield01Icon className="h-4 w-4" />
          <AlertTitle>External agents are read-only</AlertTitle>
          <AlertDescription>A workspace admin can add agents, rotate their tokens, and choose which teams can assign them tasks.</AlertDescription>
        </Alert>
      ) : null}

      {agents.length === 0 ? (
        <QuietEmptyState
          title="No external agents yet"
          description="External agents run outside Helpin, for example a coding agent on your own server. Helpin sends them tasks over the open Agent2Agent (A2A) protocol, and their answers and files come back to the task."
          action={(
            <div className="flex flex-wrap items-center gap-4">
              {canManageSettings ? (
                <QuietTextAction type="button" className="underline underline-offset-4" onClick={() => onCreateOpenChange(true)}>
                  Add an external agent
                </QuietTextAction>
              ) : (
                <span className="text-[12.5px] text-quiet-text-tertiary">A workspace admin can add one.</span>
              )}
              <a
                href={A2A_LEARN_MORE_URL}
                target="_blank"
                rel="noreferrer"
                className="text-[12.5px] text-quiet-text-secondary underline underline-offset-4 hover:text-quiet-text-primary"
              >
                About the A2A protocol
              </a>
            </div>
          )}
        />
      ) : (
        <div className="overflow-hidden rounded-lg border border-quiet-divider-strong" data-external-agents-list>
          {agents.map((agent) => (
            <ExternalAgentRow key={agent.id} workspaceId={workspaceId} agent={agent} canManageSettings={canManageSettings} />
          ))}
        </div>
      )}

      {canManageSettings ? (
        <ExternalAgentAddDialog open={createOpen} onOpenChange={onCreateOpenChange} workspaceId={workspaceId} />
      ) : null}
    </div>
  );
}

export function ExternalAgentsAddButton({ workspaceId, onAdd }: { workspaceId: string; onAdd: () => void }) {
  const agentsQuery = useExternalAgents(workspaceId);
  return (
    <QuietPrimaryAction type="button" onClick={onAdd} disabled={agentsQuery.isLoading || agentsQuery.data?.configured !== true}>
      Add agent
    </QuietPrimaryAction>
  );
}

function ExternalAgentRow({ workspaceId, agent, canManageSettings }: { workspaceId: string; agent: ExternalAgent; canManageSettings: boolean }) {
  const [dialog, setDialog] = useState<'edit' | 'token' | null>(null);
  const confirm = useConfirm();
  const update = useUpdateExternalAgent(workspaceId);
  const refresh = useRefreshExternalAgentCard(workspaceId);
  const remove = useDeleteExternalAgent(workspaceId);
  const { findTeamName } = useWorkspaceTeams(workspaceId);
  const status = STATUS_COPY[agent.status] ?? STATUS_COPY.error;
  const skills = agent.skills ?? [];
  const skillNames = skills.slice(0, VISIBLE_SKILL_NAMES).map((skill) => skill.name || skill.id);
  const hiddenSkills = skills.length - skillNames.length;
  const disabled = agent.status === 'disabled';
  const busy = update.isPending || refresh.isPending || remove.isPending;

  const refreshCard = async () => {
    try {
      const refreshed = await refresh.mutateAsync(agent.id);
      if (refreshed.status === 'error' && refreshed.last_error) toast.error(refreshed.last_error);
      else toast.success('Agent Card refreshed');
    } catch (error) {
      toast.error(error instanceof Error ? error.message : 'Could not refresh the Agent Card');
    }
  };

  const setStatus = async (next: 'active' | 'disabled') => {
    if (next === 'disabled') {
      const accepted = await confirm({
        title: `Disable ${agent.name}?`,
        description: 'Helpin stops sending it tasks until you enable it again. Its settings and token stay saved.',
        confirmText: 'Disable agent',
      });
      if (!accepted) return;
    }
    try {
      await update.mutateAsync({ externalAgentId: agent.id, request: { status: next } });
      toast.success(next === 'active' ? `${agent.name} enabled` : `${agent.name} disabled`);
    } catch (error) {
      toast.error(error instanceof Error ? error.message : 'Could not update the agent');
    }
  };

  const removeAgent = async () => {
    const accepted = await confirm({
      title: `Remove ${agent.name}?`,
      description: 'Helpin archives the linked agent, so tasks can no longer be assigned to it. Past runs, comments, and files stay on their tasks.',
      confirmText: 'Remove agent',
      variant: 'destructive',
    });
    if (!accepted) return;
    try {
      await remove.mutateAsync(agent.id);
      toast.success(`${agent.name} removed`);
    } catch (error) {
      toast.error(error instanceof Error ? error.message : 'Could not remove the agent');
    }
  };

  return (
    <div
      className={cn('grid grid-cols-[minmax(0,1fr)_auto] gap-x-3 border-b border-quiet-divider-light px-4 py-3 last:border-b-0 sm:px-5', disabled && 'opacity-75')}
      data-external-agent-row={agent.id}
    >
      <div className="min-w-0">
        <div className="flex min-w-0 flex-wrap items-center gap-x-3 gap-y-1">
          <p className="min-w-0 truncate text-[13.5px] font-semibold tracking-[-0.008em] text-quiet-text-primary">{agent.name}</p>
          <QuietStatusText tone={status.tone}>{status.label}</QuietStatusText>
        </div>
        {agent.description ? (
          <p className="mt-0.5 line-clamp-2 text-[12.5px] leading-5 text-quiet-text-tertiary">{agent.description}</p>
        ) : null}
        {agent.status === 'error' && agent.last_error ? (
          <p className="mt-1 text-[12.5px] leading-5 text-quiet-accent" role="status">{agent.last_error}</p>
        ) : null}
        <QuietMetaLine
          className="mt-1.5"
          items={[
            agent.provider_name || null,
            agent.version ? `v${agent.version}` : null,
            externalAgentProtocolLabel(agent),
            <span key="host" title={agent.interface_url || agent.card_url}>{externalAgentHost(agent.interface_url || agent.card_url)}</span>,
          ]}
        />
        <p className="mt-1 text-[12px] leading-5 text-quiet-text-tertiary" data-external-agent-skills>
          {skills.length === 0
            ? 'No skills listed'
            : `${skills.length} ${skills.length === 1 ? 'skill' : 'skills'}: ${skillNames.join(', ')}${hiddenSkills > 0 ? `, +${hiddenSkills} more` : ''}`}
        </p>
        <QuietMetaLine
          className="mt-1"
          items={[
            agent.token_hint ? <span key="token" className="font-mono">Token {agent.token_hint}</span> : null,
            `Teams: ${externalAgentTeamSummary(agent.allowed_team_ids ?? [], (teamId) => findTeamName(teamId))}`,
            agent.last_checked_at ? `Checked ${timeAgo(agent.last_checked_at)}` : 'Not checked yet',
          ]}
        />
      </div>
      {canManageSettings ? (
        <div className="flex items-start gap-1 self-start">
          <QuietIconAction
            type="button"
            aria-label={`Refresh Agent Card for ${agent.name}`}
            title="Refresh Agent Card"
            disabled={busy}
            onClick={() => void refreshCard()}
          >
            <ArrowReloadHorizontalIcon className={cn('h-[15px] w-[15px]', refresh.isPending && 'animate-spin')} />
          </QuietIconAction>
          <DropdownMenu>
            <DropdownMenuTrigger asChild>
              <QuietIconAction type="button" aria-label={`More actions for ${agent.name}`} disabled={busy}>
                <MoreHorizontalIcon className="h-[15px] w-[15px]" />
              </QuietIconAction>
            </DropdownMenuTrigger>
            <DropdownMenuContent align="end">
              <DropdownMenuItem onSelect={() => setDialog('edit')}><PencilEdit02Icon className="h-4 w-4" />Edit name and teams</DropdownMenuItem>
              <DropdownMenuItem onSelect={() => setDialog('token')}><Key01Icon className="h-4 w-4" />Rotate token</DropdownMenuItem>
              {disabled ? (
                <DropdownMenuItem onSelect={() => void setStatus('active')}><PlayIcon className="h-4 w-4" />Enable</DropdownMenuItem>
              ) : (
                <DropdownMenuItem onSelect={() => void setStatus('disabled')}><PauseIcon className="h-4 w-4" />Disable</DropdownMenuItem>
              )}
              <DropdownMenuSeparator />
              <DropdownMenuItem variant="destructive" onSelect={() => void removeAgent()}><Delete01Icon className="h-4 w-4" />Remove</DropdownMenuItem>
            </DropdownMenuContent>
          </DropdownMenu>
        </div>
      ) : null}
      {dialog === 'edit' ? <ExternalAgentEditDialog workspaceId={workspaceId} agent={agent} onClose={() => setDialog(null)} /> : null}
      {dialog === 'token' ? <ExternalAgentTokenDialog workspaceId={workspaceId} agent={agent} onClose={() => setDialog(null)} /> : null}
    </div>
  );
}

function ExternalAgentEditDialog({ workspaceId, agent, onClose }: { workspaceId: string; agent: ExternalAgent; onClose: () => void }) {
  const update = useUpdateExternalAgent(workspaceId);
  const [name, setName] = useState(agent.name);
  const [teamIds, setTeamIds] = useState<string[]>(agent.allowed_team_ids ?? []);
  const [error, setError] = useState<string | null>(null);

  const submit = async (event: FormEvent) => {
    event.preventDefault();
    if (!name.trim() || update.isPending) return;
    setError(null);
    try {
      await update.mutateAsync({ externalAgentId: agent.id, request: { name: name.trim(), allowed_team_ids: teamIds } });
      toast.success('Agent updated');
      onClose();
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Could not update the agent');
    }
  };

  return (
    <Dialog open onOpenChange={(open) => { if (!open && !update.isPending) onClose(); }}>
      <DialogContent className="sm:max-w-lg" aria-describedby={undefined}>
        <DialogHeader className="border-b border-quiet-divider-strong pb-3">
          <DialogTitle className="text-[20px] font-semibold tracking-[-0.018em]">Edit {agent.name}</DialogTitle>
        </DialogHeader>
        <form className="space-y-5" onSubmit={(event) => void submit(event)}>
          <ExternalAgentField label="Name" id="external-agent-name">
            <QuietUnderlineInput id="external-agent-name" value={name} onChange={(event) => setName(event.target.value)} required disabled={update.isPending} />
          </ExternalAgentField>
          <ExternalAgentTeamPicker workspaceId={workspaceId} value={teamIds} onChange={setTeamIds} disabled={update.isPending} id="external-agent-edit-teams" />
          {error ? <p role="alert" className="text-[12.5px] text-destructive">{error}</p> : null}
          <DialogFooter className="border-t border-quiet-divider-strong pt-3 sm:items-center">
            <QuietTextAction type="button" disabled={update.isPending} onClick={onClose}>Cancel</QuietTextAction>
            <QuietPrimaryAction type="submit" disabled={update.isPending || !name.trim()}>{update.isPending ? 'Saving…' : 'Save'}</QuietPrimaryAction>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}

function ExternalAgentTokenDialog({ workspaceId, agent, onClose }: { workspaceId: string; agent: ExternalAgent; onClose: () => void }) {
  const update = useUpdateExternalAgent(workspaceId);
  const [token, setToken] = useState('');
  const [error, setError] = useState<string | null>(null);

  const submit = async (event: FormEvent) => {
    event.preventDefault();
    if (!token.trim() || update.isPending) return;
    setError(null);
    try {
      await update.mutateAsync({ externalAgentId: agent.id, request: { token: token.trim() } });
      toast.success('Token updated');
      onClose();
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Could not update the token');
    }
  };

  return (
    <Dialog open onOpenChange={(open) => { if (!open && !update.isPending) onClose(); }}>
      <DialogContent className="sm:max-w-lg" aria-describedby={undefined}>
        <DialogHeader className="border-b border-quiet-divider-strong pb-3">
          <DialogTitle className="text-[20px] font-semibold tracking-[-0.018em]">Rotate token for {agent.name}</DialogTitle>
        </DialogHeader>
        <form className="space-y-5" onSubmit={(event) => void submit(event)}>
          <ExternalAgentField label="New access token" id="external-agent-new-token">
            <QuietUnderlineInput
              id="external-agent-new-token"
              type="password"
              autoComplete="new-password"
              autoFocus
              value={token}
              onChange={(event) => setToken(event.target.value)}
              placeholder="Bearer token"
              className="font-mono"
              disabled={update.isPending}
            />
          </ExternalAgentField>
          <p className="text-[12.5px] leading-5 text-quiet-text-tertiary">
            New runs use the new token. {agent.token_hint ? <>The current token ends in <span className="font-mono">{agent.token_hint.replace(/^…/, '')}</span>.</> : null}
          </p>
          {error ? <p role="alert" className="text-[12.5px] text-destructive">{error}</p> : null}
          <DialogFooter className="border-t border-quiet-divider-strong pt-3 sm:items-center">
            <QuietTextAction type="button" disabled={update.isPending} onClick={onClose}>Cancel</QuietTextAction>
            <QuietPrimaryAction type="submit" disabled={update.isPending || !token.trim()}>{update.isPending ? 'Saving…' : 'Save token'}</QuietPrimaryAction>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}
