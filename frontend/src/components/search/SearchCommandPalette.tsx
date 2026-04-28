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
import { buildTaskCommandValue } from '@/components/search/searchCommandPalette';
import { usePageContext } from '@/components/command-bar/pageContext';
import { useCommandBarRunStore } from '@/stores/commandBarStore';
import type { CommandBarParseResponse } from '@/lib/pmTypes';

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
}: {
  open: boolean;
  onOpenChange: (open: boolean) => void;
}) {
  const navigate = useNavigate();
  const workspace = useWorkspaceStore((s) => s.currentWorkspace);
  const pageContext = usePageContext();
  const addRuns = useCommandBarRunStore((s) => s.addRuns);
  const [query, setQuery] = useState('');
  const [results, setResults] = useState<SearchResponse>(EMPTY);
  const [searching, setSearching] = useState(false);
  const [parsing, setParsing] = useState(false);
  const [dispatching, setDispatching] = useState(false);
  const [intentResult, setIntentResult] = useState<CommandBarParseResponse | null>(null);
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
    }
  }, [open]);

  useEffect(() => {
    setIntentResult(null);
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
    setDispatching(true);
    try {
      const res = await commandBarService.dispatchPlan(workspace.id, {
        text: trimmedQuery,
        page_context: pageContext,
        steps: intentResult.plan.steps,
      });
      if (res.error || !res.data) {
        toast.error(res.error ?? 'Failed to start command run');
        return;
      }
      addRuns(res.data.runs);
      toast.success(res.data.runs.length === 1 ? 'Agent run started' : `${res.data.runs.length} agent runs started`);
      onOpenChange(false);
    } finally {
      setDispatching(false);
    }
  }, [addRuns, intentResult, onOpenChange, pageContext, trimmedQuery, workspace?.id]);

  return (
    <CommandDialog
      open={open}
      onOpenChange={onOpenChange}
      title="Search"
      description={`Search across ${workspace?.name ?? 'workspace'}`}
      className="self-start justify-self-center border-border/70 shadow-xl sm:mt-[12vh] sm:max-w-2xl"
      showCloseButton={false}
    >
      <CommandInput
        placeholder={`Search ${workspace?.name ?? 'workspace'}...`}
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
          <CommandGroup heading="Plan">
            {intentResult.plan.steps.map((step, index) => (
              <CommandItem
                key={`${step.agent_id}-${index}`}
                value={`plan-${trimmedQuery}-${step.agent_name}-${step.instructions}-${index}`}
                onSelect={() => void handleDispatchPlan()}
                className="cursor-pointer"
              >
                <BotIcon className="h-4 w-4 text-muted-foreground" />
                <div className="min-w-0 flex-1">
                  <div className="flex items-center gap-2">
                    <span className="truncate text-sm font-medium">{step.agent_name}</span>
                    <span className="rounded border px-1.5 py-0.5 text-[10px] text-muted-foreground">{step.target.entity_type}</span>
                  </div>
                  <p className="truncate text-xs text-muted-foreground">{step.instructions}</p>
                </div>
              </CommandItem>
            ))}
            <CommandItem
              value={`confirm-agent-plan-${trimmedQuery}`}
              onSelect={() => void handleDispatchPlan()}
              className="cursor-pointer"
            >
              {dispatching ? <Loading01Icon className="h-4 w-4 animate-spin text-muted-foreground" /> : <SentIcon className="h-4 w-4 text-muted-foreground" />}
              <span>{dispatching ? 'Starting run...' : `Confirm ${intentResult.plan.run_count} run`}</span>
            </CommandItem>
          </CommandGroup>
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
        <span>
          <kbd className="rounded border bg-muted px-1.5 py-0.5 text-[10px] font-mono">↵</kbd>{' '}
          Open
        </span>
        <span>
          <kbd className="rounded border bg-muted px-1.5 py-0.5 text-[10px] font-mono">Esc</kbd>{' '}
          Close
        </span>
      </div>
    </CommandDialog>
  );
}
