import { useCallback, useEffect, useRef, useState } from 'react';
import { useNavigate } from '@tanstack/react-router';
import { toast } from 'sonner';
import {
  BookOpen01Icon,
  BotIcon,
  Briefcase01Icon,
  File01Icon,
  FolderKanbanIcon,
  Loading01Icon,
  Message01Icon,
  RecordIcon,
  SentIcon,
  Target01Icon,
  Target02Icon,
  UserIcon,
} from '@/lib/icons';
import {
  CommandDialog,
  CommandEmpty,
  CommandGroup,
  CommandInput,
  CommandItem,
  CommandList,
} from '@/components/ui/command';
import { useWorkspaceStore } from '@/stores/workspaceStore';
import { SETTINGS_ROUTE_SECTIONS } from '@/lib/settingsSections';
import {
  searchService,
  type SearchResponse,
  type SearchResult,
} from '@/lib/services/searchService';
import { commandBarService } from '@/lib/services/commandBarService';
import { cn } from '@/lib/utils';
import { buildTaskCommandValue } from '@/components/search/searchCommandPalette';
import { usePageContext } from '@/components/command-bar/pageContext';
import { PageContextBadge } from '@/components/command-bar/PageContextBadge';
import { StepToolPicker } from '@/components/command-bar/StepToolPicker';
import { useCommandBarRunStore } from '@/stores/commandBarStore';
import type { CommandBarPageContext, CommandBarParseResponse } from '@/lib/pmTypes';

const EMPTY: SearchResponse = {
  tasks: [],
  epics: [],
  sprints: [],
  objectives: [],
  members: [],
  documents: [],
};

