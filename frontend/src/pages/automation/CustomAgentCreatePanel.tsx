import { useEffect, useRef, useState, type ReactNode } from 'react';
import type { WorkspaceTeam } from '@/lib/types';
import { Badge } from '@/components/ui/badge';
import { AgentAvatar } from '@/components/agents/AgentAvatar';
import { AgentIconPicker } from '@/components/agents/AgentIconPicker';
import { QuietSearchInput } from '@/components/design-system/quiet';
import { HelpCircleIcon } from '@/lib/icons';
import { AGENT_MODEL_TIER_OPTIONS } from '@/lib/agentModelTier';
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
  AgentInvocationMode,
  AgentModelTier,
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
  { value: 'sprint', label: 'Sprints' },
  { value: 'objective', label: 'Objectives' },
  { value: 'crm_deal', label: 'CRM deals' },
  { value: 'document', label: 'Docs' },
  { value: 'support_conversation', label: 'Support' },
  { value: 'workspace', label: 'Workspace' },
  { value: 'repository', label: 'Code repo' },
];

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

function skillIdentity(skill: Pick<SkillCatalogEntry, 'id' | 'key'> | AgentSkillRef) {
  if ('id' in skill && skill.id) return skill.id;
  if ('skill_id' in skill && skill.skill_id) return skill.skill_id;
  return skill.key;
}

