import { useEffect, useMemo, useState, type FormEvent, type ReactNode } from 'react';
import { useNavigate, useSearch } from '@tanstack/react-router';
import { Search01Icon, Cancel01Icon, ArrowLeft02Icon, ArrowRight02Icon, InboxIcon, FilterHorizontalIcon } from '@/lib/icons';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Badge } from '@/components/ui/badge';
import { Label } from '@/components/ui/label';
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select';
import { Popover, PopoverContent, PopoverTrigger } from '@/components/ui/popover';
import { CategoryFilterChip } from '@/components/pm/CategoryFilterChip';
import { useTitle } from '@/hooks/useTitle';
import { useWorkspaceStore } from '@/stores/workspaceStore';
import {
  hasSupportConversationSearchInput,
  useSupportConversationSearch,
  useSupportMailboxes,
  useSupportTags,
} from '@/hooks/queries/useSupport';
import { cn } from '@/lib/utils';
import type {
  ConversationPriority,
  ConversationStatus,
  SupportMailbox,
  SupportTag,
  SupportConversationSearchParams,
  SupportConversationSearchResult,
  SupportSearchHighlight,
} from '@/lib/pmTypes';
import { PRIORITY_LABELS, STATUS_LABELS } from '@/components/support/constants';

export type SupportSearchRouteSearch = SupportConversationSearchParams;

const SEARCH_PER_PAGE = 50;
const assignmentOptions = [
  { value: 'me', label: 'Assigned to me' },
  { value: 'mentioned_me', label: 'Mentioned me' },
  { value: 'opened_by_me', label: 'Opened by me' },
  { value: 'unassigned', label: 'Unassigned' },
];
const aiStateOptions = [
  { value: 'handling', label: 'AI handling' },
  { value: 'handoff', label: 'AI handoff' },
  { value: 'resolved', label: 'AI resolved' },
];
const fieldLabels: Record<string, string> = {
  title: 'Title',
  customer_email: 'Email',
  customer_name: 'Customer',
  display_id: 'Number',
  message: 'Message',
};

function normalizeSearch(search: Record<string, unknown>): SupportSearchRouteSearch {
  return {
    q: typeof search.q === 'string' ? search.q : undefined,
    sort: search.sort === 'newest' || search.sort === 'oldest' || search.sort === 'relevance' ? search.sort : undefined,
    assigned_to: typeof search.assigned_to === 'string' ? search.assigned_to : undefined,
    mailbox_ids: typeof search.mailbox_ids === 'string' ? search.mailbox_ids : undefined,
    tag_ids: typeof search.tag_ids === 'string' ? search.tag_ids : undefined,
    customer_email: typeof search.customer_email === 'string' ? search.customer_email : undefined,
    created_from: typeof search.created_from === 'string' ? search.created_from : undefined,
    created_to: typeof search.created_to === 'string' ? search.created_to : undefined,
    statuses: typeof search.statuses === 'string' ? search.statuses : undefined,
    priorities: typeof search.priorities === 'string' ? search.priorities : undefined,
    title: typeof search.title === 'string' ? search.title : undefined,
    ai: typeof search.ai === 'string' ? search.ai : undefined,
    page: typeof search.page === 'number' ? search.page : Number(search.page) || undefined,
    per_page: typeof search.per_page === 'number' ? search.per_page : Number(search.per_page) || undefined,
  };
}

function cleanedSearch(search: SupportSearchRouteSearch): SupportSearchRouteSearch {
  const next: SupportSearchRouteSearch = {};
  Object.entries(search).forEach(([key, value]) => {
    if (value === undefined || value === null) return;
    if (typeof value === 'string' && value.trim() === '') return;
    (next as Record<string, unknown>)[key] = typeof value === 'string' ? value.trim() : value;
  });
  return next;
}

function formatDate(value?: string) {
  if (!value) return '';
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return '';
  return new Intl.DateTimeFormat(undefined, {
    month: 'short',
    day: 'numeric',
    hour: 'numeric',
    minute: '2-digit',
  }).format(date);
}

