import { forwardRef, useEffect, useId, useImperativeHandle, useRef, useState, type RefObject } from 'react';
import { PlusSignIcon } from '@/lib/icons';
import { Popover, PopoverAnchor, PopoverContent, PopoverTrigger } from '@/components/ui/popover';
import { QuietSearchInput } from '@/components/design-system/quiet';
import {
  entityTypeLabel,
  searchDocsEntityItems,
  type DocsEntitySearchItem,
  type DocsEntitySearchType,
} from '@/components/docs/entitySearch';
import type { DockEntityReference } from '@/lib/dockTypes';
import { cn } from '@/lib/utils';

const REFERENCE_TYPES: DocsEntitySearchType[] = ['task', 'document', 'epic', 'contact', 'deal'];

function referenceType(type: DocsEntitySearchType): DockEntityReference['entity_type'] | null {
  if (type === 'task') return 'task';
  if (type === 'document' || type === 'epic') return type;
  if (type === 'contact') return 'crm_contact';
  if (type === 'deal') return 'crm_deal';
  return null;
}

export interface DockReferencePickerHandle {
  onKeyDown: (key: string) => boolean;
}

interface ReferenceAutocomplete {
  query: string;
  listId: string;
  inputRef: RefObject<HTMLTextAreaElement | null>;
  onSelect: (reference: DockEntityReference) => void;
  onDismiss: () => void;
  onActiveOptionChange: (id: string | undefined) => void;
}

interface DockReferencePickerProps {
  workspaceId: string;
  selected: DockEntityReference[];
  onSelect: (reference: DockEntityReference) => void;
  autocomplete?: ReferenceAutocomplete;
}

export const DockReferencePicker = forwardRef<DockReferencePickerHandle, DockReferencePickerProps>(
  function DockReferencePicker({ workspaceId, selected, onSelect, autocomplete }, ref) {
    const [manualOpen, setManualOpen] = useState(false);
    const [type, setType] = useState<DocsEntitySearchType>('task');
    const [query, setQuery] = useState('');
    const inputRef = useRef<HTMLInputElement | null>(null);
    const triggerRef = useRef<HTMLButtonElement | null>(null);
    const openedInline = useRef(false);
    const manualListId = useId();
    const inline = manualOpen ? undefined : autocomplete;
    const open = manualOpen || !!inline;
    const searchQuery = inline?.query ?? query;

    useEffect(() => {
      if (!manualOpen) return;
      const timer = window.setTimeout(() => inputRef.current?.focus(), 0);
      return () => window.clearTimeout(timer);
    }, [manualOpen]);

    const choose = (item: DocsEntitySearchItem) => {
      const entityType = referenceType(item.entityType);
      if (!entityType) return;
      (inline?.onSelect ?? onSelect)({
        entity_type: entityType,
        entity_id: item.entityId,
        display_title: item.displayId ? `${item.displayId} · ${item.title}` : item.title,
      });
      setManualOpen(false);
      setQuery('');
    };

    return (
      <Popover open={open} onOpenChange={(next) => {
        if (!next && inline) inline.onDismiss();
        else setManualOpen(next);
      }}>
        {inline ? <PopoverAnchor virtualRef={inline.inputRef as RefObject<HTMLTextAreaElement>} /> : null}
        <PopoverTrigger asChild>
          <button
            ref={triggerRef}
            type="button"
            onClick={() => {
              autocomplete?.onDismiss();
              openedInline.current = false;
              setManualOpen(!manualOpen);
            }}
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
          className="z-[70] w-[340px] max-w-[calc(100vw-2rem)] p-2"
          onOpenAutoFocus={(event) => {
            openedInline.current = !!inline;
            if (inline) event.preventDefault();
          }}
          onCloseAutoFocus={(event) => {
            if (openedInline.current) event.preventDefault();
          }}
          onEscapeKeyDown={(event) => {
            event.preventDefault();
            event.stopPropagation();
            if (inline) inline.onDismiss();
            else {
              setManualOpen(false);
              triggerRef.current?.focus();
            }
          }}
          onInteractOutside={(event) => {
            if (inline && event.target === inline.inputRef.current) event.preventDefault();
          }}
        >
          <div className="mb-2 flex flex-wrap gap-1">
            {REFERENCE_TYPES.map((candidate) => (
              <button
                key={candidate}
                type="button"
                aria-pressed={candidate === type}
                onMouseDown={(event) => { if (inline) event.preventDefault(); }}
                onClick={() => setType(candidate)}
                className={candidate === type
                  ? 'rounded-md bg-foreground px-2 py-1 text-[11px] text-background'
                  : 'rounded-md px-2 py-1 text-[11px] text-muted-foreground hover:bg-muted'}
              >
                {entityTypeLabel(candidate)}
              </button>
            ))}
          </div>
          {!inline ? <QuietSearchInput
            ref={inputRef}
            value={query}
            onChange={(event) => setQuery(event.target.value)}
            placeholder={`Search ${entityTypeLabel(type).toLowerCase()}s…`}
          /> : null}
          {open ? <ReferenceResults
            key={JSON.stringify([workspaceId, type, searchQuery, !!inline])}
            ref={ref}
            workspaceId={workspaceId}
            type={type}
            query={searchQuery}
            selected={selected}
            onSelect={choose}
            autocomplete={inline}
            listId={inline?.listId ?? manualListId}
          /> : null}
        </PopoverContent>
      </Popover>
    );
  },
);