function skillDisplayName(skill: Pick<SkillCatalogEntry, 'title' | 'key'> | undefined, fallbackKey: string) {
  return skill?.title?.trim() || fallbackKey;
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

function RequiredMark() {
  return <span aria-hidden="true" className="text-destructive">*</span>;
}

export interface CustomAgentCreatePanelProps {
  workspaceId: string;
  form: CustomAgentFormData;
  onChange: (form: CustomAgentFormData) => void;
  teams: WorkspaceTeam[];
  tools: ToolCatalogEntry[];
  skills: SkillCatalogEntry[];
  advancedOpen: boolean;
  onAdvancedOpenChange: (open: boolean) => void;
  onCreate: () => void;
  saving: boolean;
  mode?: 'create' | 'edit';
  canSave?: boolean;
  onCancel?: () => void;
  onDelete?: () => void;
  statusText?: string;
}

export function CustomAgentCreatePanel({
  workspaceId,
  form,
  onChange,
  teams,
  tools,
  skills,
  advancedOpen,
  onAdvancedOpenChange,
  onCreate,
  saving,
  mode = 'create',
  canSave,
  onCancel,
  onDelete,
  statusText,
}: CustomAgentCreatePanelProps) {
  const isEditMode = mode === 'edit';
  const [started, setStarted] = useState(isEditMode);
  const [draftDescription, setDraftDescription] = useState('');
  const [drafting, setDrafting] = useState(false);
  const [draftProgressStep, setDraftProgressStep] = useState(0);
  const [draftError, setDraftError] = useState('');
  const [toolPickerOpen, setToolPickerOpen] = useState(false);
  const [skillPickerOpen, setSkillPickerOpen] = useState(false);
  const [toolSearch, setToolSearch] = useState('');
  const [toolCategory, setToolCategory] = useState('All');
  const [toolRemovalMessage, setToolRemovalMessage] = useState('');
  const latestFormRef = useRef(form);
  const draftRequestRef = useRef(0);
  const nameInputRef = useRef<HTMLInputElement | null>(null);
  const missing = validateCustomAgentCreateForm(form);

  useEffect(() => {
    latestFormRef.current = form;
  }, [form]);

  useEffect(() => {
    if (isEditMode) {
      setStarted(true);
      window.requestAnimationFrame(() => {
        nameInputRef.current?.focus({ preventScroll: true });
      });
    }
  }, [isEditMode]);

  useEffect(() => () => {
    draftRequestRef.current += 1;
  }, []);

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
    const requestId = draftRequestRef.current + 1;
    draftRequestRef.current = requestId;
    const res = await automationService.draftCustomAgent(workspaceId, { description });
    if (requestId !== draftRequestRef.current) {
      return;
    }
    setDrafting(false);
    if (res.error || !res.data) {
      setDraftError(res.error || 'Could not draft this agent. You can still start blank.');
      return;
    }
    onChange(applyCustomAgentDraftToForm(latestFormRef.current, res.data.draft));
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
    const identity = skillIdentity(skill);
    const active = form.skills.some((ref) => skillIdentity(ref) === identity);
    if (active) {
      update({ skills: form.skills.filter((ref) => skillIdentity(ref) !== identity) });
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

  const addSkill = (identity: string) => {
    const skill = skills.find((entry) => skillIdentity(entry) === identity);
    if (!skill || form.skills.some((ref) => skillIdentity(ref) === skillIdentity(skill))) return;
    toggleSkill(skill);
    setSkillPickerOpen(false);
  };

  const availableTools = tools.filter((tool) => !form.allowed_tools.includes(tool.name));
  const selectedSkillIdentities = new Set(form.skills.map(skillIdentity));
  const availableSkills = skills.filter((skill) => !selectedSkillIdentities.has(skillIdentity(skill)));
  const supportedModes = form.supported_modes;
  const selectedSkills = form.skills
    .map((ref) => skills.find((skill) => skillIdentity(skill) === skillIdentity(ref)))
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
  const saveDisabled = saving || missing.length > 0 || canSave === false;
  const primaryLabel = saving ? (isEditMode ? 'Saving...' : 'Creating...') : isEditMode ? 'Save changes' : 'Create agent';
  const validationStatus = missing.length > 0
      ? `Missing: ${missing.join(', ')}`
      : '';
  const actionStatus = validationStatus ? '' : statusText;
  const actionTooltip = saveDisabled && actionStatus ? actionStatus : '';
  const actionControls = started ? (
    <div className="flex flex-wrap items-center justify-end gap-2">
      {actionStatus ? (
        <p className="max-w-72 text-right text-xs text-muted-foreground">
          {actionStatus}
        </p>
      ) : null}
      {onDelete ? (
        <button
          type="button"
          className="rounded-md border border-destructive/40 px-3 py-2 text-sm font-medium text-destructive hover:bg-destructive/10 disabled:opacity-50"
          disabled={saving}
          onClick={onDelete}
        >
          Delete
        </button>
      ) : null}
      {onCancel ? (
        <button
          type="button"
          className="rounded-md border border-input bg-background px-3 py-2 text-sm font-medium hover:bg-muted/60 disabled:opacity-50"
          disabled={saving}
          onClick={onCancel}
        >
          Cancel
        </button>
      ) : null}
      <Tooltip>
        <TooltipTrigger asChild>
          <span className="inline-flex">
            <button
              type="button"
              className="rounded-md bg-primary px-3 py-2 text-sm font-medium text-primary-foreground disabled:opacity-50"
              disabled={saveDisabled}
              onClick={onCreate}
            >
              {primaryLabel}
            </button>
          </span>
        </TooltipTrigger>
        {actionTooltip || validationStatus ? (
          <TooltipContent side="bottom" className="max-w-64 text-xs">
            {validationStatus || actionTooltip}
          </TooltipContent>
        ) : null}
      </Tooltip>
    </div>
  ) : null;

  return (
    <TooltipProvider>
    <div className="flex min-h-0 flex-1 flex-col">
      <div className="border-b border-border/60 bg-muted/20 py-4 pl-6 pr-14">
        <div className="flex flex-wrap items-center justify-between gap-3">
          <div className="flex items-center gap-3">
            <AgentAvatar iconKey={form.icon_key} className="h-10 w-10 rounded-none border-0 bg-transparent shadow-none" genericBare />
            <div>
              <h2 className="text-lg font-semibold">{isEditMode ? (form.name.trim() || 'Custom Agent') : 'Create Custom Agent'}</h2>
              <p className="mt-0.5 text-xs text-muted-foreground">
                {isEditMode ? 'Custom agent' : 'Define a reusable workspace agent'}
              </p>
            </div>
          </div>
          {actionControls}
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
          <div>
            <div>
              <h3 className="text-base font-semibold">Agent settings</h3>
            </div>
          </div>

          <section className="space-y-5 rounded-lg border border-border bg-card p-4">
            <div className="space-y-5">
              <div className="space-y-2">
                <FieldLabel tooltip="Choose the avatar shown anywhere this custom agent appears.">
                  Agent icon
                </FieldLabel>
                <AgentIconPicker value={form.icon_key} onValueChange={(iconKey) => update({ icon_key: iconKey })} />
              </div>
              <label className="block space-y-2">
                <FieldLabel tooltip="This is the name people will see when choosing or running the agent.">
                  Agent name <RequiredMark />
                </FieldLabel>
                <input
                  ref={nameInputRef}
                  className="h-9 w-full rounded-md border border-input bg-background px-3 text-sm"
                  value={form.name}
                  onChange={(event) => update({ name: event.target.value })}
                  placeholder="e.g. Planning Helper"
                />
              </label>
              <label className="block space-y-2">
                <FieldLabel tooltip="Tell the agent how to behave, what good output looks like, and when it should ask for help.">
                  Instructions <RequiredMark />
                </FieldLabel>
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
              <h4 className="flex items-center gap-1 text-sm font-semibold">Capabilities <RequiredMark /></h4>
              <p className="mt-1 text-xs text-muted-foreground">Pick at least one tool or skill this agent can use.</p>
            </div>
            <div className="flex flex-col gap-5">
              <label className="order-2 block space-y-3">
                <div>
                  <FieldLabel tooltip="Tools are the actions and data sources the agent is allowed to call.">Tools</FieldLabel>
                  <p className="mt-1 text-xs text-muted-foreground">Choose what this agent can use.</p>
                </div>
                {tools.length > 0 ? (
                  <div className="space-y-2">
                    <Popover open={toolPickerOpen} onOpenChange={setToolPickerOpen}>
                      <PopoverTrigger asChild>
                        <button
                          type="button"
                          className="flex h-9 w-full max-w-sm items-center rounded-md border border-input bg-background px-3 text-left text-sm"
                        >
                          <span>{form.allowed_tools.length > 0 ? `${form.allowed_tools.length} tool${form.allowed_tools.length === 1 ? '' : 's'} selected` : 'Select tools'}</span>
                        </button>
                      </PopoverTrigger>
                      <PopoverContent
                        align="start"
                        className="w-[min(42rem,calc(100vw-3rem))] overflow-hidden p-0"
                        onWheelCapture={(event) => event.stopPropagation()}
                      >
                        <div className="border-b border-border p-3">
                          <QuietSearchInput
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
                          {form.allowed_tools.map((toolName) => (
                            <button
                              key={toolName}
                              type="button"
                              className="group rounded-md border border-border bg-card px-2 py-1 text-xs hover:bg-muted/40"
                              onClick={() => removeTool(toolName)}
                            >
                              <span>{toolName}</span>
                              <span className="ml-2 text-muted-foreground group-hover:text-destructive">x</span>
                            </button>
                          ))}
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
              <label className="order-1 block space-y-3">
                <div>
                  <FieldLabel tooltip="Available skills are reusable guides the agent can discover and read during a run. If a skill requires tools, those tools are added automatically.">Skills</FieldLabel>
                  <p className="mt-1 text-xs text-muted-foreground">Attach reusable instructions. Required tools are added automatically.</p>
                </div>
                {skills.length > 0 ? (
                  <div className="space-y-2">
                    <Popover open={skillPickerOpen} onOpenChange={setSkillPickerOpen}>
                      <PopoverTrigger asChild>
                        <button
                          type="button"
                          className="flex h-9 w-full max-w-sm items-center rounded-md border border-input bg-background px-3 text-left text-sm"
                        >
                          <span>{form.skills.length > 0 ? `${form.skills.length} skill${form.skills.length === 1 ? '' : 's'} selected` : 'Select skills'}</span>
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
                                  key={skillIdentity(skill)}
                                  value={`${skill.key} ${skill.title} ${skill.description}`}
                                  onSelect={() => addSkill(skillIdentity(skill))}
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
                          const skill = skills.find((entry) => skillIdentity(entry) === skillIdentity(ref));
                          return (
                            <button
                              key={skillIdentity(ref)}
                              type="button"
                              className="group rounded-md border border-border bg-card px-2 py-1 text-xs hover:bg-muted/40"
                              onClick={() => skill ? toggleSkill(skill) : update({ skills: form.skills.filter((item) => skillIdentity(item) !== skillIdentity(ref)) })}
                            >
                              <span>{skillDisplayName(skill, ref.key)}</span>
                              <span className="ml-2 text-muted-foreground group-hover:text-destructive">x</span>
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
              <div className="space-y-2">
                <FieldLabel tooltip="Working areas control where this agent appears as a runnable option.">
                  Working areas <RequiredMark />
                </FieldLabel>
                <div className="grid gap-2 sm:grid-cols-2 md:grid-cols-3 lg:grid-cols-4">
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
              </div>
              <div className="space-y-3">
                <FieldLabel tooltip="Choose who can see this agent in pickers and start runs. The agent's tools and skills control what it can do after it starts.">Who can use this agent</FieldLabel>
                <div className="grid gap-2 sm:grid-cols-2 sm:max-w-md">
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
            <div>
              <span className="flex items-center gap-1.5 text-sm font-semibold">
                Run mode
                <Tooltip>
                  <TooltipTrigger asChild>
                    <span className="rounded-sm text-muted-foreground/70 hover:text-foreground" aria-label="Run mode help">
                      <HelpCircleIcon className="h-3.5 w-3.5" />
                    </span>
                  </TooltipTrigger>
                  <TooltipContent side="right" className="max-w-64 text-xs leading-relaxed">
                    Autonomous runs work in the background. Interactive runs can ask follow-up questions during execution.
                  </TooltipContent>
                </Tooltip>
              </span>
            </div>
            <div className="block max-w-sm border-t border-border pt-3">
              <Select
                value={form.default_invocation_mode}
                onValueChange={(value) => update({ default_invocation_mode: value as AgentInvocationMode })}
              >
                <SelectTrigger className="h-9" aria-label="Run mode">
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
              <p className="mt-2 text-xs text-muted-foreground">
                {form.default_invocation_mode === 'interactive'
                  ? 'Can ask follow-up questions or request approval.'
                  : 'Runs autonomously end-to-end.'}
              </p>
            </div>
          </section>

          <section className="space-y-3 rounded-lg border border-border bg-card p-4">
            <button
              type="button"
              className="flex w-full items-center justify-between text-left"
              onClick={() => onAdvancedOpenChange(!advancedOpen)}
            >
              <span>
                <span className="block text-sm font-semibold">Advanced settings</span>
                <span className="mt-1 block text-xs text-muted-foreground">Model size and task limits.</span>
              </span>
              <span className="text-xs text-muted-foreground">{advancedOpen ? 'Hide' : 'Show'}</span>
            </button>
            {advancedOpen ? (
              <div className="grid gap-3 border-t border-border pt-3 md:grid-cols-2">
                    <label className="block space-y-2">
                      <FieldLabel tooltip="Model sizes map to Helpin-managed models and billing rates. The underlying provider and model may change without changing this agent version.">Model size</FieldLabel>
                      <Select value={form.model_tier} onValueChange={(value) => update({ model_tier: value as AgentModelTier })}>
                        <SelectTrigger className="h-9" aria-label="Model size">
                          <SelectValue />
                        </SelectTrigger>
                        <SelectContent>
                          {AGENT_MODEL_TIER_OPTIONS.map((tier) => (
                            <SelectItem key={tier.value} value={tier.value}>{tier.label}</SelectItem>
                          ))}
                        </SelectContent>
                      </Select>
                      <p className="text-[11px] leading-relaxed text-muted-foreground">
                        {AGENT_MODEL_TIER_OPTIONS.find((tier) => tier.value === form.model_tier)?.description}
                      </p>
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