function highlightParts(text: string, ranges: SupportSearchHighlight['ranges']) {
  if (!ranges.length) return text;
  const parts: ReactNode[] = [];
  let cursor = 0;
  ranges
    .slice()
    .sort((a, b) => a.start - b.start)
    .forEach((range, index) => {
      const start = Math.max(cursor, Math.min(text.length, range.start));
      const end = Math.max(start, Math.min(text.length, range.end));
      if (start > cursor) parts.push(text.slice(cursor, start));
      if (end > start) {
        parts.push(
          <mark key={`${start}-${end}-${index}`} className="rounded-sm bg-amber-200/80 px-0.5 text-foreground dark:bg-amber-500/30">
            {text.slice(start, end)}
          </mark>,
        );
      }
      cursor = end;
    });
  if (cursor < text.length) parts.push(text.slice(cursor));
  return parts.length > 0 ? parts : text;
}

function resultHighlight(result: SupportConversationSearchResult) {
  const primary = result.highlights[0];
  if (primary) return highlightParts(primary.text, primary.ranges);
  return result.snippet || result.conversation.last_message || result.conversation.subject;
}

function fieldSummary(fields: string[]) {
  if (fields.length === 0) return 'Filter match';
  return fields.map((field) => fieldLabels[field] ?? field).join(', ');
}

function splitFilterValues(value?: string) {
  return value?.split(',').map((entry) => entry.trim()).filter(Boolean) ?? [];
}

function joinFilterValues(values: string[]) {
  return values.length > 0 ? values.join(',') : undefined;
}

function detailedFilterCount(draft: SupportSearchRouteSearch) {
  return [
    draft.customer_email,
    draft.title,
    draft.created_from,
    draft.created_to,
  ].filter((value) => typeof value === 'string' && value.trim().length > 0).length;
}