export function SearchCommandPalette({
  open,
  onOpenChange,
  initialQuery,
}: {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  initialQuery?: string;
}) {
  const navigate = useNavigate();
  const workspace = useWorkspaceStore((s) => s.currentWorkspace);
  const resolvedPageContext = usePageContext();
  const [contextOverride, setContextOverride] = useState<CommandBarPageContext | null>(null);
  const pageContext = contextOverride ?? resolvedPageContext;
  const addRuns = useCommandBarRunStore((s) => s.addRuns);
  const addPlan = useCommandBarRunStore((s) => s.addPlan);
  const [query, setQuery] = useState('');
  const [results, setResults] = useState<SearchResponse>(EMPTY);
  const [searching, setSearching] = useState(false);
  const [parsing, setParsing] = useState(false);
  const [dispatching, setDispatching] = useState(false);
  const [intentResult, setIntentResult] = useState<CommandBarParseResponse | null>(null);
  const [stepToolOverrides, setStepToolOverrides] = useState<Record<number, string[] | undefined>>({});
  const timerRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  const abortRef = useRef<AbortController | null>(null);

  // Reset state when dialog closes
  useEffect(() => {
    if (!open) {
      setQuery('');
      setResults(EMPTY);
      setSearching(false);
      setParsing(false);
      setDispatching(false);
      setIntentResult(null);
      setStepToolOverrides({});
      setContextOverride(null);
    }
  }, [open]);

  useEffect(() => {
    if (open && initialQuery?.trim()) {
      setQuery(initialQuery.trim());
    }
  }, [open, initialQuery]);

  useEffect(() => {
    setIntentResult(null);
    setStepToolOverrides({});
  }, [query]);

  // Debounced search
  useEffect(() => {
    if (!workspace?.id || !query.trim()) {
      setResults(EMPTY);
      setSearching(false);
      return;
    }

    setSearching(true);
    clearTimeout(timerRef.current ?? undefined);
    abortRef.current?.abort();

    timerRef.current = setTimeout(async () => {
      const controller = new AbortController();
      abortRef.current = controller;
      const { data, error } = await searchService.search(workspace.id, query.trim());
      if (controller.signal.aborted) return;
      if (error || !data) {
        setResults(EMPTY);
      } else {
        setResults(data);
      }
      setSearching(false);
    }, 400);

    return () => {
      clearTimeout(timerRef.current ?? undefined);
      abortRef.current?.abort();
    };
  }, [query, workspace?.id]);

  const slug = workspace?.slug ?? '';
  const taskResults = results.tasks;
  const trimmedQuery = query.trim();
  const canAskAgents = !!workspace?.id && !!pageContext && trimmedQuery.length >= 4;

  const quickNavItems = [
    { label: 'Projects', icon: FolderKanbanIcon, path: `/w/${slug}/pm/my-work` },
    { label: 'CRM', icon: Briefcase01Icon, path: `/w/${slug}/crm/contacts` },
    { label: 'Support', icon: Message01Icon, path: `/w/${slug}/support` },
    { label: 'Docs', icon: File01Icon, path: `/w/${slug}/docs` },
    { label: 'Automation', icon: BotIcon, path: `/w/${slug}/automation/flows` },
  ];

  const settingsNavItems = SETTINGS_ROUTE_SECTIONS.filter(
    (s) => s.sidebar !== false,
  ).map((s) => ({
    label: s.label,
    icon: s.icon,
    path: `/w/${slug}/settings/${s.id}`,
  }));

  type EntityType = 'task' | 'epic' | 'sprint' | 'objective' | 'member' | 'document';

  const handleSelect = useCallback(
    (type: EntityType, item: SearchResult) => {
      onOpenChange(false);
      switch (type) {
        case 'task':
          window.location.assign(`/w/${slug}/pm/tasks?task=${item.task_key || item.display_id}`);
          break;
        case 'epic':
          navigate({ to: '/w/$slug/pm/epics/$epicId', params: { slug, epicId: item.id } });
          break;
        case 'sprint':
          navigate({ to: '/w/$slug/pm/sprints/$sprintId', params: { slug, sprintId: item.id } });
          break;
        case 'objective':
          navigate({
            to: '/w/$slug/pm/objectives/$objectiveId',
            params: { slug, objectiveId: item.id },
          });
          break;
        case 'member':
          navigate({ to: '/w/$slug/settings/$section', params: { slug, section: 'members' } });
          break;
        case 'document':
          navigate({ to: '/w/$slug/docs/documents/$docId', params: { slug, docId: item.id } });
          break;
      }
    },
    [navigate, slug, onOpenChange],
  );

  const handleParseIntent = useCallback(async () => {
    if (!workspace?.id || !pageContext || !trimmedQuery) return;
    setParsing(true);
    setIntentResult(null);
    try {
      const res = await commandBarService.parseIntent(workspace.id, {
        text: trimmedQuery,
        page_context: pageContext,
      });
      if (res.error || !res.data) {
        toast.error(res.error ?? 'Failed to parse command');
        return;
      }
      setIntentResult(res.data);
    } finally {
      setParsing(false);
    }
  }, [pageContext, trimmedQuery, workspace?.id]);

  const handleDispatchPlan = useCallback(async () => {
    if (!workspace?.id || !pageContext || !intentResult || intentResult.status !== 'plan') return;
    const emptyStepIndex = intentResult.plan.steps.findIndex((_, i) => {
      const override = stepToolOverrides[i];
      return override !== undefined && override.length === 0;
    });
    if (emptyStepIndex !== -1) {
      toast.error(`Step ${emptyStepIndex + 1} has no tools enabled. Pick at least one tool or reset to all.`);
      return;
    }
    setDispatching(true);
    try {
      const stepsWithOverrides = intentResult.plan.steps.map((step, index) => {
        const override = stepToolOverrides[index];
        if (override === undefined) return step;
        return { ...step, allowed_tools: override };
      });
      const res = await commandBarService.dispatchPlan(workspace.id, {
        text: trimmedQuery,
        page_context: pageContext,
        steps: stepsWithOverrides,
      });
      if (res.error || !res.data) {
        toast.error(res.error ?? 'Failed to start command run');
        return;
      }
      const steps = (res.data.steps ?? intentResult.plan.steps).map((step, index) => {
        const override = stepToolOverrides[index];
        if (override === undefined) return step;
        return { ...step, allowed_tools: override };
      });
      if (res.data.plan_id) {
        addPlan({
          id: res.data.plan_id,
          steps,
          runIdsByStep: Object.fromEntries(res.data.runs.map((run, index) => [index, run.id])),
          planKind: intentResult.plan.plan_kind,
          status: 'running',
          prompt: trimmedQuery,
          currentStepIndex: 0,
        }, res.data.runs);
      } else {
        addRuns(res.data.runs);
      }
      toast.success(steps.length > 1 ? `Started step 1 of ${steps.length}` : 'Agent run started');
      onOpenChange(false);
    } finally {
      setDispatching(false);
    }
  }, [addPlan, addRuns, intentResult, onOpenChange, pageContext, stepToolOverrides, trimmedQuery, workspace?.id]);

  // ⌘↵ confirms the plan from anywhere in the palette while it's open
  useEffect(() => {
    if (!open) return;
    const handler = (e: KeyboardEvent) => {
      if ((e.metaKey || e.ctrlKey) && e.key === 'Enter' && intentResult?.status === 'plan') {
        e.preventDefault();
        void handleDispatchPlan();
      }
    };
    window.addEventListener('keydown', handler);
    return () => window.removeEventListener('keydown', handler);
  }, [open, intentResult, handleDispatchPlan]);

  return (
    <CommandDialog
      open={open}
      onOpenChange={onOpenChange}
      title="Search"
      description={`Search across ${workspace?.name ?? 'workspace'}`}
      className="self-start justify-self-center border-border/70 shadow-xl sm:mt-[12vh] sm:max-w-2xl"
      showCloseButton={false}
    >
      {pageContext ? (
        <PageContextBadge
          context={pageContext}
          onClear={
            resolvedPageContext && resolvedPageContext.entity_type !== 'workspace' && contextOverride?.entity_type !== 'workspace'
              ? () =>
                  setContextOverride(
                    workspace
                      ? { entity_type: 'workspace', entity_id: workspace.id, display_title: workspace.name }
                      : null,
                  )
              : undefined
          }
          onNavigate={() => onOpenChange(false)}
        />
      ) : null}
      <CommandInput
        placeholder={pageContext && pageContext.entity_type !== 'workspace' ? 'Search or ask agents about this...' : 'Search or ask agents...'}
        value={query}
        onValueChange={setQuery}
      />
      <CommandList>
        {searching && (
          <div className="flex items-center justify-center gap-2 py-6 text-sm text-muted-foreground">
            <Loading01Icon className="h-4 w-4 animate-spin" />
            Searching...
          </div>
        )}

        <CommandEmpty>No results found.</CommandEmpty>

        {canAskAgents && (
          <CommandGroup heading="Agents">
            <CommandItem
              value={`ask-agents-${trimmedQuery}`}
              onSelect={() => void handleParseIntent()}
              className="cursor-pointer"
            >
              {parsing ? <Loading01Icon className="h-4 w-4 animate-spin text-muted-foreground" /> : <SentIcon className="h-4 w-4 text-muted-foreground" />}
              <span className="truncate">Ask agents: {trimmedQuery}</span>
            </CommandItem>
          </CommandGroup>
        )}

        {intentResult?.status === 'plan' && (
          <>
            <div className="px-2 pt-2 pb-1">
              <div className="flex items-center justify-between gap-2 text-[11px] font-medium uppercase tracking-wider text-muted-foreground">
                <span>{intentResult.plan.plan_kind === 'fan_out' ? 'Fan-out plan' : intentResult.plan.plan_kind === 'one_shot_command' ? 'One-shot agent' : 'Plan'}</span>
                <span className="font-normal normal-case tracking-normal text-muted-foreground/80">
                  {intentResult.plan.plan_kind === 'fan_out'
                    ? `${intentResult.plan.steps.length} targets`
                    : intentResult.plan.plan_kind === 'one_shot_command'
                    ? 'not saved'
                    : intentResult.plan.steps.length === 1
                      ? '1 step'
                      : `${intentResult.plan.steps.length} steps`}
                </span>
              </div>
              {intentResult.rationale ? (
                <p className="mt-0.5 line-clamp-1 text-[11px] text-muted-foreground/80">{intentResult.rationale}</p>
              ) : null}
              {intentResult.plan.guardrails?.length ? (
                <div className="mt-1.5 space-y-1">
                  {intentResult.plan.guardrails.map((guardrail, index) => (
                    <div key={index} className="flex items-center gap-1.5 text-[11px] text-muted-foreground">
                      <Target01Icon className="h-3 w-3" />
                      <span>{guardrail.message}</span>
                    </div>
                  ))}
                </div>
              ) : null}
              <div className={cn('mt-2', intentResult.plan.steps.length > 1 && 'relative pl-3')}>
                {intentResult.plan.steps.length > 1 ? (
                  <div aria-hidden className="absolute left-[8px] top-3 bottom-3 w-px bg-border/70" />
                ) : null}
                <div className="space-y-1.5">
                  {intentResult.plan.steps.map((step, index) => {
                    const candidate = intentResult.candidates?.find((c) => c.id === step.agent_id);
                    const availableTools = candidate?.allowed_tools ?? step.allowed_tools ?? [];
                    const override = stepToolOverrides[index];
                    const narrowed = override !== undefined && override.length !== availableTools.length && override.length > 0;
                    const empty = override !== undefined && override.length === 0;
                    const oneShotBrief = step.plan_kind === 'one_shot_command' ? parseOneShotBrief(step.instructions) : null;
                    return (
                      <div key={`${step.agent_id}-${index}`} className="relative">
                        {intentResult.plan.steps.length > 1 ? (
                          <span
                            aria-hidden
                            className="absolute -left-3 top-2 grid h-4 w-4 place-items-center rounded-full bg-background ring-2 ring-border/70 text-[9px] font-semibold text-muted-foreground"
                          >
                            {index + 1}
                          </span>
                        ) : null}
                        <div className="rounded-md border border-border/60 bg-muted/30 px-2.5 py-2">
                          {oneShotBrief ? (
                            <OneShotBriefCard
                              brief={oneShotBrief}
                              targetLabel={`${step.target.entity_type.replace('_', ' ')} · ${step.target.display_title || step.target.entity_id}`}
                            />
                          ) : (
                            <p className="text-sm font-medium text-foreground line-clamp-2">{step.instructions}</p>
                          )}
                          <div className="mt-1 flex items-center gap-1.5 text-[11px] text-muted-foreground">
                            <BotIcon className="h-3 w-3" />
                            <span className="truncate">{step.agent_name}</span>
                            {step.plan_kind === 'one_shot_command' ? (
                              <>
                                <span className="opacity-60">·</span>
                                <span>one-shot</span>
                              </>
                            ) : step.plan_kind === 'fan_out' ? (
                              <>
                                <span className="opacity-60">·</span>
                                <span>fan-out target {index + 1}</span>
                              </>
                            ) : null}
                            <span className="opacity-60">·</span>
                            <span>{step.target.entity_type.replace('_', ' ')}</span>
                            {narrowed ? (
                              <span className="ml-1 rounded border border-amber-500/30 bg-amber-500/10 px-1 text-[9px] font-medium uppercase tracking-wider text-amber-700 dark:text-amber-400">
                                tools narrowed
                              </span>
                            ) : null}
                            {empty ? (
                              <span className="ml-1 rounded border border-destructive/30 bg-destructive/10 px-1 text-[9px] font-medium uppercase tracking-wider text-destructive">
                                no tools
                              </span>
                            ) : null}
                          </div>
                          {availableTools.length > 0 ? (
                            <div className="mt-1.5">
                              <StepToolPicker
                                workspaceId={workspace?.id}
                                agentId={step.agent_id}
                                agentName={step.agent_name}
                                availableTools={availableTools}
                                selectedTools={stepToolOverrides[index]}
                                onChange={(next) =>
                                  setStepToolOverrides((prev) => ({ ...prev, [index]: next }))
                                }
                                disabled={dispatching}
                              />
                            </div>
                          ) : null}
                        </div>
                      </div>
                    );
                  })}
                </div>
              </div>
            </div>
            <CommandGroup>
              {(() => {
                const hasEmptyStep = intentResult.plan.steps.some((_, i) => {
                  const override = stepToolOverrides[i];
                  return override !== undefined && override.length === 0;
                });
                const runCount = intentResult.plan.estimated_runs ?? intentResult.plan.run_count;
                const sequencingHint = intentResult.plan.plan_kind === 'fan_out'
                  ? ' · fan-out'
                  : intentResult.plan.steps.length > 1
                    ? ' · sequential'
                    : '';
                return (
                  <CommandItem
                    value={`confirm-agent-plan-${trimmedQuery}`}
                    onSelect={() => void handleDispatchPlan()}
                    disabled={hasEmptyStep || dispatching}
                    className="cursor-pointer"
                  >
                    {dispatching ? <Loading01Icon className="h-4 w-4 animate-spin text-muted-foreground" /> : <SentIcon className="h-4 w-4 text-muted-foreground" />}
                    <span className="flex-1">
                      {dispatching
                        ? 'Starting runs...'
                        : hasEmptyStep
                          ? 'Pick at least 1 tool per step to confirm'
                          : `Confirm ${runCount === 1 ? '1 run' : `${runCount} runs`}${sequencingHint}`}
                    </span>
                    {!dispatching && !hasEmptyStep ? (
                      <kbd className="ml-2 rounded border bg-muted px-1.5 py-0.5 text-[10px] font-mono text-muted-foreground">⌘↵</kbd>
                    ) : null}
                  </CommandItem>
                );
              })()}
            </CommandGroup>
          </>
        )}

        {intentResult?.status === 'no_matching_agent' && (
          <CommandGroup heading="No matching agent">
            <CommandItem value={`no-match-${trimmedQuery}-${intentResult.reason}`} disabled>
              <BotIcon className="h-4 w-4 text-muted-foreground" />
              <div className="min-w-0 text-sm">
                <p className="font-medium">No available agent can do that yet.</p>
                <p className="mt-1 text-xs text-muted-foreground">{intentResult.reason}</p>
                {intentResult.suggestions?.length ? (
                  <div className="mt-3 flex flex-wrap gap-1.5">
                    {intentResult.suggestions.map((suggestion) => (
                      <span key={suggestion} className="rounded border px-2 py-1 text-[11px] text-muted-foreground">
                        {suggestion}
                      </span>
                    ))}
                  </div>
                ) : null}
              </div>
            </CommandItem>
          </CommandGroup>
        )}

        <CommandGroup heading="Go to">
          {quickNavItems.map((item) => (
            <CommandItem
              key={item.label}
              value={`nav-${item.label}`}
              onSelect={() => {
                onOpenChange(false);
                navigate({ to: item.path });
              }}
              className="cursor-pointer"
            >
              <item.icon className="h-4 w-4 text-muted-foreground" />
              <span>{item.label}</span>
            </CommandItem>
          ))}
        </CommandGroup>
        <CommandGroup heading="Settings">
          {settingsNavItems.map((item) => (
            <CommandItem
              key={item.label}
              value={`settings-${item.label}`}
              onSelect={() => {
                onOpenChange(false);
                navigate({ to: item.path });
              }}
              className="cursor-pointer"
            >
              <item.icon className="h-4 w-4 text-muted-foreground" />
              <span>{item.label}</span>
            </CommandItem>
          ))}
        </CommandGroup>

        {taskResults.length > 0 && (
          <CommandGroup heading="Tasks">
            {taskResults.map((item) => (
              <CommandItem
                key={item.id}
                value={buildTaskCommandValue(item)}
                onSelect={() => handleSelect('task', item)}
                className="cursor-pointer"
              >
                <RecordIcon className="h-4 w-4 text-blue-500" />
                <span className="text-muted-foreground text-xs font-mono mr-1">
                  {item.task_key || `#${item.display_id}`}
                </span>
                <span className="truncate">{item.name}</span>
              </CommandItem>
            ))}
          </CommandGroup>
        )}

        {results.epics.length > 0 && (
          <CommandGroup heading="Epics">
            {results.epics.map((item) => (
              <CommandItem
                key={item.id}
                value={`epic-${item.id}-${item.name}`}
                onSelect={() => handleSelect('epic', item)}
                className="cursor-pointer"
              >
                <BookOpen01Icon className="h-4 w-4 text-purple-500" />
                <span className="truncate">{item.name}</span>
              </CommandItem>
            ))}
          </CommandGroup>
        )}

        {results.sprints.length > 0 && (
          <CommandGroup heading="Sprints">
            {results.sprints.map((item) => (
              <CommandItem
                key={item.id}
                value={`sprint-${item.id}-${item.name}`}
                onSelect={() => handleSelect('sprint', item)}
                className="cursor-pointer"
              >
                <Target02Icon className="h-4 w-4 text-green-500" />
                <span className="truncate">{item.name}</span>
              </CommandItem>
            ))}
          </CommandGroup>
        )}

        {results.objectives.length > 0 && (
          <CommandGroup heading="Objectives">
            {results.objectives.map((item) => (
              <CommandItem
                key={item.id}
                value={`objective-${item.id}-${item.name}`}
                onSelect={() => handleSelect('objective', item)}
                className="cursor-pointer"
              >
                <Target01Icon className="h-4 w-4 text-orange-500" />
                <span className="truncate">{item.name}</span>
              </CommandItem>
            ))}
          </CommandGroup>
        )}

        {(results.documents?.length ?? 0) > 0 && (
          <CommandGroup heading="Documents">
            {results.documents.map((item) => (
              <CommandItem
                key={item.id}
                value={`document-${item.id}-${item.name}`}
                onSelect={() => handleSelect('document', item)}
                className="cursor-pointer"
              >
                <File01Icon className="h-4 w-4 text-blue-400" />
                <span className="truncate">{item.name}</span>
              </CommandItem>
            ))}
          </CommandGroup>
        )}

        {results.members.length > 0 && (
          <CommandGroup heading="Members">
            {results.members.map((item) => (
              <CommandItem
                key={item.id}
                value={`member-${item.id}-${item.name}`}
                onSelect={() => handleSelect('member', item)}
                className="cursor-pointer"
              >
                <UserIcon className="h-4 w-4 text-cyan-500" />
                <span className="truncate">{item.name}</span>
                {item.team_name && (
                  <span className="ml-auto text-xs text-muted-foreground truncate">
                    {item.team_name}
                  </span>
                )}
              </CommandItem>
            ))}
          </CommandGroup>
        )}
      </CommandList>

      <div className="border-t px-3 py-2 text-xs text-muted-foreground flex items-center gap-3">
        <span>
          <kbd className="rounded border bg-muted px-1.5 py-0.5 text-[10px] font-mono">↑↓</kbd>{' '}
          Navigate
        </span>
        {intentResult?.status === 'plan' ? (
          <span>
            <kbd className="rounded border bg-muted px-1.5 py-0.5 text-[10px] font-mono">⌘↵</kbd>{' '}
            Confirm
          </span>
        ) : (
          <span>
            <kbd className="rounded border bg-muted px-1.5 py-0.5 text-[10px] font-mono">↵</kbd>{' '}
            Open
          </span>
        )}
        <span>
          <kbd className="rounded border bg-muted px-1.5 py-0.5 text-[10px] font-mono">Esc</kbd>{' '}
          Close
        </span>
      </div>
    </CommandDialog>
  );
}

