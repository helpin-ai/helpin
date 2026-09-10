import { useDeferredValue } from 'react';
import { Link } from '@tanstack/react-router';
import { QuietFilterDropdown, QuietEmptyState, QuietPageHeader, QuietPageViewport, QuietSearchInput, QuietTextAction } from '@/components/design-system/quiet';
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table';
import { Tabs, TabsList, TabsTrigger } from '@/components/ui/tabs';
import { PMFilterBar } from '@/components/pm/PMFilterControls';
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/tooltip';
import { useCRMSignalInbox } from '@/hooks/queries/useCRMSituations';
import { usePermissions, useWorkspaceAccess } from '@/hooks/queries/useSession';
import { useWorkspaceStore } from '@/stores/workspaceStore';
import { useTitle } from '@/hooks/useTitle';
import { timeAgo } from '@/lib/utils';
import { signalCategories } from '@/lib/crmSituationPresentation';
import { inboxFilterDefinitions, inboxNavigation, inboxStatus, parseSignalsSearch, type SignalsSearch } from '@/lib/crmSignalInboxQueryBuilder';
import type { CRMSignalInboxItem } from '@/lib/crmSignalInboxTypes';
import { PlaybookError, PlaybookHelp, PlaybookLoading, PlaybookPagination } from '@/components/crm/playbooks/PlaybookUI';
import { SignalDrawer } from '@/components/crm/signals/SignalDrawer';
import { RecommendationDrawer } from '@/components/crm/signals/RecommendationDrawer';
import { SignalGroupDrawer } from '@/components/crm/signals/SignalGroupDrawer';

interface Props { search: SignalsSearch; onChange: (search: SignalsSearch, replace?: boolean) => void }

function SignalReceivedTime({ item }: { item: CRMSignalInboxItem }) {
  const received = new Date(item.created_at);
  if (Number.isNaN(received.getTime())) return <span aria-label="Arrival time unavailable">—</span>;
  const exactTime = received.toLocaleString(undefined, { dateStyle: 'medium', timeStyle: 'long' });
  return <Tooltip>
    <TooltipTrigger asChild><time dateTime={item.created_at} tabIndex={0} className="rounded-sm focus-visible:outline-2 focus-visible:outline-quiet-field">{timeAgo(received)}</time></TooltipTrigger>
    <TooltipContent>{item.kind === 'evidence' ? 'Latest signal received' : 'Received'}: {exactTime}</TooltipContent>
  </Tooltip>;
}

export function SignalsPage({ search, onChange }: Props) {
  useTitle('Signals');
  const workspace = useWorkspaceStore((state) => state.currentWorkspace);
  const access = useWorkspaceAccess(workspace?.id || '');
  const { has } = usePermissions(access.data);
  if (access.isPending) return <QuietPageViewport><PlaybookLoading /></QuietPageViewport>;
  if (access.isError) return <QuietPageViewport><PlaybookError error={access.error} retry={() => void access.refetch()} /></QuietPageViewport>;
  if (!workspace || !has('crm.read')) return <QuietPageViewport><QuietEmptyState title="CRM access required" description="Ask a workspace admin for access to Signals." /></QuietPageViewport>;
  return <SignalsList key={workspace.id} ws={workspace.id} slug={workspace.slug} canEdit={has('crm.edit')} search={parseSignalsSearch({ ...search })} onChange={onChange} />;
}