export function SupportSearchToolbar({
  draft,
  mailboxes,
  tags,
  onDraftChange,
  onSubmit,
  onClear,
}: {
  draft: SupportSearchRouteSearch;
  mailboxes: SupportMailbox[];
  tags: SupportTag[];
  onDraftChange: (key: keyof SupportSearchRouteSearch, value: string | number | undefined) => void;
  onSubmit: (event?: FormEvent) => void;
  onClear: () => void;
}) {
  const hasAnyFilter = hasSupportConversationSearchInput(draft);
  const moreCount = detailedFilterCount(draft);

  return (
    <form onSubmit={onSubmit} className="ui-divider-bottom-fade flex flex-col gap-2 px-4 pb-2 pt-3 md:px-6">
      <div data-slot="support-search-primary-row" className="flex flex-col gap-2 lg:flex-row lg:items-center">
        <div className="relative min-w-0 flex-1">
          <Search01Icon className="pointer-events-none absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
          <Input
            autoFocus
            value={draft.q ?? ''}
            maxLength={256}
            onChange={(event) => onDraftChange('q', event.target.value)}
            placeholder="Search conversations by email, #number, title, customer, or message"
            className="h-10 pl-9"
          />
        </div>
        <div className="flex shrink-0 items-center gap-2">
          <Button type="submit" className="h-10 gap-2">
            <Search01Icon className="h-4 w-4" />
            Search
          </Button>
          <Button type="button" variant="ghost" size="icon" className="h-10 w-10" onClick={onClear} aria-label="Clear search">
            <Cancel01Icon className="h-4 w-4" />
          </Button>
        </div>
      </div>

      <div data-slot="support-search-filter-row" className="flex flex-wrap items-end gap-2">
        <CategoryFilterChip
          label="Status"
          options={Object.entries(STATUS_LABELS).map(([value, label]) => ({ value, label }))}
          selected={splitFilterValues(draft.statuses)}
          onChange={(next) => onDraftChange('statuses', joinFilterValues(next))}
        />
        <CategoryFilterChip
          label="Priority"
          options={Object.entries(PRIORITY_LABELS).map(([value, label]) => ({ value, label }))}
          selected={splitFilterValues(draft.priorities)}
          onChange={(next) => onDraftChange('priorities', joinFilterValues(next))}
        />
        <CategoryFilterChip
          label="Assignee"
          options={assignmentOptions}
          selected={splitFilterValues(draft.assigned_to)}
          onChange={(next) => onDraftChange('assigned_to', joinFilterValues(next))}
        />
        <CategoryFilterChip
          label="Inbox"
          options={[
            { value: 'shared', label: 'Main inbox' },
            ...mailboxes.map((mailbox) => ({ value: mailbox.id, label: mailbox.name })),
          ]}
          selected={splitFilterValues(draft.mailbox_ids)}
          onChange={(next) => onDraftChange('mailbox_ids', joinFilterValues(next))}
        />
        <CategoryFilterChip
          label="Tag"
          options={tags.map((tag) => ({
            value: tag.id,
            label: tag.name,
            leading: <span className="h-2.5 w-2.5 rounded-full" style={{ backgroundColor: tag.color ?? '#94a3b8' }} />,
          }))}
          selected={splitFilterValues(draft.tag_ids)}
          onChange={(next) => onDraftChange('tag_ids', joinFilterValues(next))}
        />
        <CategoryFilterChip
          label="AI state"
          options={aiStateOptions}
          selected={splitFilterValues(draft.ai)}
          onChange={(next) => onDraftChange('ai', joinFilterValues(next))}
        />

        <div className="flex flex-col gap-0.5">
          <span className="text-xs font-medium text-muted-foreground">More</span>
          <Popover>
            <PopoverTrigger asChild>
              <button
                type="button"
                className={cn(
                  'inline-flex h-7 min-w-[90px] items-center justify-between gap-1 rounded-md border border-input bg-transparent px-2 text-xs transition-colors hover:bg-accent',
                  moreCount > 0 ? 'border-primary/40 bg-primary/5 text-foreground' : 'text-muted-foreground',
                )}
              >
                <FilterHorizontalIcon className="h-3.5 w-3.5" />
                <span>{moreCount > 0 ? `${moreCount} active` : 'Fields'}</span>
              </button>
            </PopoverTrigger>
            <PopoverContent align="start" className="w-[360px] p-3">
              <div className="grid gap-3">
                <LabeledInput label="Customer email" value={draft.customer_email ?? ''} onChange={(value) => onDraftChange('customer_email', value)} placeholder="customer@example.com" />
                <LabeledInput label="Conversation title" value={draft.title ?? ''} onChange={(value) => onDraftChange('title', value)} placeholder="Title contains" />
                <div className="grid grid-cols-2 gap-2">
                  <LabeledInput label="From" type="date" value={draft.created_from ?? ''} onChange={(value) => onDraftChange('created_from', value)} />
                  <LabeledInput label="To" type="date" value={draft.created_to ?? ''} onChange={(value) => onDraftChange('created_to', value)} />
                </div>
              </div>
            </PopoverContent>
          </Popover>
        </div>

        {hasAnyFilter ? (
          <div className="flex flex-col gap-0.5">
            <span className="text-[11px] font-medium text-muted-foreground/0">&nbsp;</span>
            <Button
              type="button"
              variant="ghost"
              size="sm"
              className="h-7 px-2 text-xs text-muted-foreground"
              onClick={onClear}
            >
              Clear Filters
            </Button>
          </div>
        ) : null}

        <div className="ml-auto flex items-center gap-2">
          <span className="text-xs text-muted-foreground">Sort by:</span>
          <Select value={draft.sort ?? 'relevance'} onValueChange={(value) => onDraftChange('sort', value)}>
            <SelectTrigger className="h-7 w-[130px] text-xs">
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="relevance">Relevance</SelectItem>
              <SelectItem value="newest">Newest</SelectItem>
              <SelectItem value="oldest">Oldest</SelectItem>
            </SelectContent>
          </Select>
        </div>
      </div>
    </form>
  );
}

