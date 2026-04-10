import { useMemo, useState } from 'react'
import {
  Popover,
  PopoverContent,
  PopoverTrigger,
} from '@/components/ui/popover'
import {
  Command,
  CommandEmpty,
  CommandGroup,
  CommandInput,
  CommandItem,
  CommandList,
} from '@/components/ui/command'
import { ArrowDown01Icon, Folder01Icon, Tick01Icon } from '@/lib/icons'
import { cn } from '@/lib/utils'
import { buildCollectionTree, type CollectionTreeNode } from './docsCollectionTree'
import type { DocsCollection } from '@/lib/docsTypes'

/**
 * A flat representation of one selectable collection row, with the
 * depth-derived indentation level and a breadcrumb path ("Parent / Self")
 * used for search and display.
 */
export interface CollectionTreeOption {
  id: string
  name: string
  depth: number
  path: string // "Parent / Child / Self"
}

/**
 * buildCollectionTreeOptions folds a flat collection list into a
 * depth-first sequence of CollectionTreeOption rows. Exposed alongside
 * the component so tests and other pickers (settings, pm tasks) can
 * reuse the same ordering and path logic.
 */
export function buildCollectionTreeOptions(
  spaceId: string,
  collections: DocsCollection[],
): CollectionTreeOption[] {
  const tree = buildCollectionTree(spaceId, collections, [])
  const out: CollectionTreeOption[] = []
  const walk = (nodes: CollectionTreeNode[], ancestors: string[]) => {
    for (const node of nodes) {
      const path = [...ancestors, node.collection.name].join(' / ')
      out.push({
        id: node.collection.id,
        name: node.collection.name,
        depth: ancestors.length,
        path,
      })
      if (node.children.length > 0) {
        walk(node.children, [...ancestors, node.collection.name])
      }
    }
  }
  walk(tree.topLevel, [])
  return out
}

/**
 * Value sentinels used by the picker.
 *
 * The picker always returns a DocsCollection.id or null (via the
 * NONE value). It never leaks the internal sentinel to onChange.
 */
const NONE_VALUE = '__none__'

interface CollectionTreePickerProps {
  /** All collections in the target space. Filtered internally by space_id. */
  collections: DocsCollection[]
  /** The space the picker is scoped to. Required so cross-space rows are ignored. */
  spaceId: string
  /** Currently selected collection id. null means the "None" option is selected. */
  value: string | null
  /** Called with the new collection id (or null) after the user picks. */
  onChange: (next: string | null) => void
  /** Label shown for the "no collection" entry. Default: "None". */
  noneLabel?: string
  /** Optional trigger content override. Default: selected name or placeholder. */
  placeholder?: string
  /** Disable the picker. */
  disabled?: boolean
  /** Optional extra classnames for the trigger. */
  triggerClassName?: string
  /**
   * When false, the "None" option is hidden — useful when the caller
   * requires the selection to live inside a collection.
   */
  allowNone?: boolean
}

/**
 * CollectionTreePicker is a reusable popover-backed search picker that
 * renders a flat collection list as a depth-indented tree. Every node
 * keeps its full ancestor path in the searchable value so typing a
 * parent name filters to every descendant bucket as well.
 *
 * The picker is a pure selector — it never creates or deletes
 * collections. Callers that need a "new collection" action should
 * render an adjacent button.
 */
export function CollectionTreePicker({
  collections,
  spaceId,
  value,
  onChange,
  noneLabel = 'None',
  placeholder = 'Select collection…',
  disabled = false,
  triggerClassName,
  allowNone = true,
}: CollectionTreePickerProps) {
  const [open, setOpen] = useState(false)

  const options = useMemo(
    () => buildCollectionTreeOptions(spaceId, collections),
    [spaceId, collections],
  )
  const optionById = useMemo(() => {
    const map = new Map<string, CollectionTreeOption>()
    for (const opt of options) map.set(opt.id, opt)
    return map
  }, [options])

  const selected = value ? optionById.get(value) : undefined
  const triggerLabel = selected
    ? selected.name
    : value == null && allowNone
      ? noneLabel
      : placeholder

  const handleSelect = (next: string) => {
    if (next === NONE_VALUE) {
      onChange(null)
    } else {
      onChange(next)
    }
    setOpen(false)
  }

  return (
    <Popover open={open} onOpenChange={setOpen}>
      <PopoverTrigger asChild disabled={disabled}>
        <button
          type="button"
          className={cn(
            'flex w-full items-center justify-between gap-2 rounded-md border border-input bg-background px-3 py-2 text-left text-sm shadow-sm transition-colors hover:bg-accent/40 disabled:cursor-not-allowed disabled:opacity-60',
            triggerClassName,
          )}
        >
          <div className="flex min-w-0 flex-1 items-center gap-2">
            <Folder01Icon className="h-3.5 w-3.5 shrink-0 text-muted-foreground" />
            <span className="min-w-0 flex-1 truncate">{triggerLabel}</span>
          </div>
          <ArrowDown01Icon className="h-3.5 w-3.5 shrink-0 text-muted-foreground" />
        </button>
      </PopoverTrigger>
      <PopoverContent className="w-[280px] p-0" align="start">
        <Command>
          <CommandInput placeholder="Search collections…" className="h-9" />
          <CommandList>
            <CommandEmpty>No collections.</CommandEmpty>
            <CommandGroup>
              {allowNone && (
                <CommandItem
                  value={noneLabel}
                  onSelect={() => handleSelect(NONE_VALUE)}
                  className="flex items-center gap-2 text-sm"
                >
                  <span className="flex-1 text-muted-foreground">{noneLabel}</span>
                  {value == null && <Tick01Icon className="h-3.5 w-3.5 text-primary" />}
                </CommandItem>
              )}
              {options.map((option) => (
                <CommandItem
                  key={option.id}
                  // value is what cmdk matches against during search; using
                  // the full breadcrumb path means typing "Parent Child"
                  // still finds the nested node.
                  value={option.path}
                  onSelect={() => handleSelect(option.id)}
                  className="flex items-center gap-2 text-sm"
                >
                  <span
                    className="flex min-w-0 flex-1 items-center gap-1.5"
                    style={{ paddingLeft: `${option.depth * 12}px` }}
                  >
                    <Folder01Icon className="h-3.5 w-3.5 shrink-0 text-muted-foreground" />
                    <span className="truncate">{option.name}</span>
                  </span>
                  {value === option.id && <Tick01Icon className="h-3.5 w-3.5 text-primary" />}
                </CommandItem>
              ))}
            </CommandGroup>
          </CommandList>
        </Command>
      </PopoverContent>
    </Popover>
  )
}
