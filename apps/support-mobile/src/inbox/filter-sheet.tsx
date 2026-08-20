import { useEffect, useState } from 'react'
import { Check, RotateCcw } from 'lucide-react'
import {
  useInboxScopes,
  useSupportTags,
} from '@helpin-ai/support-core'
import {
  conversationListFiltersEqual,
  type ConversationAIStateFilter,
  type ConversationAssignmentFilter,
  type ConversationListFilters,
  type ConversationSortOrder,
  type ConversationStateFilter,
} from '@/lib/supportInboxFilters'
import type { NavFilter } from '@/stores/supportInboxStore'
import { cn } from '@mobile/lib/cn'
import { haptic } from '@mobile/lib/haptics'
import { Pressable } from '@mobile/ui/pressable'
import { Sheet } from '@mobile/ui/sheet'
import { Spinner } from '@mobile/ui/spinner'

interface InboxFilterSheetProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  workspaceId: string
  navFilter: NavFilter
  selectedMailboxId: string
  filters: ConversationListFilters
  baseline: ConversationListFilters
  onApply: (filters: ConversationListFilters | null) => void
}

interface Option<T extends string> {
  value: T
  label: string
}

const STATE_OPTIONS: Option<ConversationStateFilter>[] = [
  { value: 'open', label: 'Open' },
  { value: 'waiting_on_customer', label: 'Waiting' },
  { value: 'resolved', label: 'Resolved' },
  { value: 'spam', label: 'Spam' },
]

const AI_OPTIONS: Option<ConversationAIStateFilter>[] = [
  { value: 'handling', label: 'AI handling' },
  { value: 'handoff', label: 'Needs teammate' },
  { value: 'resolved', label: 'AI resolved' },
]

const ASSIGNMENT_OPTIONS: Option<ConversationAssignmentFilter>[] = [
  { value: 'me', label: 'Assigned to me' },
  { value: 'mentioned_me', label: 'Mentioned me' },
  { value: 'opened_by_me', label: 'Opened by me' },
  { value: 'unassigned', label: 'Unassigned' },
  { value: 'others', label: 'Other teammates' },
]

function toggleValue<T extends string>(values: T[], value: T): T[] {
  return values.includes(value)
    ? values.filter((item) => item !== value)
    : [...values, value]
}

function FilterSection<T extends string>({
  title,
  options,
  selected,
  onToggle,
}: {
  title: string
  options: Option<T>[]
  selected: T[]
  onToggle: (value: T) => void
}) {
  return (
    <section className="space-y-2">
      <h3 className="px-1 text-caption font-semibold uppercase tracking-wide text-muted-foreground">{title}</h3>
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
            </Pressable>
          )
        })}
      </div>
    </section>
  )
}

