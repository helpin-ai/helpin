import { useState, useCallback } from 'react';
import { useNavigate } from '@tanstack/react-router';
import { Hexagon, BookOpen, Link2, Search, X } from 'lucide-react';
import { Button } from '@/components/ui/button';
import { Popover, PopoverContent, PopoverTrigger } from '@/components/ui/popover';
import { Input } from '@/components/ui/input';
import { useCreateAssociation, useDeleteAssociation } from '@/hooks/queries';
import { api } from '@/lib/api';
import type { CRMAssociationEnriched } from '@/lib/crmTypes';

interface SearchResult {
  id: string;
  type: string;
  title: string;
  display_id?: string;
}

interface LinkedPMItemsProps {
  dealId: string;
  workspaceId: string;
  workspaceSlug: string;
  associations: CRMAssociationEnriched[];
}

export function LinkedPMItems({ dealId, workspaceId, workspaceSlug, associations }: LinkedPMItemsProps) {
  const navigate = useNavigate();
  const createAssoc = useCreateAssociation(workspaceId);
  const deleteAssoc = useDeleteAssociation(workspaceId);

  const pmAssociations = associations.filter((a) => {
    if (a.from_object_type === 'deal' && a.from_object_id === dealId) {
      return a.to_object_type === 'epic' || a.to_object_type === 'story';
    }
    if (a.to_object_type === 'deal' && a.to_object_id === dealId) {
      return a.from_object_type === 'epic' || a.from_object_type === 'story';
    }
    return false;
  });

  const getLinkedType = (a: CRMAssociationEnriched) => {
    if (a.from_object_type === 'deal') return a.to_object_type;
    return a.from_object_type;
  };

  const getLinkedId = (a: CRMAssociationEnriched) => {
    if (a.from_object_type === 'deal') return a.to_object_id;
    return a.from_object_id;
  };

  const [open, setOpen] = useState(false);
  const [query, setQuery] = useState('');
  const [results, setResults] = useState<SearchResult[]>([]);
  const [searching, setSearching] = useState(false);

  const search = useCallback(
    async (q: string) => {
      if (q.length < 2) {
        setResults([]);
        return;
      }
      setSearching(true);
      const res = await api.get<SearchResult[]>(`/search?workspace_id=${encodeURIComponent(workspaceId)}&q=${encodeURIComponent(q)}`);
      const items = (res.data ?? []).filter((r) => r.type === 'epic' || r.type === 'story');
      setResults(items);
      setSearching(false);
    },
    [workspaceId],
  );

  const handleSearchInput = useCallback(
    (value: string) => {
      setQuery(value);
      const timeout = setTimeout(() => search(value), 300);
      return () => clearTimeout(timeout);
    },
    [search],
  );

  const handleLink = async (result: SearchResult) => {
    await createAssoc.mutateAsync({
      workspace_id: workspaceId,
      from_object_type: 'deal',
      from_object_id: dealId,
      to_object_type: result.type as 'epic' | 'story',
      to_object_id: result.id,
    });
    setOpen(false);
    setQuery('');
    setResults([]);
  };

  const handleUnlink = (assocId: string) => {
    deleteAssoc.mutate(assocId);
  };

  const navigateTo = (type: string, id: string) => {
    if (type === 'epic') {
      navigate({ to: '/w/$slug/pm/epics/$epicId', params: { slug: workspaceSlug, epicId: id } });
    }
    // Stories don't have a standalone page, so no navigation for them
  };

  return (
    <div>
      <div className="flex items-center justify-between mb-2">
        <h4 className="text-xs font-semibold uppercase tracking-wide text-muted-foreground">Epics & Stories</h4>
        <Popover open={open} onOpenChange={setOpen}>
          <PopoverTrigger asChild>
            <Button variant="ghost" size="sm" className="h-6 px-2 text-xs">
              <Link2 className="mr-1 h-3 w-3" />
              Link
            </Button>
          </PopoverTrigger>
          <PopoverContent className="w-72 p-2" align="end">
            <div className="flex items-center gap-1 border-b pb-2 mb-2">
              <Search className="h-3.5 w-3.5 text-muted-foreground" />
              <Input
                placeholder="Search epics & stories..."
                value={query}
                onChange={(e) => handleSearchInput(e.target.value)}
                className="h-7 border-0 shadow-none text-xs focus-visible:ring-0"
                autoFocus
              />
            </div>
            <div className="max-h-48 overflow-y-auto space-y-0.5">
              {searching && <p className="text-xs text-muted-foreground p-2">Searching...</p>}
              {!searching && query.length >= 2 && results.length === 0 && (
                <p className="text-xs text-muted-foreground p-2">No results found</p>
              )}
              {results.map((r) => (
                <button
                  key={r.id}
                  type="button"
                  className="w-full flex items-center gap-2 rounded px-2 py-1.5 text-xs hover:bg-accent cursor-pointer text-left"
                  onClick={() => handleLink(r)}
                >
                  {r.type === 'epic' ? (
                    <Hexagon className="h-3 w-3 text-muted-foreground shrink-0" />
                  ) : (
                    <BookOpen className="h-3 w-3 text-muted-foreground shrink-0" />
                  )}
                  <div className="min-w-0">
                    <span className="font-medium truncate block">{r.title}</span>
                    {r.display_id && <span className="text-muted-foreground">{r.display_id}</span>}
                  </div>
                </button>
              ))}
            </div>
          </PopoverContent>
        </Popover>
      </div>

      {pmAssociations.length === 0 ? (
        <p className="text-xs text-muted-foreground">No linked epics or stories</p>
      ) : (
        <ul className="space-y-1.5">
          {pmAssociations.map((assoc) => {
            const linkedType = getLinkedType(assoc);
            const linkedId = getLinkedId(assoc);
            const name = assoc.linked_object_name;
            const displayId = assoc.linked_object_display_id;
            return (
              <li key={assoc.id} className="flex items-center justify-between group rounded px-2 py-1 hover:bg-accent/50">
                <button
                  type="button"
                  className="flex items-center gap-2 min-w-0 cursor-pointer text-left"
                  onClick={() => navigateTo(linkedType, linkedId)}
                >
                  {linkedType === 'epic' ? (
                    <Hexagon className="h-3 w-3 text-muted-foreground shrink-0" />
                  ) : (
                    <BookOpen className="h-3 w-3 text-muted-foreground shrink-0" />
                  )}
                  <div className="min-w-0">
                    <span className="text-xs font-medium truncate block">{name || linkedType}</span>
                    {displayId && <span className="text-[10px] text-muted-foreground">{displayId}</span>}
                  </div>
                </button>
                <button
                  type="button"
                  className="opacity-0 group-hover:opacity-100 transition-opacity p-0.5 hover:text-destructive cursor-pointer"
                  onClick={() => handleUnlink(assoc.id)}
                >
                  <X className="h-3 w-3" />
                </button>
              </li>
            );
          })}
        </ul>
      )}
    </div>
  );
}
