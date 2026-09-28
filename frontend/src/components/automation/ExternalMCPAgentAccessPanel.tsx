import { useMemo, useState } from 'react';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import { Link } from '@tanstack/react-router';
import { toast } from 'sonner';
import { AgentAvatar } from '@/components/agents/AgentAvatar';
import { QuietSearchInput } from '@/components/design-system/quiet';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Checkbox } from '@/components/ui/checkbox';
import { agentService } from '@/lib/services/agentService';
import { automationService } from '@/lib/services/automationService';
import { assignedServerTools, editableAgentVersion, saveServerAgentToolAccess, serverAgentAccessState } from '@/lib/externalMCPAgentAccess';
import { queryKeys } from '@/lib/queryKeys';
import { unwrap } from '@/lib/queryUtils';
import type { ExternalMCPServer } from '@/lib/externalMCPTypes';
import type { Agent, AgentPresetDefinition } from '@/lib/pmTypes';

type Props = {
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

export function ExternalMCPAgentAccessPanel({
  server, workspaceId, workspaceSlug, agents, agentsLoading, agentsError,
  canManageSettings, canEditCustomAgents, canEditPresetAgents,
}: Props) {
  const queryClient = useQueryClient();
  const [selectedAgentId, setSelectedAgentId] = useState<string | null>(null);
  const [draftAliases, setSelectedAliases] = useState<string[] | null>(null);
  const [search, setSearch] = useState('');
  const [saving, setSaving] = useState(false);
  const selectedAgent = agents.find((agent) => agent.id === selectedAgentId);
  const presetsQuery = useQuery({
    queryKey: ['external-mcp-agent-access-presets', workspaceId],
    queryFn: async () => unwrap(await agentService.listPresets(workspaceId)),
    enabled: Boolean(selectedAgent?.is_system) && canManageSettings && canEditPresetAgents,
    staleTime: 30_000,
  });
  const presets: AgentPresetDefinition[] = presetsQuery.data ?? [];
  const activeVersion = selectedAgent ? editableAgentVersion(selectedAgent, presets) : null;
  const canEdit = Boolean(canManageSettings && selectedAgent && activeVersion
    && (activeVersion.kind === 'preset' ? canEditPresetAgents : canEditCustomAgents));
  const assigned = selectedAgent ? assignedServerTools(selectedAgent.allowed_tools ?? [], server) : [];
  const enabledAliases = (server.tools ?? []).filter((tool) => tool.enabled).map((tool) => tool.runtime_alias);
  const selectedAliases = draftAliases ?? (assigned.length || !canEdit ? assigned : enabledAliases);
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

  const chooseAgent = (agent: Agent) => {
    setSelectedAgentId(agent.id);
    setSelectedAliases(null);
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
    <div data-external-mcp-agent-access className="min-w-0 border-t bg-muted/15 px-4 py-4">
      <div className="max-w-3xl">
        <div className="mb-4 flex flex-wrap items-start justify-between gap-3">
          {selectedAgent ? <div>
            <Button variant="ghost" size="sm" className="-ml-2 mb-2 text-muted-foreground" onClick={() => setSelectedAgentId(null)} disabled={saving}>← All agents</Button>
            <div className="flex items-center gap-3">
              <AgentAvatar agent={selectedAgent} className="size-10 rounded-none border-0 bg-transparent shadow-none" genericBare />
              <div><p className="text-sm font-semibold">{selectedAgent.name}</p><p className="text-xs text-muted-foreground">Choose which {server.name} tools this agent can use.</p></div>
            </div>
          </div> : <div><p className="text-sm font-semibold">Agent access</p><p className="text-xs text-muted-foreground">Select an agent to manage its tools.</p></div>}
          {!selectedAgent && !agentsLoading && !agentsError ? <Badge variant="outline">{assignedAgents} of {agents.length} assigned</Badge> : null}
        </div>

        <div className="max-h-[60vh] overflow-y-auto pr-1">
          {!selectedAgent ? (
            <div className="space-y-3">
              {agents.length > 10 ? <QuietSearchInput aria-label="Search agents" placeholder="Search agents" value={search} onChange={(event) => setSearch(event.target.value)} /> : null}
              {agentsLoading ? <p className="py-8 text-center text-sm text-muted-foreground">Loading agents…</p> : null}
              {agentsError ? <p className="rounded-lg border border-destructive/30 bg-destructive/5 p-4 text-sm text-destructive">Agent access could not be loaded. Try again later.</p> : null}
              {!agentsLoading && !agentsError && agents.length === 0 ? <p className="rounded-lg border border-dashed p-8 text-center text-sm text-muted-foreground">No agents in this workspace yet.</p> : null}
              {!agentsLoading && !agentsError && agents.length > 0 && matchingAgents.length === 0 ? <p className="py-8 text-center text-sm text-muted-foreground">No agents match your search.</p> : null}
              {!agentsLoading && !agentsError ? matchingAgents.map((agent) => {
                const count = assignedServerTools(agent.allowed_tools ?? [], server).length;
                const accessState = serverAgentAccessState(agent.allowed_tools ?? [], server);
                return (
                  <button key={agent.id} data-agent-access-row type="button" onClick={() => chooseAgent(agent)} className="flex w-full items-center gap-3 rounded-lg border border-border/80 bg-background px-3 py-3 text-left transition-colors hover:border-primary/30 hover:bg-muted/30 focus-visible:outline-2 focus-visible:outline-primary">
                    <AgentAvatar agent={agent} className="size-9 rounded-none border-0 bg-transparent shadow-none" genericBare />
                    <span className="min-w-0 flex-1"><span className="block truncate text-sm font-medium">{agent.name}</span><span className="block text-xs text-muted-foreground">{count ? `${count} tool${count === 1 ? '' : 's'} assigned` : agent.is_system ? 'Preset agent' : 'Custom agent'}</span></span>
                    <Badge variant="outline" className={accessState === 'available' ? 'shrink-0 border-emerald-300/70 bg-emerald-50 text-emerald-700 dark:border-emerald-900 dark:bg-emerald-950/30 dark:text-emerald-300' : accessState === 'inactive' ? 'shrink-0 border-amber-300/70 bg-amber-50 text-amber-800 dark:border-amber-900 dark:bg-amber-950/30 dark:text-amber-300' : 'shrink-0 text-muted-foreground'}>{accessState === 'available' ? 'Has access' : accessState === 'inactive' ? 'Assigned, unavailable' : 'No access'}</Badge>
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
              <div className="flex flex-wrap items-center justify-between gap-x-3 gap-y-1">
                <p className="text-sm font-medium">Tools on {server.name} <span className="ml-2 text-xs font-normal text-muted-foreground">{selectedAliases.length} selected</span></p>
                {canEdit && (server.tools ?? []).length > 0 ? <div className="flex items-center gap-1">
                  <Button variant="ghost" size="sm" disabled={saving || enabledAliases.every((alias) => selectedAliases.includes(alias))} onClick={() => setSelectedAliases([...new Set([...selectedAliases, ...enabledAliases])])}>Select all</Button>
                  <Button variant="ghost" size="sm" disabled={saving || selectedAliases.length === 0} onClick={() => setSelectedAliases([])}>Select none</Button>
                </div> : null}
              </div>
              {(server.tools ?? []).length === 0 ? <p className="rounded-lg border border-dashed p-6 text-center text-sm text-muted-foreground">No tools discovered. Refresh this server’s tools first.</p> : null}
              <div className="space-y-2">
                {(server.tools ?? []).map((tool) => {
                  const selected = selectedAliases.includes(tool.runtime_alias);
                  const disabled = !canEdit || saving || (!tool.enabled && !selected);
                  return (
                    <label key={tool.id} className="flex cursor-pointer gap-3 rounded-lg border border-border/80 px-3 py-3 has-disabled:cursor-default">
                      <Checkbox checked={selected} disabled={disabled} onCheckedChange={(checked) => setSelectedAliases(checked === true ? [...selectedAliases, tool.runtime_alias] : selectedAliases.filter((alias) => alias !== tool.runtime_alias))} aria-label={`${selected ? 'Remove' : 'Allow'} ${tool.remote_name} for ${selectedAgent.name}`} />
                      <span className="min-w-0 flex-1"><span className="flex flex-wrap items-center gap-2"><span className="break-all font-mono text-xs font-medium">{tool.remote_name}</span><Badge variant="outline" className="text-[10px] capitalize">{tool.access}</Badge>{!tool.enabled ? <Badge variant="secondary" className="text-[10px]">Off</Badge> : null}</span>{tool.description ? <span className="mt-1 block text-xs leading-5 text-muted-foreground">{tool.description}</span> : null}</span>
                    </label>
                  );
                })}
              </div>
            </div>
          )}
        </div>

        <div className="mt-4 flex items-center justify-between gap-3 border-t pt-3">
          {workspaceSlug ? <Link to="/w/$slug/automation/agents" params={{ slug: workspaceSlug }} search={{ agent_id: selectedAgent?.id }} className="text-xs text-muted-foreground underline-offset-4 hover:text-foreground hover:underline">{selectedAgent ? 'Open agent editor' : 'Manage agents'}</Link> : <span />}
          {selectedAgent && canEdit ? <Button size="sm" disabled={!changed || saving} onClick={() => void save()}>{saving ? 'Saving…' : 'Save access'}</Button> : null}
        </div>
      </div>
    </div>
  );
}
