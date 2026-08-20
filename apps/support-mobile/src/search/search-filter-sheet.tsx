import { useEffect, useState } from 'react'
import { Check, RotateCcw } from 'lucide-react'
import { useInboxScopes, useSupportTags } from '@helpin-ai/support-core'
import { cn } from '@mobile/lib/cn'
import { haptic } from '@mobile/lib/haptics'
import { Pressable } from '@mobile/ui/pressable'
import { Sheet } from '@mobile/ui/sheet'
import { Spinner } from '@mobile/ui/spinner'
import {
  EMPTY_CONVERSATION_SEARCH_FILTERS,
  type ConversationSearchFilters,
} from './search-filters'

interface SearchFilterSheetProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  workspaceId: string
  filters: ConversationSearchFilters
  onApply: (filters: ConversationSearchFilters) => void
}

interface Option {
  value: string
  label: string
}

const STATUS_OPTIONS: Option[] = [
  { value: 'open', label: 'Open' },
  { value: 'waiting_on_customer', label: 'Waiting' },
  { value: 'resolved', label: 'Resolved' },
  { value: 'spam', label: 'Spam' },
]
const PRIORITY_OPTIONS: Option[] = [
  { value: 'low', label: 'Low' },
  { value: 'medium', label: 'Medium' },
  { value: 'high', label: 'High' },
  { value: 'urgent', label: 'Urgent' },
]
const ASSIGNMENT_OPTIONS: Option[] = [
  { value: 'me', label: 'Assigned to me' },
  { value: 'mentioned_me', label: 'Mentioned me' },
  { value: 'opened_by_me', label: 'Opened by me' },
  { value: 'unassigned', label: 'Unassigned' },
]
const AI_OPTIONS: Option[] = [
  { value: 'handling', label: 'AI handling' },
  { value: 'handoff', label: 'AI handoff' },
  { value: 'resolved', label: 'AI resolved' },
]
const SORT_OPTIONS: Option[] = [
  { value: 'relevance', label: 'Relevance' },
  { value: 'newest', label: 'Newest' },
  { value: 'oldest', label: 'Oldest' },
]

function toggleValue(values: string[], value: string) {
  return values.includes(value) ? values.filter((item) => item !== value) : [...values, value]
}

function FilterSection({
  title,
  options,
  selected,
  onToggle,
  exclusive = false,
}: {
  title: string
  options: Option[]
  selected: string[]
  onToggle: (value: string) => void
  exclusive?: boolean
}) {
  return (
    <section className="space-y-2">
      {title && <h3 className="text-caption font-semibold uppercase tracking-wide text-muted-foreground">{title}</h3>}
      <div className="flex flex-wrap gap-2">
        {options.map((option) => {
          const active = selected.includes(option.value)
          return (
            <Pressable
              key={option.value}
              aria-pressed={active}
              onPress={() => onToggle(option.value)}
              className={cn(
                'flex h-auto min-h-10 w-auto min-w-0 items-center gap-1.5 rounded-full border px-3 text-footnote font-medium',
                active
                  ? 'border-primary/30 bg-primary/10 text-primary'
                  : 'border-border bg-background text-foreground',
              )}
            >
              {active && <Check className="h-3.5 w-3.5" />}
              {option.label}
              {exclusive && active ? <span className="sr-only"> selected</span> : null}
            </Pressable>
          )
        })}
      </div>
    </section>
  )
}

function SearchInput({
  label,
  value,
  type = 'text',
  placeholder,
  onChange,
}: {
  label: string
  value: string
  type?: 'text' | 'email' | 'date'
  placeholder?: string
  onChange: (value: string) => void
}) {
  return (
    <label className="block space-y-1.5 text-footnote font-medium text-foreground">
      <span>{label}</span>
      <input
        type={type}
        value={value}
        placeholder={placeholder}
        onChange={(event) => onChange(event.target.value)}
        className="h-11 w-full rounded-xl border border-input bg-background px-3 text-body text-foreground outline-none focus:border-primary/50 focus:ring-2 focus:ring-primary/20"
      />
    </label>
  )
}

