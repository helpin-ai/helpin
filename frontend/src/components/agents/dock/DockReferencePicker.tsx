import { forwardRef, useEffect, useImperativeHandle, useRef, useState } from 'react';
import { PlusSignIcon } from '@/lib/icons';
import { Popover, PopoverContent, PopoverTrigger } from '@/components/ui/popover';
import { QuietSearchInput } from '@/components/design-system/quiet';
import {
  entityTypeLabel,
  searchDocsEntityItems,
  type DocsEntitySearchItem,
  type DocsEntitySearchType,
} from '@/components/docs/entitySearch';
import type { DockEntityReference } from '@/lib/dockTypes';

const REFERENCE_TYPES: DocsEntitySearchType[] = ['task', 'document', 'epic', 'contact', 'deal'];

function referenceType(type: DocsEntitySearchType): DockEntityReference['entity_type'] | null {
  if (type === 'task' || type === 'story') return 'task';
  if (type === 'document' || type === 'epic') return type;
  if (type === 'contact') return 'crm_contact';
  if (type === 'deal') return 'crm_deal';
  return null;
}

export interface DockReferencePickerHandle {
  open: () => void;
}

interface DockReferencePickerProps {
  workspaceId: string;
  selected: DockEntityReference[];
  onSelect: (reference: DockEntityReference) => void;
}

export const DockReferencePicker = forwardRef<DockReferencePickerHandle, DockReferencePickerProps>(
  function DockReferencePicker({ workspaceId, selected, onSelect }, ref) {
    const [open, setOpen] = useState(false);
    const [type, setType] = useState<DocsEntitySearchType>('task');
    const [query, setQuery] = useState('');
    const [items, setItems] = useState<DocsEntitySearchItem[]>([]);
    const [loading, setLoading] = useState(false);
    const inputRef = useRef<HTMLInputElement | null>(null);

    useImperativeHandle(ref, () => ({ open: () => setOpen(true) }), []);

    useEffect(() => {
      if (!open) return;
      const timer = window.setTimeout(() => inputRef.current?.focus(), 0);
      return () => window.clearTimeout(timer);
    }, [open]);

    useEffect(() => {
      if (!open) return;
      let cancelled = false;
      const timer = window.setTimeout(() => {
        setLoading(true);
        void searchDocsEntityItems({
          workspaceId,
          query,
          fixedEntityType: type,
          includeDocuments: true,
          fixedLimit: 8,
        }).then((result) => {
          if (cancelled) return;
          setItems(result.items);
          setLoading(false);
        });
      }, 180);
      return () => {
        cancelled = true;
        window.clearTimeout(timer);
      };
    }, [open, query, type, workspaceId]);

    const selectedKeys = new Set(selected.map((item) => `${item.entity_type}:${item.entity_id}`));

    const choose = (item: DocsEntitySearchItem) => {
      const entityType = referenceType(item.entityType);
      if (!entityType) return;
      onSelect({
        entity_type: entityType,
        entity_id: item.entityId,
        display_title: item.displayId ? `${item.displayId} · ${item.title}` : item.title,
      });
      setOpen(false);
      setQuery('');
    };

    return (
      <Popover open={open} onOpenChange={setOpen}>
        <PopoverTrigger asChild>
          <button
            type="button"
            className="inline-flex items-center gap-1 rounded-full border border-dashed border-border/70 px-2 py-0.5 text-[11px] text-muted-foreground transition hover:border-foreground/40 hover:text-foreground"
          >
            <PlusSignIcon className="h-3 w-3" />
            Add context
          </button>
        </PopoverTrigger>
        <PopoverContent
          data-helpin-dock-overlay="true"
          align="start"
          side="top"
          className="z-[70] w-[340px] p-2"
        >
          <div className="mb-2 flex flex-wrap gap-1">
            {REFERENCE_TYPES.map((candidate) => (
              <button
                key={candidate}
                type="button"
                onClick={() => {
                  setType(candidate);
                  setItems([]);
                }}
                className={candidate === type
                  ? 'rounded-md bg-foreground px-2 py-1 text-[11px] text-background'
                  : 'rounded-md px-2 py-1 text-[11px] text-muted-foreground hover:bg-muted'}
              >
                {entityTypeLabel(candidate)}
              </button>
            ))}
          </div>
          <QuietSearchInput
            ref={inputRef}
            value={query}
            onChange={(event) => setQuery(event.target.value)}
            placeholder={`Search ${entityTypeLabel(type).toLowerCase()}s…`}
          />
          <div className="mt-1 max-h-64 overflow-y-auto">
            {loading ? <p className="px-2 py-3 text-xs text-muted-foreground">Searching…</p> : null}
            {!loading && items.length === 0 ? (
              <p className="px-2 py-3 text-xs text-muted-foreground">No matches.</p>
            ) : null}
            {!loading && items.map((item) => {
              const mappedType = referenceType(item.entityType);
              const alreadySelected = mappedType
                ? selectedKeys.has(`${mappedType}:${item.entityId}`)
                : false;
              return (
                <button
                  key={`${item.entityType}:${item.entityId}`}
                  type="button"
                  disabled={alreadySelected}
                  onClick={() => choose(item)}
                  className="flex w-full items-start gap-2 rounded-md px-2 py-2 text-left hover:bg-muted disabled:opacity-45"
                >
                  <span className="mt-0.5 rounded bg-muted px-1.5 py-0.5 text-[10px] font-medium uppercase text-muted-foreground">
                    {entityTypeLabel(item.entityType)}
                  </span>
                  <span className="min-w-0">
                    <span className="block truncate text-xs font-medium">{item.title}</span>
                    <span className="block truncate text-[11px] text-muted-foreground">{item.meta}</span>
                  </span>
                </button>
              );
            })}
          </div>
        </PopoverContent>
      </Popover>
    );
  },
);