// Remount for each search so stale results and keyboard selection can never leak
// into the next query, including during the debounce interval.
const ReferenceResults = forwardRef<DockReferencePickerHandle, {
  workspaceId: string;
  type: DocsEntitySearchType;
  query: string;
  selected: DockEntityReference[];
  onSelect: (item: DocsEntitySearchItem) => void;
  autocomplete?: ReferenceAutocomplete;
  listId: string;
}>(function ReferenceResults({ workspaceId, type, query, selected, onSelect, autocomplete, listId }, ref) {
  const [result, setResult] = useState<Awaited<ReturnType<typeof searchDocsEntityItems>> | null>(null);
  const [activeIndex, setActiveIndex] = useState(-1);
  const listRef = useRef<HTMLDivElement | null>(null);
  const selectedKeys = new Set(selected.map((item) => `${item.entity_type}:${item.entity_id}`));
  const items = result?.items ?? [];
  const isSelected = (item: DocsEntitySearchItem) => selectedKeys.has(`${referenceType(item.entityType)}:${item.entityId}`);
  const activeItem = items[activeIndex];
  const activeId = activeItem && !isSelected(activeItem) ? `${listId}-${activeIndex}` : undefined;
  const onActiveOptionChange = autocomplete?.onActiveOptionChange;

  useEffect(() => {
    let cancelled = false;
    const timer = window.setTimeout(() => {
      void searchDocsEntityItems({ workspaceId, query, fixedEntityType: type, includeDocuments: true, fixedLimit: 8 })
        .then((next) => { if (!cancelled) setResult(next); })
        .catch(() => { if (!cancelled) setResult({ items: [], error: 'Could not load context. Try again.' }); });
    }, 180);
    return () => { cancelled = true; window.clearTimeout(timer); };
  }, [workspaceId, query, type]);

  useEffect(() => {
    onActiveOptionChange?.(activeId);
    if (activeId) listRef.current?.querySelector(`[id="${activeId}"]`)?.scrollIntoView?.({ block: 'nearest' });
  }, [activeId, onActiveOptionChange]);

  useImperativeHandle(ref, () => ({
    onKeyDown(key) {
      if (!autocomplete) return false;
      if (key === 'Enter' && activeItem && !isSelected(activeItem)) {
        onSelect(activeItem);
        return true;
      }
      if (key !== 'ArrowDown' && key !== 'ArrowUp') return false;
      const available = items.map((item, index) => isSelected(item) ? -1 : index).filter(index => index >= 0);
      if (available.length === 0) return false;
      const position = available.indexOf(activeIndex);
      const next = position < 0
        ? (key === 'ArrowDown' ? 0 : available.length - 1)
        : (position + (key === 'ArrowDown' ? 1 : -1) + available.length) % available.length;
      setActiveIndex(available[next]);
      return true;
    },
  }));

  return (
    <div ref={listRef} id={listId} role="listbox" aria-label="Context suggestions" aria-busy={!result} className="mt-1 max-h-64 overflow-y-auto">
      {!result ? <p role="status" className="px-2 py-3 text-xs text-muted-foreground">Searching…</p> : null}
      {result?.error ? <p role="status" className="px-2 py-3 text-xs text-muted-foreground">Could not load context. Try again.</p> : null}
      {result && !result.error && items.length === 0 ? <p role="status" className="px-2 py-3 text-xs text-muted-foreground">No matches.</p> : null}
      {items.map((item, index) => (
        <button
          key={`${item.entityType}:${item.entityId}`}
          id={`${listId}-${index}`}
          role="option"
          aria-selected={!!autocomplete && index === activeIndex}
          type="button"
          disabled={isSelected(item)}
          tabIndex={autocomplete ? -1 : 0}
          onMouseDown={(event) => { if (autocomplete) event.preventDefault(); }}
          onClick={() => onSelect(item)}
          className={cn('flex w-full items-start gap-2 rounded-md px-2 py-2 text-left hover:bg-muted disabled:opacity-45', autocomplete && index === activeIndex && 'bg-muted')}
        >
          <span className="mt-0.5 rounded bg-muted px-1.5 py-0.5 text-[10px] font-medium uppercase text-muted-foreground">
            {entityTypeLabel(item.entityType)}
          </span>
          <span className="min-w-0">
            <span className="block truncate text-xs font-medium">{item.title}</span>
            <span className="block truncate text-[11px] text-muted-foreground">{item.meta}</span>
          </span>
        </button>
      ))}
    </div>
  );
});
