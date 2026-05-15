import { useEffect, useState, type ReactNode } from 'react';
import type { WorkspaceTeam } from '@/lib/types';
import { Badge } from '@/components/ui/badge';
import { AGENT_RUNTIME_LABELS } from '@/lib/agentRuntime';
import { HelpCircleIcon } from '@/lib/icons';
import {
  Command,
  CommandEmpty,
  CommandGroup,
  CommandInput,
  CommandItem,
  CommandList,
} from '@/components/ui/command';
import {
  Popover,
  PopoverContent,
  PopoverTrigger,
} from '@/components/ui/popover';
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select';
import {
  Tooltip,
  TooltipContent,
  TooltipProvider,
  TooltipTrigger,
} from '@/components/ui/tooltip';
import type {
  AgentApprovalMode,
  AgentInvocationMode,
  AgentModelProvider,
  AgentModelProviderOption,
  AgentRuntimeKind,
  AgentSkillRef,
  AgentTargetType,
  SkillCatalogEntry,
  ToolCatalogEntry,
} from '@/lib/pmTypes';
import { automationService } from '@/lib/services/automationService';
import {
  applyCustomAgentDraftToForm,
  createDefaultCustomAgentForm,
  validateCustomAgentCreateForm,
  type CustomAgentFormData,
} from './customAgentCreateModel';

const TARGET_OPTIONS: Array<{ value: AgentTargetType; label: string }> = [
  { value: 'task', label: 'Tasks' },
  { value: 'epic', label: 'Epics' },
  { value: 'repository', label: 'Repositories' },
  { value: 'workspace', label: 'Workspace' },
  { value: 'crm_deal', label: 'CRM deals' },
  { value: 'document', label: 'Documents' },
  { value: 'support_conversation', label: 'Support conversations' },
];

const CUSTOM_RUNTIME_KIND_OPTIONS: AgentRuntimeKind[] = ['opencode', 'native_sdk'];
const DRAFT_PROGRESS_LABELS = [
  'Reading brief...',
  'Choosing targets...',
  'Matching tools...',
  'Checking access...',
  'Drafting instructions...',
];
const INVOCATION_MODE_LABELS: Record<AgentInvocationMode, string> = {
  autonomous: 'Autonomous',
  interactive: 'Interactive',
};

function supportedModesForRuntime(runtimeKind: AgentRuntimeKind): AgentInvocationMode[] {
  if (runtimeKind === 'native_sdk' || runtimeKind === 'codex') {
    return ['autonomous', 'interactive'];
  }
  return ['autonomous'];
}

function normalizeInvocationMode(value: AgentInvocationMode, runtimeKind: AgentRuntimeKind): AgentInvocationMode {
  const supportedModes = supportedModesForRuntime(runtimeKind);
  return supportedModes.includes(value) ? value : 'autonomous';
}

function FieldLabel({ children, tooltip }: { children: ReactNode; tooltip?: string }) {
  return (
    <span className="flex items-center gap-1.5 text-sm font-medium">
      {children}
      {tooltip ? (
        <Tooltip>
          <TooltipTrigger asChild>
            <button type="button" className="rounded-sm text-muted-foreground/70 hover:text-foreground" aria-label={`${children} help`}>
              <HelpCircleIcon className="h-3.5 w-3.5" />
            </button>
          </TooltipTrigger>
          <TooltipContent side="right" className="max-w-64 text-xs leading-relaxed">
            {tooltip}
          </TooltipContent>
        </Tooltip>
      ) : null}
    </span>
  );
}

export interface CustomAgentCreatePanelProps {
  workspaceId: string;
  form: CustomAgentFormData;
  onChange: (form: CustomAgentFormData) => void;
  teams: WorkspaceTeam[];
  tools: ToolCatalogEntry[];
  skills: SkillCatalogEntry[];
  providerOptions: AgentModelProviderOption[];
  advancedOpen: boolean;
  onAdvancedOpenChange: (open: boolean) => void;
  onCreate: () => void;
  saving: boolean;
}

