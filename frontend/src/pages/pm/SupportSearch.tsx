import { useEffect, useMemo, useState, type FormEvent, type ReactNode } from 'react';
import { useNavigate, useSearch } from '@tanstack/react-router';
import { Search01Icon, Cancel01Icon, ArrowLeft02Icon, ArrowRight02Icon, InboxIcon } from '@/lib/icons';
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
  SupportConversationSearchParams,
  SupportConversationSearchResult,
  SupportSearchHighlight,
} from '@/lib/pmTypes';
import { PRIORITY_LABELS, STATUS_LABELS } from '@/components/support/constants';

export type SupportSearchRouteSearch = SupportConversationSearchParams;

const SEARCH_PER_PAGE = 50;
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
      <div className="border-b px-6 py-4">
        <form onSubmit={applySearch} className="space-y-4">
          <div className="flex flex-col gap-3 lg:flex-row lg:items-center">
            <div className="relative min-w-0 flex-1">
              <Search01Icon className="pointer-events-none absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
              <Input
                autoFocus
                className="h-10 pl-9"
                value={draft.q ?? ''}
                maxLength={256}
                onChange={(event) => updateDraft('q', event.target.value)}
                placeholder="Search support by email, #number, title, customer, or message"
              />
            </div>
            <div className="flex shrink-0 items-center gap-2">
              <Button type="submit" className="h-10 gap-2">
                <Search01Icon className="h-4 w-4" />
                Search
              </Button>
              <Button type="button" variant="ghost" size="icon" className="h-10 w-10" onClick={clearSearch} aria-label="Clear search">
                <Cancel01Icon className="h-4 w-4" />
              </Button>
            </div>
          </div>

          <div className="grid gap-3 md:grid-cols-2 xl:grid-cols-5">
            <FilterSelect label="Status" value={draft.statuses ?? 'any'} onValueChange={(value) => updateDraft('statuses', value === 'any' ? undefined : value)}>
              <SelectItem value="any">Any status</SelectItem>
              {Object.entries(STATUS_LABELS).map(([value, label]) => (
                <SelectItem key={value} value={value}>{label}</SelectItem>
              ))}
            </FilterSelect>
            <FilterSelect label="Priority" value={draft.priorities ?? 'any'} onValueChange={(value) => updateDraft('priorities', value === 'any' ? undefined : value)}>
              <SelectItem value="any">Any priority</SelectItem>
              {Object.entries(PRIORITY_LABELS).map(([value, label]) => (
                <SelectItem key={value} value={value}>{label}</SelectItem>
              ))}
            </FilterSelect>
            <FilterSelect label="Assignee" value={draft.assigned_to ?? 'any'} onValueChange={(value) => updateDraft('assigned_to', value === 'any' ? undefined : value)}>
              <SelectItem value="any">Anyone</SelectItem>
              <SelectItem value="me">Assigned to me</SelectItem>
              <SelectItem value="mentioned_me">Mentioned me</SelectItem>
              <SelectItem value="opened_by_me">Opened by me</SelectItem>
              <SelectItem value="unassigned">Unassigned</SelectItem>
            </FilterSelect>
            <FilterSelect label="Inbox" value={draft.mailbox_ids ?? 'any'} onValueChange={(value) => updateDraft('mailbox_ids', value === 'any' ? undefined : value)}>
              <SelectItem value="any">All inboxes</SelectItem>
              <SelectItem value="shared">Main inbox</SelectItem>
              {mailboxes.map((mailbox) => (
                <SelectItem key={mailbox.id} value={mailbox.id}>{mailbox.name}</SelectItem>
              ))}
            </FilterSelect>
            <FilterSelect label="Sort" value={draft.sort ?? 'relevance'} onValueChange={(value) => updateDraft('sort', value)}>
              <SelectItem value="relevance">Relevance</SelectItem>
              <SelectItem value="newest">Newest</SelectItem>
              <SelectItem value="oldest">Oldest</SelectItem>
            </FilterSelect>
          </div>

          <div className="grid gap-3 md:grid-cols-2 xl:grid-cols-5">
            <LabeledInput label="Customer email" value={draft.customer_email ?? ''} onChange={(value) => updateDraft('customer_email', value)} placeholder="customer@example.com" />
            <LabeledInput label="Conversation title" value={draft.title ?? ''} onChange={(value) => updateDraft('title', value)} placeholder="Title contains" />
            <FilterSelect label="Tag" value={draft.tag_ids ?? 'any'} onValueChange={(value) => updateDraft('tag_ids', value === 'any' ? undefined : value)}>
              <SelectItem value="any">Any tag</SelectItem>
              {tags.map((tag) => (
                <SelectItem key={tag.id} value={tag.id}>{tag.name}</SelectItem>
              ))}
            </FilterSelect>
            <FilterSelect label="AI state" value={draft.ai ?? 'any'} onValueChange={(value) => updateDraft('ai', value === 'any' ? undefined : value)}>
              <SelectItem value="any">Any AI state</SelectItem>
              <SelectItem value="handling">AI handling</SelectItem>
              <SelectItem value="handoff">AI handoff</SelectItem>
              <SelectItem value="resolved">AI resolved</SelectItem>
            </FilterSelect>
            <div className="grid grid-cols-2 gap-2">
              <LabeledInput label="From" type="date" value={draft.created_from ?? ''} onChange={(value) => updateDraft('created_from', value)} />
              <LabeledInput label="To" type="date" value={draft.created_to ?? ''} onChange={(value) => updateDraft('created_to', value)} />
            </div>
          </div>
        </form>
      </div>

      <div className="flex min-h-0 flex-1 flex-col">
        <div className="flex h-12 shrink-0 items-center justify-between border-b px-6">
          <div className="text-sm text-muted-foreground">
            {hasInput ? (
              <>
                <span className="font-medium text-foreground">{total}</span>
                {data?.meta.total_capped ? '+' : ''} result{total === 1 ? '' : 's'}
                {data?.meta.total_capped ? `, capped at ${data.meta.total_cap}` : ''}
              </>
            ) : (
              'Search all support conversations'
            )}
          </div>
          {isFetching && <span className="text-xs text-muted-foreground">Searching...</span>}
        </div>

        <div className="min-h-0 flex-1 overflow-y-auto">
          {!hasInput ? (
            <SearchEmptyState
              title="Search every support conversation"
              description="Use the search field or filters to find conversations by customer email, conversation number, title, or message text."
            />
          ) : error ? (
            <SearchEmptyState title="Search failed" description={error.message} />
          ) : results.length === 0 && !isFetching ? (
            <SearchEmptyState title="No conversations found" description="Try a broader query or remove a filter." />
          ) : (
            <div className="divide-y">
              {results.map((result) => (
                <button
                  key={result.conversation.id}
                  type="button"
                  className="grid w-full grid-cols-[minmax(0,1fr)_auto] gap-4 px-6 py-4 text-left transition-colors hover:bg-muted/50 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
                  onClick={() => openConversation(result.conversation.id)}
                >
                  <div className="min-w-0 space-y-2">
                    <div className="flex min-w-0 items-center gap-2">
                      <span className="shrink-0 font-mono text-xs text-muted-foreground">#{result.display_id}</span>
                      <span className="truncate text-sm font-semibold">{result.conversation.subject}</span>
                      <Badge variant="secondary" className={cn('h-5 shrink-0 px-1.5 text-[11px]', STATUS_LABELS[result.conversation.status as ConversationStatus] && 'font-medium')}>
                        {STATUS_LABELS[result.conversation.status as ConversationStatus] ?? result.conversation.status}
                      </Badge>
                      <Badge variant="outline" className="h-5 shrink-0 px-1.5 text-[11px]">
                        {PRIORITY_LABELS[result.conversation.priority as ConversationPriority] ?? result.conversation.priority}
                      </Badge>
                    </div>
                    <div className="line-clamp-2 text-sm leading-5 text-muted-foreground">
                      {resultHighlight(result)}
                    </div>
                    <div className="flex flex-wrap items-center gap-x-3 gap-y-1 text-xs text-muted-foreground">
                      <span>{fieldSummary(result.matched_fields)}</span>
                      {result.conversation.customer_email && <span>{result.conversation.customer_email}</span>}
                      {result.conversation.mailbox_name && <span>{result.conversation.mailbox_name}</span>}
                      {result.conversation.created_at && <span>{formatDate(result.conversation.created_at)}</span>}
                    </div>
                  </div>
                  <div className="hidden items-center text-muted-foreground sm:flex">
                    <ArrowRight02Icon className="h-4 w-4" />
                  </div>
                </button>
              ))}
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

function FilterSelect({
  label,
  value,
  onValueChange,
  children,
}: {
  label: string;
  value: string;
  onValueChange: (value: string) => void;
  children: ReactNode;
}) {
  return (
    <div className="min-w-0 space-y-1.5">
      <Label className="text-xs text-muted-foreground">{label}</Label>
      <Select value={value} onValueChange={onValueChange} size="sm">
        <SelectTrigger className="h-8 w-full rounded-md bg-background">
          <SelectValue />
        </SelectTrigger>
        <SelectContent>{children}</SelectContent>
      </Select>
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
