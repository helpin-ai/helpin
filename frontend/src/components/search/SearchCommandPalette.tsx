import { useCallback, useEffect, useRef, useState } from 'react';
import { useNavigate } from '@tanstack/react-router';
import {
  BookOpen,
  CircleDot,
  Crosshair,
  FileText,
  Loader2,
  Target,
  User,
} from 'lucide-react';
import {
  CommandDialog,
  CommandEmpty,
  CommandGroup,
  CommandInput,
  CommandItem,
  CommandList,
} from '@/components/ui/command';
import { useWorkspaceStore } from '@/stores/workspaceStore';
import {
  searchService,
  type SearchResponse,
  type SearchResult,
} from '@/lib/services/searchService';

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
  const [query, setQuery] = useState('');
  const [results, setResults] = useState<SearchResponse>(EMPTY);
  const [searching, setSearching] = useState(false);
  const timerRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  const abortRef = useRef<AbortController | null>(null);

  // Reset state when dialog closes
  useEffect(() => {
    if (!open) {
      setQuery('');
      setResults(EMPTY);
      setSearching(false);
    }
  }, [open]);

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

  const totalResults =
    results.tasks.length +
    results.epics.length +
    results.sprints.length +
    results.objectives.length +
    results.members.length +
    (results.documents?.length ?? 0);

  const slug = workspace?.slug ?? '';
  const taskResults = results.tasks;

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

  return (
    <CommandDialog
      open={open}
      onOpenChange={onOpenChange}
      title="Search"
      description={`Search across ${workspace?.name ?? 'workspace'}`}
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
            <Loader2 className="h-4 w-4 animate-spin" />
            Searching...
          </div>
        )}

        {!searching && query.trim() && totalResults === 0 && (
          <CommandEmpty>No results found.</CommandEmpty>
        )}

        {!searching && !query.trim() && (
          <div className="py-6 text-center text-sm text-muted-foreground">
            Start typing to search...
          </div>
        )}

        {taskResults.length > 0 && (
          <CommandGroup heading="Tasks">
            {taskResults.map((item) => (
              <CommandItem
                key={item.id}
                value={`task-${item.id}-${item.name}`}
                onSelect={() => handleSelect('task', item)}
                className="cursor-pointer"
              >
                <CircleDot className="h-4 w-4 text-blue-500" />
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
                <BookOpen className="h-4 w-4 text-purple-500" />
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
                <Crosshair className="h-4 w-4 text-green-500" />
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
                <Target className="h-4 w-4 text-orange-500" />
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
                <FileText className="h-4 w-4 text-blue-400" />
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
                <User className="h-4 w-4 text-cyan-500" />
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