export function CustomAgentCreatePanel({
  workspaceId,
  form,
  onChange,
  teams,
  tools,
  skills,
  providerOptions,
  onCreate,
  saving,
}: CustomAgentCreatePanelProps) {
  const [started, setStarted] = useState(false);
  const [approvalOpen, setApprovalOpen] = useState(false);
  const [advancedSettingsOpen, setAdvancedSettingsOpen] = useState(false);
  const [draftDescription, setDraftDescription] = useState('');
  const [drafting, setDrafting] = useState(false);
  const [draftProgressStep, setDraftProgressStep] = useState(0);
  const [draftError, setDraftError] = useState('');
  const [toolPickerOpen, setToolPickerOpen] = useState(false);
  const [skillPickerOpen, setSkillPickerOpen] = useState(false);
  const [toolSearch, setToolSearch] = useState('');
  const [toolCategory, setToolCategory] = useState('All');
  const [toolRemovalMessage, setToolRemovalMessage] = useState('');
  const missing = validateCustomAgentCreateForm(form);

  useEffect(() => {
    if (!drafting) {
      setDraftProgressStep(0);
      return undefined;
    }
    const timer = window.setInterval(() => {
      setDraftProgressStep((current) => current + 1);
    }, 1300);
    return () => window.clearInterval(timer);
  }, [drafting]);

  const update = (patch: Partial<CustomAgentFormData>) => onChange({ ...form, ...patch });

  const toggleTeam = (teamId: string) => {
    const selected = form.team_ids.includes(teamId);
    update({
      team_ids: selected
        ? form.team_ids.filter((id) => id !== teamId)
        : Array.from(new Set([...form.team_ids, teamId])),
    });
  };

  const toggleTarget = (target: AgentTargetType) => {
    const active = form.allowed_targets.includes(target);
    const nextTargets = active
      ? form.allowed_targets.filter((value) => value !== target)
      : [...form.allowed_targets, target];
    update({ allowed_targets: nextTargets });
  };

  const startBlank = () => {
    onChange({
      ...createDefaultCustomAgentForm(),
      ...form,
    });
    setDraftError('');
    setStarted(true);
  };

  const draftAgent = async () => {
    const description = draftDescription.trim();
    if (!description) {
      setDraftError('Describe what this agent should do first.');
      return;
    }
    setDrafting(true);
    setDraftProgressStep(0);
    setDraftError('');
    const res = await automationService.draftCustomAgent(workspaceId, { description });
    setDrafting(false);
    if (res.error || !res.data) {
      setDraftError(res.error || 'Could not draft this agent. You can still start blank.');
      return;
    }
    onChange(applyCustomAgentDraftToForm(form, res.data.draft));
    setStarted(true);
  };

  const addTool = (toolName: string) => {
    if (!toolName || form.allowed_tools.includes(toolName)) return;
    setToolRemovalMessage('');
    update({ allowed_tools: [...form.allowed_tools, toolName] });
    setToolPickerOpen(false);
    setToolSearch('');
  };

  const toggleSkill = (skill: SkillCatalogEntry) => {
    const active = form.skills.some((ref) => ref.key === skill.key);
    if (active) {
      update({ skills: form.skills.filter((ref) => ref.key !== skill.key) });
      return;
    }
    const ref: AgentSkillRef = { key: skill.key };
    if (skill.id) ref.skill_id = skill.id;
    if (skill.version_key) ref.version_key = skill.version_key;
    const requiredTools = skill.required_tools ?? [];
    update({
      skills: [...form.skills, ref],
      allowed_tools: Array.from(new Set([...form.allowed_tools, ...requiredTools])),
    });
  };

  const addSkill = (skillKey: string) => {
    const skill = skills.find((entry) => entry.key === skillKey);
    if (!skill || form.skills.some((ref) => ref.key === skill.key)) return;
    toggleSkill(skill);
    setSkillPickerOpen(false);
  };

  const updateRuntimeKind = (runtimeKind: AgentRuntimeKind) => {
    if (!CUSTOM_RUNTIME_KIND_OPTIONS.includes(runtimeKind)) return;
    update({
      runtime_kind: runtimeKind,
      supported_modes: supportedModesForRuntime(runtimeKind),
      default_invocation_mode: normalizeInvocationMode(form.default_invocation_mode, runtimeKind),
    });
  };

  const availableTools = tools.filter((tool) => !form.allowed_tools.includes(tool.name));
  const availableSkills = skills.filter((skill) => !form.skills.some((ref) => ref.key === skill.key));
  const supportedModes = supportedModesForRuntime(form.runtime_kind);
  const selectedSkills = form.skills
    .map((ref) => skills.find((skill) => skill.key === ref.key))
    .filter((skill): skill is SkillCatalogEntry => Boolean(skill));
  const skillRequiredTools = new Map<string, SkillCatalogEntry[]>();
  selectedSkills.forEach((skill) => {
    (skill.required_tools ?? []).forEach((toolName) => {
      const list = skillRequiredTools.get(toolName) ?? [];
      list.push(skill);
      skillRequiredTools.set(toolName, list);
    });
  });
  const toolCategories = ['All', ...Array.from(new Set(availableTools.map((tool) => tool.category).filter(Boolean))).sort()];
  const filteredTools = availableTools.filter((tool) => {
    const categoryMatches = toolCategory === 'All' || tool.category === toolCategory;
    const query = toolSearch.trim().toLowerCase();
    const searchMatches = query === '' || `${tool.name} ${tool.description} ${tool.category}`.toLowerCase().includes(query);
    return categoryMatches && searchMatches;
  });
  const removeTool = (toolName: string) => {
    const requiredBy = skillRequiredTools.get(toolName) ?? [];
    if (requiredBy.length > 0) {
      const skillNames = requiredBy.map((skill) => skill.title || skill.key).join(', ');
      setToolRemovalMessage(`${toolName} is required by ${skillNames}. Remove the skill first to remove this tool.`);
      return;
    }
    setToolRemovalMessage('');
    update({ allowed_tools: form.allowed_tools.filter((value) => value !== toolName) });
  };

  return (
    <TooltipProvider>
    <div className="flex min-h-0 flex-1 flex-col">
      <div className="border-b border-border/60 px-6 py-4">
        <div>
          <div>
            <h2 className="text-lg font-semibold">Create Custom Agent</h2>
          </div>
        </div>
      </div>

      <div className="min-h-0 flex-1 overflow-y-auto px-6 pb-28 pt-5">
        <div className="mx-auto max-w-4xl space-y-5">
          {!started ? (
            <div className="space-y-5">
              <div>
                <h3 className="text-base font-semibold">Describe the agent you want</h3>
                <p className="mt-1 text-sm text-muted-foreground">Share the outcome you want in a few sentences. AI will turn it into editable instructions and settings using this workspace's available tools and skills.</p>
              </div>
              <label className="block">
                <textarea
                  className="min-h-36 w-full rounded-md border border-input bg-background px-3 py-2 text-sm"
                  value={draftDescription}
                  onChange={(event) => setDraftDescription(event.target.value)}
                  placeholder="e.g. Review support conversations, draft helpful replies from docs, and escalate billing questions."
                  autoFocus
                />
              </label>
              {draftError ? (
                <p className="rounded-md border border-destructive/30 bg-destructive/10 px-3 py-2 text-xs text-destructive">
                  {draftError}
                </p>
              ) : null}
              <div className="space-y-3">
                <button
                  type="button"
                  className="rounded-md bg-primary px-3 py-2 text-sm font-medium text-primary-foreground disabled:opacity-50"
                  disabled={drafting || draftDescription.trim().length < 10}
                  onClick={() => void draftAgent()}
                >
                  {drafting ? DRAFT_PROGRESS_LABELS[draftProgressStep % DRAFT_PROGRESS_LABELS.length] : 'Generate agent setup'}
                </button>
                <p className="text-xs text-muted-foreground">
                  Prefer to configure every setting yourself?{' '}
                  <button
                    type="button"
                    className="font-medium text-foreground underline-offset-4 hover:underline"
                    onClick={startBlank}
                  >
                    Start blank
                  </button>
                </p>
              </div>
            </div>
          ) : null}

          {started ? (
            <>
          <div className="flex flex-wrap items-start justify-between gap-3">
            <div>
              <h3 className="text-base font-semibold">Agent settings</h3>
              <p className="mt-1 max-w-xl text-sm text-muted-foreground">Set the name, instructions, access, capabilities, and runtime options on one page.</p>
              {missing.length > 0 ? (
                <p className="mt-1 text-xs text-amber-700">Missing: {missing.join(', ')}</p>
              ) : null}
            </div>
            <div className="flex gap-2">
              <button
                type="button"
                className="rounded-md bg-primary px-3 py-2 text-sm font-medium text-primary-foreground disabled:opacity-50"
                disabled={saving || missing.length > 0}
                onClick={onCreate}
              >
                {saving ? 'Creating...' : 'Create agent'}
              </button>
            </div>
          </div>

          <section className="space-y-5 rounded-lg border border-border bg-card p-4">
            <div className="space-y-5">
              <label className="block space-y-2">
                <FieldLabel tooltip="This is the name people will see when choosing or running the agent.">Agent name</FieldLabel>
                <input
                  className="h-9 w-full rounded-md border border-input bg-background px-3 text-sm"
                  value={form.name}
                  onChange={(event) => update({ name: event.target.value })}
                  placeholder="e.g. Planning Helper"
                />
              </label>
              <label className="block space-y-2">
                <FieldLabel tooltip="Tell the agent how to behave, what good output looks like, and when it should ask for help.">Instructions</FieldLabel>
                <textarea
                  className="min-h-32 w-full rounded-md border border-input bg-background px-3 py-2 text-sm"
                  value={form.system_prompt}
                  onChange={(event) => update({ system_prompt: event.target.value })}
                  placeholder="Describe how this agent should work."
                />
              </label>
            </div>
          </section>

          <section className="space-y-5 rounded-lg border border-border bg-card p-4">
            <div>
              <h4 className="text-sm font-semibold">Capabilities</h4>
              <p className="mt-1 text-xs text-muted-foreground">Pick the tools and skills this agent can use.</p>
            </div>
            <div className="space-y-5">
              <label className="block space-y-3">
                <div>
                  <FieldLabel tooltip="Tools are the actions and data sources the agent is allowed to use at runtime. Leave empty to use the default tool set.">Tools</FieldLabel>
                  <p className="mt-1 text-xs text-muted-foreground">Choose what this agent can use. Leave empty to use the default tool set.</p>
                </div>
                {tools.length > 0 ? (
                  <div className="space-y-2">
                    <Popover open={toolPickerOpen} onOpenChange={setToolPickerOpen}>
                      <PopoverTrigger asChild>
                        <button
                          type="button"
                          className="flex h-9 w-full items-center justify-between rounded-md border border-input bg-background px-3 text-left text-sm"
                        >
                          <span>{form.allowed_tools.length > 0 ? `${form.allowed_tools.length} tool${form.allowed_tools.length === 1 ? '' : 's'} selected` : 'Select tools'}</span>
                          <span className="text-xs text-muted-foreground">Search</span>
                        </button>
                      </PopoverTrigger>
                      <PopoverContent
                        align="start"
                        className="w-[min(42rem,calc(100vw-3rem))] overflow-hidden p-0"
                        onWheelCapture={(event) => event.stopPropagation()}
                      >
                        <div className="border-b border-border p-3">
                          <input
                            className="h-9 w-full rounded-md border border-input bg-background px-3 text-sm"
                            value={toolSearch}
                            onChange={(event) => setToolSearch(event.target.value)}
                            placeholder="Search tools..."
                          />
                        </div>
                        <div className="grid h-80 min-h-0 grid-cols-[11rem_minmax(0,1fr)]">
                          <div className="min-h-0 overflow-y-auto overscroll-contain border-r border-border bg-muted/20 p-2">
                            {toolCategories.map((category) => (
                              <button
                                key={category}
                                type="button"
                                className={[
                                  'flex w-full items-center justify-between rounded-md px-2 py-1.5 text-left text-xs',
                                  toolCategory === category ? 'bg-background text-foreground shadow-sm' : 'text-muted-foreground hover:bg-background/70 hover:text-foreground',
                                ].join(' ')}
                                onClick={() => setToolCategory(category)}
                              >
                                <span className="truncate">{category}</span>
                                <span className="text-[10px]">
                                  {category === 'All'
                                    ? availableTools.length
                                    : availableTools.filter((tool) => tool.category === category).length}
                                </span>
                              </button>
                            ))}
                          </div>
                          <div className="min-h-0 overflow-y-auto overscroll-contain p-2">
                            {filteredTools.length > 0 ? (
                              <div className="space-y-1.5">
                                {filteredTools.map((tool) => (
                                  <button
                                    key={tool.name}
                                    type="button"
                                    className="w-full rounded-md px-2 py-2 text-left hover:bg-muted/50"
                                    onClick={() => addTool(tool.name)}
                                  >
                                    <span className="flex items-center gap-2">
                                      <span className="font-mono text-xs text-foreground">{tool.name}</span>
                                      <Badge variant="outline" className="text-[10px]">{tool.category}</Badge>
                                    </span>
                                    <span className="mt-1 block text-xs leading-relaxed text-muted-foreground">{tool.description}</span>
                                  </button>
                                ))}
                              </div>
                            ) : (
                              <p className="px-2 py-8 text-center text-sm text-muted-foreground">No tools found.</p>
                            )}
                          </div>
                        </div>
                      </PopoverContent>
                    </Popover>
                    {form.allowed_tools.length > 0 ? (
                      <>
                        <div className="flex flex-wrap gap-1.5">
                          {form.allowed_tools.map((toolName) => {
                            const requiredBy = skillRequiredTools.get(toolName) ?? [];
                            return (
                              <button
                                key={toolName}
                                type="button"
                                className="rounded-md border border-border bg-card px-2 py-1 text-xs hover:bg-muted/40"
                                onClick={() => removeTool(toolName)}
                              >
                                {toolName} x
                                {requiredBy.length > 0 ? (
                                  <span className="ml-1 text-muted-foreground">
                                    required by {requiredBy.map((skill) => skill.title || skill.key).join(', ')}
                                  </span>
                                ) : null}
                              </button>
                            );
                          })}
                        </div>
                        {toolRemovalMessage ? (
                          <p className="rounded-md border border-amber-500/30 bg-amber-500/10 px-3 py-2 text-xs text-amber-800 dark:text-amber-300">
                            {toolRemovalMessage}
                          </p>
                        ) : null}
                      </>
                    ) : null}
                  </div>
                ) : (
                  <p className="rounded-md border border-border bg-card px-3 py-2 text-xs text-muted-foreground">Tool catalog is still loading.</p>
                )}
              </label>
              <label className="block space-y-3">
                <div>
                  <FieldLabel tooltip="Skills attach reusable behavior instructions. If a skill requires tools, those tools are added automatically.">Skills</FieldLabel>
                  <p className="mt-1 text-xs text-muted-foreground">Attach reusable instructions. Required tools are added automatically.</p>
                </div>
                {skills.length > 0 ? (
                  <div className="space-y-2">
                    <Popover open={skillPickerOpen} onOpenChange={setSkillPickerOpen}>
                      <PopoverTrigger asChild>
                        <button
                          type="button"
                          className="flex h-9 w-full items-center justify-between rounded-md border border-input bg-background px-3 text-left text-sm"
                        >
                          <span>{form.skills.length > 0 ? `${form.skills.length} skill${form.skills.length === 1 ? '' : 's'} selected` : 'Select skills'}</span>
                          <span className="text-xs text-muted-foreground">Search</span>
                        </button>
                      </PopoverTrigger>
                      <PopoverContent align="start" className="w-[min(32rem,calc(100vw-3rem))] overflow-hidden p-0">
                        <Command>
                          <CommandInput placeholder="Search skills..." />
                          <CommandList>
                            <CommandEmpty>No skills found.</CommandEmpty>
                            <CommandGroup heading={`${availableSkills.length} available skills`}>
                              {availableSkills.map((skill) => (
                                <CommandItem
                                  key={skill.key}
                                  value={`${skill.key} ${skill.title} ${skill.description}`}
                                  onSelect={() => addSkill(skill.key)}
                                  className="cursor-pointer items-start py-2"
                                >
                                  <div className="min-w-0 flex-1">
                                    <div className="flex items-center gap-2">
                                      <span className="text-sm font-medium">{skill.title || skill.key}</span>
                                      <Badge variant="outline" className="text-[10px]">{skill.source_kind === 'built_in' ? 'built-in' : skill.source_kind}</Badge>
                                    </div>
                                    <p className="mt-1 text-xs leading-relaxed text-muted-foreground">{skill.description}</p>
                                    {(skill.required_tools?.length ?? 0) > 0 ? (
                                      <p className="mt-1 font-mono text-[10px] text-muted-foreground">Adds: {skill.required_tools?.join(', ')}</p>
                                    ) : null}
                                  </div>
                                </CommandItem>
                              ))}
                            </CommandGroup>
                          </CommandList>
                        </Command>
                      </PopoverContent>
                    </Popover>
                    {form.skills.length > 0 ? (
                      <div className="flex flex-wrap gap-1.5">
                        {form.skills.map((ref) => {
                          const skill = skills.find((entry) => entry.key === ref.key);
                          return (
                            <button
                              key={ref.key}
                              type="button"
                              className="rounded-md border border-border bg-card px-2 py-1 text-xs hover:bg-muted/40"
                              onClick={() => skill ? toggleSkill(skill) : update({ skills: form.skills.filter((item) => item.key !== ref.key) })}
                            >
                              {skill?.title || ref.key} x
                            </button>
                          );
                        })}
                      </div>
                    ) : null}
                  </div>
                ) : (
                  <p className="rounded-md border border-border bg-card px-3 py-2 text-xs text-muted-foreground">No skills available yet.</p>
                )}
              </label>
            </div>
          </section>

          <section className="space-y-5 rounded-lg border border-border bg-card p-4">
            <div>
              <h4 className="text-sm font-semibold">Access and work area</h4>
              <p className="mt-1 text-xs text-muted-foreground">Choose where this agent can be started and who can use it.</p>
            </div>
            <div className="space-y-5">
              <div className="grid gap-2 md:grid-cols-2">
                {TARGET_OPTIONS.map((target) => {
                  const active = form.allowed_targets.includes(target.value);
                  return (
                    <button
                      key={target.value}
                      type="button"
                      className={[
                        'rounded-md border px-3 py-2 text-left text-sm',
                        active ? 'border-primary bg-primary/10' : 'border-border bg-card',
                      ].join(' ')}
                      onClick={() => toggleTarget(target.value)}
                    >
                      {target.label}
                    </button>
                  );
                })}
              </div>
              <div className="space-y-3">
                <FieldLabel tooltip="Workspace-wide agents can work across the workspace. Specific teams narrows who can see and run the agent.">Team access</FieldLabel>
                <div className="grid gap-2 md:grid-cols-2">
                  <button
                    type="button"
                    className={[
                      'rounded-md border px-3 py-3 text-left',
                      form.teamAccessMode === 'all_teams' ? 'border-primary bg-primary/10' : 'border-border bg-card',
                    ].join(' ')}
                    onClick={() => update({ teamAccessMode: 'all_teams', team_ids: [] })}
                  >
                    <span className="block text-sm font-medium">All teams</span>
                    <span className="mt-1 block text-xs text-muted-foreground">Workspace-wide access.</span>
                  </button>
                  <button
                    type="button"
                    className={[
                      'rounded-md border px-3 py-3 text-left',
                      form.teamAccessMode === 'specific_teams' ? 'border-primary bg-primary/10' : 'border-border bg-card',
                    ].join(' ')}
                    onClick={() => update({ teamAccessMode: 'specific_teams' })}
                  >
                    <span className="block text-sm font-medium">Specific teams</span>
                    <span className="mt-1 block text-xs text-muted-foreground">Only selected teams can use it.</span>
                  </button>
                </div>
                {form.teamAccessMode === 'specific_teams' ? (
                  <div className="flex flex-wrap gap-1.5 rounded-md border border-border bg-muted/20 p-2">
                    {teams.map((team) => {
                      const selected = form.team_ids.includes(team.id);
                      return (
                        <button
                          key={team.id}
                          type="button"
                          className={[
                            'rounded-md border px-2 py-1 text-xs',
                            selected ? 'border-primary bg-primary/10 text-foreground' : 'border-border bg-background text-muted-foreground',
                          ].join(' ')}
                          onClick={() => toggleTeam(team.id)}
                        >
                          {team.name}
                        </button>
                      );
                    })}
                  </div>
                ) : null}
              </div>
            </div>
          </section>

          <section className="space-y-3 rounded-lg border border-border bg-card p-4">
            <button
              type="button"
              className="flex w-full items-center justify-between text-left"
              onClick={() => setApprovalOpen((open) => !open)}
            >
              <span>
                <span className="block text-sm font-semibold">Run approval</span>
                <span className="mt-1 block text-xs text-muted-foreground">Require approval before each run starts. Tool-requested checkpoints still appear separately during a run.</span>
              </span>
              <span className="text-xs text-muted-foreground">{approvalOpen ? 'Hide' : 'Show'}</span>
            </button>
            {approvalOpen ? (
              <label className="block max-w-sm space-y-2 border-t border-border pt-3">
                <FieldLabel tooltip="Controls the initial approval gate for each agent run, not individual tool or checkpoint approvals.">Run approval</FieldLabel>
                <Select
                  value={form.approval_mode}
                  onValueChange={(value) => update({ approval_mode: value as AgentApprovalMode })}
                >
                  <SelectTrigger className="h-9">
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value="always">Require approval before each run</SelectItem>
                    <SelectItem value="never">Start runs automatically</SelectItem>
                    <SelectItem value="preset_default">Use runtime default</SelectItem>
                  </SelectContent>
                </Select>
              </label>
            ) : null}
          </section>

          <section className="space-y-3 rounded-lg border border-border bg-card p-4">
            <button
              type="button"
              className="flex w-full items-center justify-between text-left"
              onClick={() => setAdvancedSettingsOpen((open) => !open)}
            >
              <span>
                <span className="block text-sm font-semibold">Advanced settings</span>
                <span className="mt-1 block text-xs text-muted-foreground">Runtime, model, parallel tasks, and token limits.</span>
              </span>
              <span className="text-xs text-muted-foreground">{advancedSettingsOpen ? 'Hide' : 'Show'}</span>
            </button>
            {advancedSettingsOpen ? (
              <div className="grid gap-3 border-t border-border pt-3 md:grid-cols-2">
                    <label className="block space-y-2">
                      <FieldLabel tooltip="Execution engine for this agent. These options match the original custom-agent form.">Runtime</FieldLabel>
                      <Select value={form.runtime_kind} onValueChange={(value) => updateRuntimeKind(value as AgentRuntimeKind)}>
                        <SelectTrigger className="h-9">
                          <SelectValue />
                        </SelectTrigger>
                        <SelectContent>
                          {CUSTOM_RUNTIME_KIND_OPTIONS.map((runtimeKind) => (
                            <SelectItem key={runtimeKind} value={runtimeKind}>
                              {AGENT_RUNTIME_LABELS[runtimeKind]}
                            </SelectItem>
                          ))}
                        </SelectContent>
                      </Select>
                    </label>
                    <label className="block space-y-2">
                      <FieldLabel tooltip="Autonomous runs work in the background. Interactive runs can ask follow-up questions during execution.">Run mode</FieldLabel>
                      <Select value={form.default_invocation_mode} onValueChange={(value) => update({ default_invocation_mode: value as AgentInvocationMode })}>
                        <SelectTrigger className="h-9">
                          <SelectValue />
                        </SelectTrigger>
                        <SelectContent>
                          {supportedModes.map((mode) => (
                            <SelectItem key={mode} value={mode}>
                              {INVOCATION_MODE_LABELS[mode]}
                            </SelectItem>
                          ))}
                        </SelectContent>
                      </Select>
                    </label>
                    <label className="block space-y-2">
                      <FieldLabel tooltip="The AI service that powers this agent. The list only includes providers configured for this workspace/server.">AI Provider</FieldLabel>
                      <Select value={form.provider} onValueChange={(value) => update({ provider: value as AgentModelProvider, model: '' })}>
                        <SelectTrigger className="h-9">
                          <SelectValue />
                        </SelectTrigger>
                        <SelectContent>
                          {providerOptions.map((provider) => (
                            <SelectItem key={provider.value} value={provider.value}>
                              {provider.label}
                            </SelectItem>
                          ))}
                        </SelectContent>
                      </Select>
                    </label>
                    <label className="block space-y-2">
                      <FieldLabel tooltip="Leave blank for Auto. Manual model names are provider-specific and are not validated until the agent is saved or run.">Model</FieldLabel>
                      <input className="h-9 w-full rounded-md border border-input bg-background px-3 text-sm" value={form.model} onChange={(event) => update({ model: event.target.value })} placeholder="Auto" />
                    </label>
                    <label className="block space-y-2">
                      <FieldLabel tooltip="Coming soon: this will limit how many runs this agent can work on at the same time. It is saved as 1 today.">Parallel tasks</FieldLabel>
                      <div className="flex items-center gap-2">
                        <input className="h-9 w-full rounded-md border border-input bg-muted/40 px-3 text-sm text-muted-foreground" value="1" disabled readOnly />
                        <Badge variant="outline" className="shrink-0 text-[10px]">Coming soon</Badge>
                      </div>
                    </label>
                    <label className="block space-y-2">
                      <FieldLabel tooltip="Coming soon. Monthly token limits are not enforced for custom agents yet.">Monthly token limit</FieldLabel>
                      <div className="flex items-center gap-2">
                        <input className="h-9 w-full rounded-md border border-input bg-muted/40 px-3 text-sm text-muted-foreground" value="Coming soon" disabled readOnly />
                        <Badge variant="outline" className="shrink-0 text-[10px]">Coming soon</Badge>
                      </div>
                    </label>
                  </div>
            ) : null}
          </section>
            </>
          ) : null}
        </div>
      </div>
    </div>
    </TooltipProvider>
  );
}