export function SearchFilterSheet({
  open,
  onOpenChange,
  workspaceId,
  filters,
  onApply,
}: SearchFilterSheetProps) {
  const [draft, setDraft] = useState(filters)
  const scopesQuery = useInboxScopes(workspaceId, open)
  const tagsQuery = useSupportTags(workspaceId, open)

  useEffect(() => {
    if (open) setDraft(filters)
  }, [filters, open])

  const mailboxOptions: Option[] = [
    { value: 'shared', label: scopesQuery.data?.shared_inbox.name ?? 'Main inbox' },
    ...(scopesQuery.data?.mailboxes ?? []).map((mailbox) => ({ value: mailbox.id, label: mailbox.name })),
  ]
  const tagOptions: Option[] = (tagsQuery.data ?? []).map((tag) => ({ value: tag.id, label: tag.name }))

  const apply = () => {
    haptic('impactLight')
    onApply(draft)
    onOpenChange(false)
  }

  return (
    <Sheet open={open} onOpenChange={onOpenChange} title="Search filters" className="h-[90vh]">
      <div className="flex items-center justify-between border-b border-border/60 px-4 pb-3">
        <div>
          <h2 className="text-title-3">Search filters</h2>
          <p className="text-footnote text-muted-foreground">Narrow the same fields available on web.</p>
        </div>
        <Pressable
          aria-label="Clear search filters"
          onPress={() => setDraft(EMPTY_CONVERSATION_SEARCH_FILTERS)}
          className="flex h-auto min-h-9 w-auto min-w-0 items-center gap-1.5 rounded-full px-3 text-footnote font-medium text-primary"
        >
          <RotateCcw className="h-3.5 w-3.5" />
          Clear
        </Pressable>
      </div>

      <div className="min-h-0 flex-1 space-y-5 overflow-y-auto px-4 py-4">
        <FilterSection
          title="Sort"
          options={SORT_OPTIONS}
          selected={[draft.sort]}
          exclusive
          onToggle={(value) => setDraft((current) => ({ ...current, sort: value as ConversationSearchFilters['sort'] }))}
        />
        <FilterSection
          title="State"
          options={STATUS_OPTIONS}
          selected={draft.statuses}
          onToggle={(value) => setDraft((current) => ({ ...current, statuses: toggleValue(current.statuses, value) }))}
        />
        <FilterSection
          title="Priority"
          options={PRIORITY_OPTIONS}
          selected={draft.priorities}
          onToggle={(value) => setDraft((current) => ({ ...current, priorities: toggleValue(current.priorities, value) }))}
        />
        <FilterSection
          title="Assignment"
          options={ASSIGNMENT_OPTIONS}
          selected={draft.assignedTo}
          onToggle={(value) => setDraft((current) => ({ ...current, assignedTo: toggleValue(current.assignedTo, value) }))}
        />
        <FilterSection
          title="AI state"
          options={AI_OPTIONS}
          selected={draft.aiStates}
          onToggle={(value) => setDraft((current) => ({ ...current, aiStates: toggleValue(current.aiStates, value) }))}
        />

        <section className="space-y-2">
          <h3 className="text-caption font-semibold uppercase tracking-wide text-muted-foreground">Inbox</h3>
          {scopesQuery.isPending ? <Spinner size={16} /> : (
            <FilterSection
              title=""
              options={mailboxOptions}
              selected={draft.mailboxIds}
              onToggle={(value) => setDraft((current) => ({ ...current, mailboxIds: toggleValue(current.mailboxIds, value) }))}
            />
          )}
        </section>

        <section className="space-y-2">
          <h3 className="text-caption font-semibold uppercase tracking-wide text-muted-foreground">Tags</h3>
          {tagsQuery.isPending ? <Spinner size={16} /> : tagOptions.length === 0 ? (
            <p className="text-footnote text-muted-foreground">No workspace tags yet.</p>
          ) : (
            <FilterSection
              title=""
              options={tagOptions}
              selected={draft.tagIds}
              onToggle={(value) => setDraft((current) => ({ ...current, tagIds: toggleValue(current.tagIds, value) }))}
            />
          )}
        </section>

        <div className="grid grid-cols-1 gap-3">
          <SearchInput
            label="Customer email"
            type="email"
            value={draft.customerEmail}
            placeholder="customer@example.com"
            onChange={(value) => setDraft((current) => ({ ...current, customerEmail: value }))}
          />
          <SearchInput
            label="Conversation title"
            value={draft.title}
            placeholder="Title contains…"
            onChange={(value) => setDraft((current) => ({ ...current, title: value }))}
          />
          <div className="grid grid-cols-2 gap-3">
            <SearchInput
              label="Created from"
              type="date"
              value={draft.createdFrom}
              onChange={(value) => setDraft((current) => ({ ...current, createdFrom: value }))}
            />
            <SearchInput
              label="Created to"
              type="date"
              value={draft.createdTo}
              onChange={(value) => setDraft((current) => ({ ...current, createdTo: value }))}
            />
          </div>
        </div>
      </div>

      <div className="border-t border-border/60 px-4 pt-3">
        <Pressable
          haptic="impactMedium"
          onPress={apply}
          className="flex min-h-12 w-full items-center justify-center rounded-xl bg-primary px-4 text-body font-semibold text-primary-foreground active:opacity-90"
        >
          Apply filters
        </Pressable>
      </div>
    </Sheet>
  )
}