export function SupportSearchPage() {
  useTitle('Support Search');
  const workspace = useWorkspaceStore((state) => state.currentWorkspace);
  const workspaceId = workspace?.id ?? '';
  const slug = workspace?.slug ?? '';
  const navigate = useNavigate();
  const routeSearch = normalizeSearch(useSearch({ strict: false }) as Record<string, unknown>);
  const activeSearch = useMemo(
    () => cleanedSearch({ ...routeSearch, per_page: routeSearch.per_page ?? SEARCH_PER_PAGE }),
    [routeSearch],
  );
  const [draft, setDraft] = useState(activeSearch);
  const hasInput = hasSupportConversationSearchInput(activeSearch);
  const { data, isFetching, error } = useSupportConversationSearch(workspaceId, activeSearch, hasInput);
  const { data: mailboxes = [] } = useSupportMailboxes(workspaceId);
  const { data: tags = [] } = useSupportTags(workspaceId);

  useEffect(() => {
    setDraft(activeSearch);
  }, [activeSearch]);

  const updateDraft = (key: keyof SupportSearchRouteSearch, value: string | number | undefined) => {
    setDraft((current) => cleanedSearch({ ...current, [key]: value, page: undefined }));
  };

  const applySearch = (event?: FormEvent) => {
    event?.preventDefault();
    void navigate({
      to: '/w/$slug/support/search',
      params: { slug },
      search: cleanedSearch({ ...draft, page: 1, per_page: SEARCH_PER_PAGE }) as never,
    });
  };

  const clearSearch = () => {
    setDraft({});
    void navigate({ to: '/w/$slug/support/search', params: { slug }, search: {} as never });
  };

  const setPage = (page: number) => {
    void navigate({
      to: '/w/$slug/support/search',
      params: { slug },
      search: cleanedSearch({ ...activeSearch, page, per_page: SEARCH_PER_PAGE }) as never,
    });
  };

  const openConversation = (conversationId: string) => {
    void navigate({
      to: '/w/$slug/support/$conversationId',
      params: { slug, conversationId },
    });
  };

  if (!workspace) {
    return <p className="p-4 text-sm text-muted-foreground">Workspace not found.</p>;
  }

  const results = data?.data ?? [];
  const total = data?.total ?? 0;
  const page = data?.page ?? activeSearch.page ?? 1;
  const totalPages = data?.total_pages ?? 0;

  return (
    <div className="flex h-full min-h-0 flex-col bg-background">
      <SupportSearchToolbar
        draft={draft}
        mailboxes={mailboxes}
        tags={tags}
        onDraftChange={updateDraft}
        onSubmit={applySearch}
        onClear={clearSearch}
      />

      <div className="flex min-h-0 flex-1 flex-col">
        {hasInput && (
          <div className="flex h-12 shrink-0 items-center justify-between border-b px-6">
            <div className="text-sm text-muted-foreground">
              <>
                <span className="font-medium text-foreground">{total}</span>
                {data?.meta.total_capped ? '+' : ''} result{total === 1 ? '' : 's'}
                {data?.meta.total_capped ? `, capped at ${data.meta.total_cap}` : ''}
              </>
            </div>
            {isFetching && <span className="text-xs text-muted-foreground">Searching...</span>}
          </div>
        )}

        <div className="min-h-0 flex-1 overflow-y-auto">
          {!hasInput ? (
            <div />
          ) : error ? (
            <SearchEmptyState title="Search failed" description={error.message} />
          ) : results.length === 0 && !isFetching ? (
            <SearchEmptyState title="No conversations found" description="Try a broader query or remove a filter." />
          ) : (
            <div data-slot="support-search-results" className="min-w-0">
              <div className="sticky top-0 z-10 hidden border-b bg-background/95 px-6 py-2 text-[11px] font-medium uppercase tracking-wide text-muted-foreground backdrop-blur md:grid md:grid-cols-[minmax(220px,1.25fr)_minmax(280px,1.6fr)_minmax(180px,0.9fr)_minmax(170px,0.85fr)_minmax(120px,0.65fr)_24px] md:gap-4">
                <span>Conversation</span>
                <span>Match</span>
                <span>Customer</span>
                <span>State</span>
                <span>Updated</span>
                <span />
              </div>
              <div className="divide-y">
                {results.map((result) => (
                  <button
                    key={result.conversation.id}
                    type="button"
                    data-slot="support-search-result-row"
                    className="group grid w-full grid-cols-1 gap-3 px-4 py-3 text-left transition-colors hover:bg-muted/40 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring md:grid-cols-[minmax(220px,1.25fr)_minmax(280px,1.6fr)_minmax(180px,0.9fr)_minmax(170px,0.85fr)_minmax(120px,0.65fr)_24px] md:items-center md:gap-4 md:px-6"
                    onClick={() => openConversation(result.conversation.id)}
                  >
                    <div className="min-w-0">
                      <div className="flex min-w-0 items-center gap-2">
                        <span className="shrink-0 font-mono text-[11px] text-muted-foreground">#{result.display_id}</span>
                        <span className="truncate text-sm font-semibold text-foreground">{result.conversation.subject}</span>
                      </div>
                      {result.conversation.mailbox_name ? (
                        <div className="mt-1 truncate text-xs text-muted-foreground">{result.conversation.mailbox_name}</div>
                      ) : null}
                    </div>

                    <div className="min-w-0">
                      <div className="mb-1 text-[11px] font-medium uppercase tracking-wide text-muted-foreground md:hidden">Match</div>
                      <div className="line-clamp-2 text-sm leading-5 text-foreground/85">
                        {resultHighlight(result)}
                      </div>
                      <div className="mt-1 text-xs text-muted-foreground">
                        <span>{fieldSummary(result.matched_fields)}</span>
                      </div>
                    </div>

                    <div className="min-w-0">
                      <div className="mb-1 text-[11px] font-medium uppercase tracking-wide text-muted-foreground md:hidden">Customer</div>
                      <div className="truncate text-sm text-foreground">
                        {result.conversation.customer_name || result.conversation.customer_email || 'Unknown customer'}
                      </div>
                      {result.conversation.customer_email && result.conversation.customer_name ? (
                        <div className="mt-1 truncate text-xs text-muted-foreground">{result.conversation.customer_email}</div>
                      ) : null}
                    </div>

                    <div className="min-w-0">
                      <div className="mb-1 text-[11px] font-medium uppercase tracking-wide text-muted-foreground md:hidden">State</div>
                      <div className="flex flex-wrap items-center gap-1.5">
                        <Badge variant="secondary" className={cn('h-5 shrink-0 px-1.5 text-[11px]', STATUS_LABELS[result.conversation.status as ConversationStatus] && 'font-medium')}>
                          {STATUS_LABELS[result.conversation.status as ConversationStatus] ?? result.conversation.status}
                        </Badge>
                        <Badge variant="outline" className="h-5 shrink-0 px-1.5 text-[11px]">
                          {PRIORITY_LABELS[result.conversation.priority as ConversationPriority] ?? result.conversation.priority}
                        </Badge>
                      </div>
                    </div>

                    <div className="min-w-0 text-sm text-muted-foreground">
                      <div className="mb-1 text-[11px] font-medium uppercase tracking-wide md:hidden">Updated</div>
                      <span>{formatDate(result.conversation.updated_at || result.conversation.created_at)}</span>
                    </div>

                    <div className="hidden items-center justify-end text-muted-foreground md:flex">
                      <ArrowRight02Icon className="h-4 w-4 transition-transform group-hover:translate-x-0.5" />
                    </div>
                  </button>
                ))}
              </div>
            </div>
          )}
        </div>

        {hasInput && totalPages > 1 && (
          <div className="flex h-14 shrink-0 items-center justify-between border-t px-6">
            <Button type="button" variant="outline" size="sm" disabled={page <= 1 || isFetching} onClick={() => setPage(page - 1)}>
              <ArrowLeft02Icon className="h-4 w-4" />
              Previous
            </Button>
            <span className="text-sm text-muted-foreground">Page {page} of {totalPages}</span>
            <Button type="button" variant="outline" size="sm" disabled={page >= totalPages || isFetching} onClick={() => setPage(page + 1)}>
              Next
              <ArrowRight02Icon className="h-4 w-4" />
            </Button>
          </div>
        )}
      </div>
    </div>
  );
}

function LabeledInput({
  label,
  value,
  onChange,
  placeholder,
  type = 'text',
}: {
  label: string;
  value: string;
  onChange: (value: string) => void;
  placeholder?: string;
  type?: string;
}) {
  return (
    <div className="min-w-0 space-y-1.5">
      <Label className="text-xs text-muted-foreground">{label}</Label>
      <Input
        type={type}
        className="h-8"
        value={value}
        placeholder={placeholder}
        onChange={(event) => onChange(event.target.value)}
      />
    </div>
  );
}

function SearchEmptyState({ title, description }: { title: string; description: string }) {
  return (
    <div className="flex h-full min-h-[320px] items-center justify-center px-6">
      <div className="max-w-md text-center">
        <div className="mx-auto mb-3 flex h-10 w-10 items-center justify-center rounded-md border bg-muted/30">
          <InboxIcon className="h-5 w-5 text-muted-foreground" />
        </div>
        <h2 className="text-base font-semibold">{title}</h2>
        <p className="mt-1 text-sm leading-6 text-muted-foreground">{description}</p>
      </div>
    </div>
  );
}
