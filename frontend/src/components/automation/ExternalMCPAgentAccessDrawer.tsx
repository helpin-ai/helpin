import { useMemo, useState } from 'react';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import { Link } from '@tanstack/react-router';
import { toast } from 'sonner';
import { QuietSearchInput } from '@/components/design-system/quiet';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Checkbox } from '@/components/ui/checkbox';
import { Sheet, SheetContent, SheetDescription, SheetFooter, SheetHeader, SheetTitle } from '@/components/ui/sheet';
import { agentService } from '@/lib/services/agentService';
import { automationService } from '@/lib/services/automationService';
import { assignedServerTools, editableAgentVersion, saveServerAgentToolAccess } from '@/lib/externalMCPAgentAccess';
import { queryKeys } from '@/lib/queryKeys';
import { unwrap } from '@/lib/queryUtils';
import type { ExternalMCPServer } from '@/lib/externalMCPTypes';
import type { Agent, AgentPresetDefinition } from '@/lib/pmTypes';

type Props = {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  server: ExternalMCPServer;
  workspaceId: string;
  workspaceSlug: string;
  agents: Agent[];
  agentsLoading: boolean;
  agentsError: boolean;
  canManageSettings: boolean;
  canEditCustomAgents: boolean;
  canEditPresetAgents: boolean;
};

