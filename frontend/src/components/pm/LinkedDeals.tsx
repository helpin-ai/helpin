import { useState, useCallback } from 'react';
import { DollarSign, Link2, Search, X } from 'lucide-react';
import { Button } from '@/components/ui/button';
import { Popover, PopoverContent, PopoverTrigger } from '@/components/ui/popover';
import { Input } from '@/components/ui/input';
import {
  useEpicAssociations,
  useStoryAssociations,
  useCreateAssociationFromPM,
  useDeleteAssociationFromPM,
} from '@/hooks/queries';
import { crmSearchService } from '@/lib/services/crmService';
import type { CRMSearchResult, CRMAssociationEnriched } from '@/lib/crmTypes';

interface LinkedDealsProps {
  objectType: 'epic' | 'story';
  objectId: string;
  workspaceId: string;
}

export function LinkedDeals({ objectType, objectId, workspaceId }: LinkedDealsProps) {
  const { data: associations = [] } = objectType === 'epic'
    ? useEpicAssociations(workspaceId, objectId)
    : useStoryAssociations(workspaceId, objectId);

  const createAssoc = useCreateAssociationFromPM(workspaceId);
  const deleteAssoc = useDeleteAssociationFromPM(workspaceId);

  const dealAssociations = associations.filter((a: CRMAssociationEnriched) => {
    if (a.from_object_type === objectType && a.from_object_id === objectId) return a.to_object_type === 'deal';
    if (a.to_object_type === objectType && a.to_object_id === objectId) return a.from_object_type === 'deal';
    return false;
  });

  const [open, setOpen] = useState(false);
  const [query, setQuery] = useState('');
  const [results, setResults] = useState<CRMSearchResult[]>([]);
  const [searching, setSearching] = useState(false);

  const search = useCallback(
    async (q: string) => {
      if (q.length < 2) {
        setResults([]);
        return;
      }
      setSearching(true);
      const res = await crmSearchService.search(workspaceId, q);
      const deals = (res.data ?? []).filter((r) => r.type === 'deal');
      setResults(deals);
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

  const handleLink = async (result: CRMSearchResult) => {
    await createAssoc.mutateAsync({
      workspace_id: workspaceId,
      from_object_type: objectType,
      from_object_id: objectId,
      to_object_type: 'deal',
      to_object_id: result.id,
    });
    setOpen(false);
    setQuery('');
    setResults([]);
  };

  const handleUnlink = (assocId: string) => {
    deleteAssoc.mutate(assocId);
  };

  return (
    <div className="mt-6">
      <div className="flex items-center justify-between mb-2">
        <h4 className="text-xs font-semibold uppercase tracking-wide text-muted-foreground">Linked Deals</h4>
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
                placeholder="Search deals..."
                value={query}
                onChange={(e) => handleSearchInput(e.target.value)}
                className="h-7 border-0 shadow-none text-xs focus-visible:ring-0"
                autoFocus
              />
            </div>
            <div className="max-h-48 overflow-y-auto space-y-0.5">
              {searching && <p className="text-xs text-muted-foreground p-2">Searching...</p>}
              {!searching && query.length >= 2 && results.length === 0 && (
                <p className="text-xs text-muted-foreground p-2">No deals found</p>
              )}
              {results.map((r) => (
                <button
                  key={r.id}
                  type="button"
                  className="w-full flex items-center gap-2 rounded px-2 py-1.5 text-xs hover:bg-accent cursor-pointer text-left"
                  onClick={() => handleLink(r)}
                >
                  <DollarSign className="h-3 w-3 text-muted-foreground shrink-0" />
                  <div className="min-w-0">
                    <span className="font-medium truncate block">{r.name}</span>
                    {r.detail && <span className="text-muted-foreground truncate block">{r.detail}</span>}
                  </div>
                </button>
              ))}
            </div>
          </PopoverContent>
        </Popover>
      </div>

      {dealAssociations.length === 0 ? (
        <p className="text-xs text-muted-foreground">No linked deals</p>
      ) : (
        <ul className="space-y-1.5">
          {dealAssociations.map((assoc: CRMAssociationEnriched) => {
            const name = assoc.linked_object_name;
            const displayId = assoc.linked_object_display_id;
            return (
              <li key={assoc.id} className="flex items-center justify-between group rounded px-2 py-1 hover:bg-accent/50">
                <div className="flex items-center gap-2 min-w-0">
                  <DollarSign className="h-3 w-3 text-muted-foreground shrink-0" />
                  <div className="min-w-0">
                    <span className="text-xs font-medium truncate block">{name || 'Deal'}</span>
                    {displayId && <span className="text-[10px] text-muted-foreground">{displayId}</span>}
                  </div>
                </div>
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