function SignalsList({ ws, slug, canEdit, search, onChange }: Props & { ws: string; slug: string; canEdit: boolean }) {
  const filters = { ...search, signal: undefined, recommendation: undefined, group: undefined, view: undefined };
  const query = useDeferredValue(search.q?.trim() || '');
  const list = useCRMSignalInbox(ws, { ...filters, q: query });
  const scope = filters.scope || 'all';
  const state = filters.state || 'needs_attention';
  const category = filters.category || 'all';
  const change = (values: Partial<SignalsSearch>, replace = false) => onChange({ ...search, ...values, page: 1 }, replace);
  const select = (item?: Pick<CRMSignalInboxItem, 'id' | 'kind'>) => onChange({ ...search, signal: item?.kind === 'situation' ? item.id : undefined, recommendation: item?.kind === 'recommendation' ? item.id : undefined, group: item?.kind === 'evidence' ? item.id : undefined });
  const clear = (status: string) => onChange({ signal: search.signal, recommendation: search.recommendation, group: search.group, sort: search.sort, scope: 'all', state: status, category: 'all', page: 1 });
  const filtered = !!query || scope !== 'all' || state !== 'all' || category !== 'all';
  const filterValues = { scope: scope === 'all' ? [] : [scope], state: state === 'all' ? [] : [state], category: category === 'all' ? [] : [category] };
  const visibleKeys = new Set(inboxFilterDefinitions.filter(({ key }) => filterValues[key].length > 0).map(({ key }) => key));
  return <div className="flex h-full min-h-0 flex-col overflow-hidden">
    <QuietPageHeader variant="shell" title="Signals" description="Know what needs attention. Agree the next step." actions={<div className="flex flex-wrap items-center gap-4 text-sm">
      <Link to="/w/$slug/crm/insights" params={{ slug }} search={{ view: 'evidence' }} className="text-quiet-text-secondary hover:text-quiet-text-primary">Evidence</Link>
      <Link to="/w/$slug/crm/playbooks" params={{ slug }} className="text-quiet-text-secondary hover:text-quiet-text-primary">Playbooks</Link>
    </div>} />
    <Tabs className="shrink-0" value={category} onValueChange={(value) => change({ category: value })}>
      <TabsList variant="quiet" aria-label="Signal category" className="h-auto w-full justify-start overflow-x-auto px-4 md:px-6">
        {signalCategories.map((entry) => <TabsTrigger key={entry.value} value={entry.value} className="shrink-0 gap-2">{entry.label}<span className="text-xs tabular-nums text-quiet-text-tertiary">{list.isSuccess ? list.data.category_counts[entry.value] : '—'}</span></TabsTrigger>)}
      </TabsList>
    </Tabs>
    <div className="flex shrink-0 flex-wrap items-center gap-x-3 gap-y-2 border-b border-quiet-divider-strong px-4 py-2 md:px-6">
      <QuietSearchInput aria-label="Search signals" placeholder="Search customers or signals…" maxLength={500} value={search.q || ''} onChange={(event) => change({ q: event.target.value }, true)} containerClassName="w-64 max-w-full" />
      <QuietFilterDropdown label="Assignment" value={scope} onChange={(value) => change({ scope: value })} options={inboxNavigation.scope} />
      <QuietFilterDropdown label="Signal status" value={state} onChange={(value) => change({ state: value })} options={inboxNavigation.state} />
      {visibleKeys.size === 0 && !!search.q?.trim() && <QuietTextAction onClick={() => clear('needs_attention')}>Clear filters</QuietTextAction>}
      <span className="inline-flex items-center sm:ml-auto"><span className="text-xs text-quiet-text-tertiary">Sort</span><QuietFilterDropdown label="Sort signals" value={filters.sort || 'priority'} onChange={(value) => change({ sort: value })} options={inboxNavigation.sort} /></span>
    </div>
    <div className="shrink-0">
      <PMFilterBar definitions={inboxFilterDefinitions} values={filterValues} visibleKeys={visibleKeys}
        onToggle={(key, value) => change({ [key]: filterValues[key].includes(value) ? 'all' : value })}
        onRemove={(key) => change({ [key]: 'all' })} onClearAll={() => clear('needs_attention')} />
    </div>
    <QuietPageViewport className="min-h-0 flex-1">
    {list.isPending ? <PlaybookLoading /> : list.isError ? <PlaybookError error={list.error} retry={() => void list.refetch()} /> : <>
      {category === 'all' && list.data.uncategorized_count > 0 && <p className="py-3 text-xs text-quiet-text-tertiary">{list.data.uncategorized_count} {list.data.uncategorized_count === 1 ? 'signal needs' : 'signals need'} customer context before a category can be determined. Included in All.</p>}
      {list.data.data.length ? <Table aria-label="Signals" aria-busy={list.isFetching || (search.q?.trim() || '') !== query}>
        <TableHeader><TableRow><TableHead>Signal</TableHead><TableHead>Customer</TableHead><TableHead>Category</TableHead><TableHead>Owner</TableHead><TableHead><span className="inline-flex items-center">Priority<PlaybookHelp label="About signal priority">Based on signal importance, recency, evidence confidence, and available deal context. High: 15 or above. Medium: 8 to below 15. Low: below 8.</PlaybookHelp></span></TableHead></TableRow></TableHeader>
        <TableBody>{list.data.data.map((item) => <TableRow key={item.kind + ':' + item.id} className="cursor-pointer" onClick={(event) => { if (!(event.target as HTMLElement).closest('a,button')) select(item); }}>
          <TableCell className="min-w-72 max-w-xl whitespace-normal"><button type="button" onClick={() => select(item)} className="text-left font-semibold leading-6 text-quiet-text-primary underline-offset-4 hover:underline focus-visible:outline-2 focus-visible:outline-quiet-field">{item.title}</button>{item.next_step && <p className="mt-1 text-xs leading-5 text-quiet-text-secondary">{item.next_step}</p>}<div className="mt-1 flex flex-wrap items-center gap-x-2 gap-y-1 text-xs text-quiet-text-tertiary"><span>{inboxStatus(item)}</span><span aria-hidden="true">·</span><span className="whitespace-nowrap">Received <SignalReceivedTime item={item} /></span></div></TableCell>
          <TableCell className="min-w-40 max-w-56 whitespace-normal text-quiet-text-secondary">{item.customer_name || 'Customer unavailable'}</TableCell>
          <TableCell className="max-w-40 whitespace-normal text-xs text-quiet-text-secondary">{signalCategories.find((entry) => entry.value === item.category)?.label || 'Needs customer context'}</TableCell>
          <TableCell className="min-w-32 max-w-44 whitespace-normal">{item.owner_name || 'Unassigned'}{item.owner_member_id && !item.owner_available && <span className="block text-xs text-quiet-accent">Unavailable</span>}</TableCell>
          <TableCell className="text-sm tabular-nums text-quiet-text-secondary"><Tooltip><TooltipTrigger asChild><span tabIndex={0}>{{ high: 'High', medium: 'Medium', low: 'Low', unscored: 'Not scored' }[item.priority_band]}</span></TooltipTrigger><TooltipContent>{item.priority === null ? 'This recommendation has no signal priority score yet.' : 'Signal priority score: ' + item.priority.toLocaleString(undefined, { maximumFractionDigits: 1 })}</TooltipContent></Tooltip></TableCell>
        </TableRow>)}</TableBody>
      </Table> : <QuietEmptyState title={filtered ? 'No signals match this view' : 'No signals yet'} description={filtered ? 'Try another category, assignment, or status.' : 'Customer activity and recommendations will appear here when there is something to follow up on.'} action={filtered ? <QuietTextAction onClick={() => clear('all')}>Show all signals</QuietTextAction> : <Link className="text-sm text-quiet-accent hover:underline" to="/w/$slug/crm/insights" params={{ slug }} search={{ view: 'evidence' }}>Explore customer evidence</Link>} />}
      <PlaybookPagination page={filters.page || 1} total={list.data.total} pageSize={list.data.page_size} onChange={(page) => onChange({ ...search, page })} busy={list.isFetching} />
    </>}
    </QuietPageViewport>
    {search.signal ? <SignalDrawer key={search.signal} ws={ws} slug={slug} signalId={search.signal} canEdit={canEdit} onClose={() => select()} /> : search.recommendation ? <RecommendationDrawer key={search.recommendation} ws={ws} slug={slug} id={search.recommendation} canEdit={canEdit} onClose={() => select()} onSignal={(id) => select({ id, kind: 'situation' })} /> : search.group && <SignalGroupDrawer key={search.group} ws={ws} slug={slug} id={search.group} canEdit={canEdit} onClose={() => select()} />}
  </div>;
}