export function ExternalMCPAgentAccessDrawer({
  open, onOpenChange, server, workspaceId, workspaceSlug, agents, agentsLoading, agentsError,
  canManageSettings, canEditCustomAgents, canEditPresetAgents,
}: Props) {
  const queryClient = useQueryClient();
  const [selectedAgentId, setSelectedAgentId] = useState<string | null>(null);
  const [selectedAliases, setSelectedAliases] = useState<string[]>([]);
  const [search, setSearch] = useState('');
  const [saving, setSaving] = useState(false);
  const selectedAgent = agents.find((agent) => agent.id === selectedAgentId);
  const presetsQuery = useQuery({
    queryKey: ['external-mcp-agent-access-presets', workspaceId],
    queryFn: async () => unwrap(await agentService.listPresets(workspaceId)),
    enabled: open && Boolean(selectedAgent?.is_system) && canManageSettings && canEditPresetAgents,
    staleTime: 30_000,
  });
  const presets: AgentPresetDefinition[] = presetsQuery.data ?? [];
  const activeVersion = selectedAgent ? editableAgentVersion(selectedAgent, presets) : null;
  const canEdit = Boolean(canManageSettings && selectedAgent && activeVersion
    && (activeVersion.kind === 'preset' ? canEditPresetAgents : canEditCustomAgents));
  const assigned = selectedAgent ? assignedServerTools(selectedAgent.allowed_tools ?? [], server) : [];
  const changed = selectedAliases.length !== assigned.length || selectedAliases.some((alias) => !assigned.includes(alias));
  const matchingAgents = useMemo(() => {
    const query = search.trim().toLowerCase();
    return [...agents].filter((agent) => !query || agent.name.toLowerCase().includes(query))
      .sort((a, b) => {
        const aAssigned = assignedServerTools(a.allowed_tools ?? [], server).length > 0;
        const bAssigned = assignedServerTools(b.allowed_tools ?? [], server).length > 0;
        return Number(bAssigned) - Number(aAssigned) || a.name.localeCompare(b.name);
      });
  }, [agents, search, server]);
  const assignedAgents = agents.filter((agent) => assignedServerTools(agent.allowed_tools ?? [], server).length > 0).length;

  const close = (nextOpen: boolean) => {
    if (saving) return;
    if (!nextOpen) {
      setSelectedAgentId(null);
      setSelectedAliases([]);
      setSearch('');
    }
    onOpenChange(nextOpen);
  };
  const chooseAgent = (agent: Agent) => {
    setSelectedAgentId(agent.id);
    setSelectedAliases(assignedServerTools(agent.allowed_tools ?? [], server));
  };
  const save = async () => {
    if (!selectedAgent || !canEdit || !changed || saving) return;
    setSaving(true);
    try {
      await saveServerAgentToolAccess(selectedAgent, server, selectedAliases, {
        getAgent: async (agentId) => unwrap(await automationService.getAgent(workspaceId, agentId)),
        listCustomVersions: async (agentId) => unwrap(await automationService.listAgentVersions(workspaceId, agentId)),
        listPresets: async () => unwrap(await agentService.listPresets(workspaceId)),
        updateCustomVersion: async (agentId, versionId, allowedTools) => unwrap(await automationService.updateAgentVersion(workspaceId, agentId, versionId, { allowed_tools: allowedTools })),
        updatePresetVersion: async (versionId, allowedTools) => unwrap(await agentService.updatePresetVersion(workspaceId, versionId, { allowed_tools: allowedTools })),
      });
      await queryClient.invalidateQueries({ queryKey: queryKeys.automation.agents(workspaceId) });
      if (selectedAgent.is_system) await presetsQuery.refetch();
      toast.success(`${selectedAgent.name} access updated`);
      setSelectedAgentId(null);
    } catch (error) {
      toast.error('Could not update agent access', { description: error instanceof Error ? error.message : undefined });
    } finally {
      setSaving(false);
    }
  };

  return (
    <Sheet open={open} onOpenChange={close}>
      <SheetContent className="w-full p-0 sm:max-w-xl" aria-label={`${server.name} agent access`}>
        <SheetHeader className="shrink-0 border-b px-5 py-5 pr-14">
          {selectedAgent ? (
            <Button variant="ghost" size="sm" className="-ml-2 w-fit text-muted-foreground" onClick={() => setSelectedAgentId(null)} disabled={saving}>← All agents</Button>
          ) : null}
          <SheetTitle>{selectedAgent ? selectedAgent.name : 'Agent access'}</SheetTitle>
          <SheetDescription>
            {selectedAgent ? `Choose which ${server.name} tools this agent’s active version can use.` : `${server.name} · ${assignedAgents} of ${agents.length} agents assigned`}
          </SheetDescription>
        </SheetHeader>

        <div className="min-h-0 flex-1 overflow-y-auto px-5 py-4">
          {!selectedAgent ? (
            <div className="space-y-3">
              {agents.length > 10 ? <QuietSearchInput aria-label="Search agents" placeholder="Search agents" value={search} onChange={(event) => setSearch(event.target.value)} /> : null}
              {agentsLoading ? <p className="py-8 text-center text-sm text-muted-foreground">Loading agents…</p> : null}
              {agentsError ? <p className="rounded-lg border border-destructive/30 bg-destructive/5 p-4 text-sm text-destructive">Agent access could not be loaded. Try again later.</p> : null}
              {!agentsLoading && !agentsError && agents.length === 0 ? <p className="rounded-lg border border-dashed p-8 text-center text-sm text-muted-foreground">No agents in this workspace yet.</p> : null}
              {!agentsLoading && !agentsError && agents.length > 0 && matchingAgents.length === 0 ? <p className="py-8 text-center text-sm text-muted-foreground">No agents match your search.</p> : null}
              {!agentsLoading && !agentsError ? matchingAgents.map((agent) => {
                const count = assignedServerTools(agent.allowed_tools ?? [], server).length;
                return (
                  <button key={agent.id} type="button" onClick={() => chooseAgent(agent)} className="flex w-full items-center gap-3 rounded-lg border border-border/80 bg-background px-4 py-3 text-left transition-colors hover:border-primary/30 hover:bg-muted/30 focus-visible:outline-2 focus-visible:outline-primary">
                    <span className="flex size-9 shrink-0 items-center justify-center rounded-lg bg-muted text-sm font-semibold text-muted-foreground">{agent.name.trim().charAt(0).toUpperCase() || 'A'}</span>
                    <span className="min-w-0 flex-1"><span className="block truncate text-sm font-medium">{agent.name}</span><span className="block text-xs text-muted-foreground">{agent.is_system ? 'Preset agent' : 'Custom agent'} · Active version</span></span>
                    <span className="shrink-0 text-xs text-muted-foreground">{count ? `${count} tool${count === 1 ? '' : 's'}` : 'No access'}</span>
                    <span aria-hidden="true" className="text-muted-foreground">›</span>
                  </button>
                );
              }) : null}
            </div>
          ) : (
            <div className="space-y-4">
              {!server.enabled || server.status !== 'connected' ? <p className="rounded-lg border border-amber-300/50 bg-amber-50/70 px-3 py-2 text-xs text-amber-800 dark:bg-amber-950/20 dark:text-amber-300">Assignments take effect when this server is enabled and connected.</p> : null}
              {selectedAgent.is_system && presetsQuery.isLoading ? <p className="text-sm text-muted-foreground">Checking active version…</p> : null}
              {selectedAgent.is_system && presetsQuery.isError ? <p className="text-sm text-destructive">Could not check this agent’s active version.</p> : null}
              {selectedAgent.is_system && canManageSettings && canEditPresetAgents && !presetsQuery.isLoading && !presetsQuery.isError && !activeVersion ? <p className="rounded-lg border bg-muted/30 p-3 text-sm text-muted-foreground">This agent uses a product version. Duplicate it in the agent editor before changing its tools.</p> : null}
              {selectedAgent.is_system && (!canManageSettings || !canEditPresetAgents) ? <p className="rounded-lg border bg-muted/30 p-3 text-sm text-muted-foreground">You need workspace management and PM editing access to change preset agent tools.</p> : null}
              {!canEdit && !selectedAgent.is_system && !selectedAgent.active_version_id ? <p className="rounded-lg border bg-muted/30 p-3 text-sm text-muted-foreground">This agent has no active version to edit.</p> : null}
              {!canEdit && activeVersion && !canManageSettings ? <p className="rounded-lg border bg-muted/30 p-3 text-sm text-muted-foreground">Only workspace managers can change agent access.</p> : null}
              {!canEdit && activeVersion && canManageSettings && !(activeVersion.kind === 'preset' ? canEditPresetAgents : canEditCustomAgents) ? <p className="rounded-lg border bg-muted/30 p-3 text-sm text-muted-foreground">You need agent editing access to change this version.</p> : null}
              <div className="flex items-center justify-between gap-3"><p className="text-sm font-medium">Tools on {server.name}</p><Badge variant="secondary">{selectedAliases.length} selected</Badge></div>
              {(server.tools ?? []).length === 0 ? <p className="rounded-lg border border-dashed p-6 text-center text-sm text-muted-foreground">No tools discovered. Refresh this server’s tools first.</p> : null}
              <div className="space-y-2">
                {(server.tools ?? []).map((tool) => {
                  const selected = selectedAliases.includes(tool.runtime_alias);
                  const disabled = !canEdit || saving || (!tool.enabled && !selected);
                  return (
                    <label key={tool.id} className="flex cursor-pointer gap-3 rounded-lg border border-border/80 px-3 py-3 has-disabled:cursor-default">
                      <Checkbox checked={selected} disabled={disabled} onCheckedChange={(checked) => setSelectedAliases((current) => checked === true ? [...current, tool.runtime_alias] : current.filter((alias) => alias !== tool.runtime_alias))} aria-label={`${selected ? 'Remove' : 'Allow'} ${tool.remote_name} for ${selectedAgent.name}`} />
                      <span className="min-w-0 flex-1"><span className="flex flex-wrap items-center gap-2"><span className="break-all font-mono text-xs font-medium">{tool.remote_name}</span><Badge variant="outline" className="text-[10px] capitalize">{tool.access}</Badge>{!tool.enabled ? <Badge variant="secondary" className="text-[10px]">Off</Badge> : null}</span>{tool.description ? <span className="mt-1 block text-xs leading-5 text-muted-foreground">{tool.description}</span> : null}</span>
                    </label>
                  );
                })}
              </div>
            </div>
          )}
        </div>

        <SheetFooter className="shrink-0 flex-row items-center justify-between border-t px-5 py-4">
          {workspaceSlug ? <Link to="/w/$slug/automation/agents" params={{ slug: workspaceSlug }} search={{ agent_id: selectedAgent?.id }} className="text-xs text-muted-foreground underline-offset-4 hover:text-foreground hover:underline">Open agent editor</Link> : <span />}
          {selectedAgent && canEdit ? <Button size="sm" disabled={!changed || saving} onClick={() => void save()}>{saving ? 'Saving…' : 'Save access'}</Button> : null}
        </SheetFooter>
      </SheetContent>
    </Sheet>
  );
}