export function InboxFilterSheet({
  open,
  onOpenChange,
  workspaceId,
  filters,
  navFilter,
  selectedMailboxId,
  baseline,
  onApply,
}: InboxFilterSheetProps) {
  const [draft, setDraft] = useState(filters)
  const scopesQuery = useInboxScopes(workspaceId, open)
  const tagsQuery = useSupportTags(workspaceId, open)

  useEffect(() => {
    if (open) setDraft(filters)
  }, [filters, open])

  const mailboxOptions: Option<string>[] = [
    { value: 'all', label: 'All inboxes' },
    { value: 'shared', label: scopesQuery.data?.shared_inbox.name ?? 'Main inbox' },
    ...(scopesQuery.data?.mailboxes ?? []).map((mailbox) => ({
      value: mailbox.id,
      label: mailbox.name,
    })),
  ]
  const tagOptions: Option<string>[] = (tagsQuery.data ?? []).map((tag) => ({
    value: tag.id,
    label: tag.name,
  }))
  const changed = !conversationListFiltersEqual(draft, baseline)

  const displayedMailboxIds = draft.mailboxIds.length > 0
    ? draft.mailboxIds
    : selectedMailboxId !== 'all'
      ? [selectedMailboxId]
      : navFilter === 'inbox'
        ? ['shared']
        : ['all']

  const toggleMailbox = (mailboxId: string) => {
    if (mailboxId === 'all') {
      setDraft((current) => ({
        ...current,
        mailboxIds: navFilter === 'inbox' ? ['all'] : [],
      }))
      return
    }
    setDraft((current) => {
      const currentMailboxIds = current.mailboxIds.length > 0
        ? current.mailboxIds
        : selectedMailboxId !== 'all'
          ? [selectedMailboxId]
          : navFilter === 'inbox'
            ? ['shared']
            : []
      const scoped = currentMailboxIds.filter((value) => value !== 'all')
      const next = toggleValue(scoped, mailboxId)
      return next.length === 0 ? current : { ...current, mailboxIds: next }
    })
  }

  const apply = () => {
    haptic('impactLight')
    onApply(changed ? draft : null)
    onOpenChange(false)
  }

  const reset = () => {
    setDraft(baseline)
    haptic('selection')
  }

  return (
    <Sheet
      open={open}
      onOpenChange={onOpenChange}
      title="Filter conversations"
      className="h-[86vh]"
    >
      <div className="flex items-center justify-between border-b border-border/60 px-4 pb-3">
        <div>
          <h2 className="text-title-3">Filter conversations</h2>
          <p className="text-footnote text-muted-foreground">Match the same fields available on web.</p>
        </div>
        <Pressable
          aria-label="Reset filters"
          disabled={!changed}
          onPress={reset}
          className="flex h-auto min-h-9 w-auto min-w-0 items-center gap-1.5 rounded-full px-3 text-footnote font-medium text-primary disabled:opacity-40"
        >
          <RotateCcw className="h-3.5 w-3.5" />
          Reset
        </Pressable>
      </div>

      <div className="min-h-0 flex-1 space-y-5 overflow-y-auto px-4 py-4">
        <FilterSection
          title="State"
          options={STATE_OPTIONS}
          selected={draft.states}
          onToggle={(value) => setDraft((current) => {
            const states = toggleValue(current.states, value)
            return states.length === 0 ? current : { ...current, states }
          })}
        />
        <FilterSection
          title="AI state"
          options={AI_OPTIONS}
          selected={draft.aiStates}
          onToggle={(value) => setDraft((current) => ({
            ...current,
            aiStates: toggleValue(current.aiStates, value),
          }))}
        />
        <FilterSection
          title="Assignment"
          options={ASSIGNMENT_OPTIONS}
          selected={draft.assignment}
          onToggle={(value) => setDraft((current) => {
            const assignment = toggleValue(current.assignment, value)
            return navFilter === 'mine' && assignment.length === 0
              ? current
              : { ...current, assignment }
          })}
        />

        <section className="space-y-2">
          <h3 className="px-1 text-caption font-semibold uppercase tracking-wide text-muted-foreground">Inbox</h3>
          {scopesQuery.isPending ? (
            <Spinner size={16} />
          ) : (
            <>
              {draft.mailboxIds.length === 0 && (
                <p className="px-1 text-footnote text-muted-foreground">Using the current view's inbox scope.</p>
              )}
              <FilterSection
                title=""
                options={mailboxOptions}
                selected={displayedMailboxIds}
                onToggle={toggleMailbox}
              />
            </>
          )}
        </section>

        <section className="space-y-2">
          <h3 className="px-1 text-caption font-semibold uppercase tracking-wide text-muted-foreground">Tags</h3>
          {tagsQuery.isPending ? (
            <Spinner size={16} />
          ) : tagOptions.length === 0 ? (
            <p className="px-1 text-footnote text-muted-foreground">No workspace tags yet.</p>
          ) : (
            <FilterSection
              title=""
              options={tagOptions}
              selected={draft.tagIds}
              onToggle={(value) => setDraft((current) => ({
                ...current,
                tagIds: toggleValue(current.tagIds, value),
              }))}
            />
          )}
        </section>

        <FilterSection<ConversationSortOrder>
          title="Sort"
          options={[
            { value: 'newest', label: 'Newest first' },
            { value: 'oldest', label: 'Oldest first' },
          ]}
          selected={[draft.sort]}
          onToggle={(value) => setDraft((current) => ({ ...current, sort: value }))}
        />
      </div>

      <div className="border-t border-border/60 px-4 pt-3">
        <Pressable
          haptic="impactMedium"
          onPress={apply}
          className="flex min-h-12 w-full items-center justify-center rounded-xl bg-primary px-4 text-body font-semibold text-primary-foreground active:opacity-90"
        >
          Show conversations
        </Pressable>
      </div>
    </Sheet>
  )
}