interface OneShotBrief {
  goal: string;
  plan: string[];
  constraints: string[];
}

function OneShotBriefCard({ brief, targetLabel }: { brief: OneShotBrief; targetLabel: string }) {
  return (
    <div className="space-y-2">
      <div className="flex items-start justify-between gap-3">
        <div className="min-w-0">
          <div className="flex items-center gap-1.5 text-[11px] font-medium uppercase tracking-wider text-muted-foreground">
            <Target02Icon className="h-3 w-3" />
            Execution brief
          </div>
          <p className="mt-1 text-sm font-medium leading-5 text-foreground">{brief.goal}</p>
        </div>
      </div>
      {brief.plan.length ? (
        <div className="rounded-md border border-border/60 bg-background/70 px-2 py-1.5">
          <div className="mb-1 text-[10px] font-medium uppercase tracking-wider text-muted-foreground">Plan</div>
          <ol className="space-y-1">
            {brief.plan.slice(0, 5).map((item, itemIndex) => (
              <li key={`${itemIndex}-${item}`} className="grid grid-cols-[16px_1fr] gap-1.5 text-[11px] leading-4 text-muted-foreground">
                <span className="text-right tabular-nums text-muted-foreground/70">{itemIndex + 1}</span>
                <span>{item}</span>
              </li>
            ))}
          </ol>
        </div>
      ) : null}
      <div className="flex flex-wrap items-center gap-1.5 text-[11px] text-muted-foreground">
        <span className="rounded border border-border/70 bg-background/80 px-1.5 py-0.5">{targetLabel}</span>
        {brief.constraints.slice(0, 2).map((constraint) => (
          <span key={constraint} className="rounded border border-amber-500/25 bg-amber-500/10 px-1.5 py-0.5 text-amber-700 dark:text-amber-400">
            {constraint}
          </span>
        ))}
      </div>
    </div>
  );
}

function parseOneShotBrief(instructions: string): OneShotBrief {
  const sections: Record<'goal' | 'plan' | 'constraints', string[]> = {
    goal: [],
    plan: [],
    constraints: [],
  };
  let current: keyof typeof sections | null = null;
  for (const rawLine of instructions.split('\n')) {
    const line = rawLine.trim();
    if (!line || line === 'One-shot execution brief') continue;
    const lower = line.toLowerCase();
    if (lower === 'goal:') {
      current = 'goal';
      continue;
    }
    if (lower === 'plan:') {
      current = 'plan';
      continue;
    }
    if (lower === 'constraints:') {
      current = 'constraints';
      continue;
    }
    if (lower.startsWith('target:') || lower.startsWith('user request:')) {
      current = null;
      continue;
    }
    if (!current) continue;
    sections[current].push(line.replace(/^[-*]\s+/, '').replace(/^\d+\.\s+/, ''));
  }
  return {
    goal: sections.goal.join(' ') || 'Complete the confirmed one-shot command.',
    plan: sections.plan,
    constraints: sections.constraints,
  };
}
