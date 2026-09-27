import { useState } from 'react';
import { useInfiniteQuery } from '@tanstack/react-query';
import { format, formatDistanceToNow, parseISO } from 'date-fns';
import { QuietTextAction } from '@/components/design-system/quiet';
import { UserAvatar } from '@/components/pm/UserAvatar';
import { pmObjectiveService } from '@/lib/services/pmObjectiveService';
import { queryKeys } from '@/lib/queryKeys';
import { unwrap } from '@/lib/queryUtils';
import type { ActivityLogEntry, KeyResult } from '@/lib/pmTypes';

const numberFormat = new Intl.NumberFormat(undefined, { maximumFractionDigits: 20 });

function HistoryEntry({ entry, memberMap }: { entry: ActivityLogEntry; memberMap: Map<string, string> }) {
  const { activity, actor } = entry;
  const name = actor?.full_name || (activity.actor_id ? memberMap.get(activity.actor_id) : undefined) || (activity.actor_id ? 'Unknown user' : 'System');
  const raw = activity.new_value ?? '';
  const value = Number(raw);
  const type = activity.metadata?.result_type;
  const rendered = type === 'boolean'
    ? (value >= Number(activity.metadata?.target_value ?? 1) ? 'Complete' : 'Incomplete')
    : `${Number.isFinite(value) && raw !== '' ? numberFormat.format(value) : raw}${type === 'percent' ? '%' : ''}`;
  return <div className="flex items-start gap-2 text-xs leading-5 text-quiet-text-tertiary">
    <UserAvatar name={name} avatarUrl={actor?.avatar_url} avatarStyle={actor?.avatar_style} avatarSeed={actor?.avatar_seed}
      avatarBackgroundMode={actor?.avatar_background_mode} avatarBackgroundColor={actor?.avatar_background_color} className="mt-0.5 size-4 shrink-0" />
    <p className="min-w-0">
      <span className="font-medium text-quiet-text-secondary">{name}</span> changed the current value to <span className="font-medium text-quiet-text-secondary">{rendered}</span>
      <span aria-hidden="true" className="mx-2 text-quiet-muted">·</span>
      <time className="whitespace-nowrap" dateTime={activity.created_at} title={format(parseISO(activity.created_at), 'PPpp')}>{formatDistanceToNow(parseISO(activity.created_at), { addSuffix: true })}</time>
    </p>
  </div>;
}

export function KeyResultHistory({ kr, workspaceId, memberMap }: { kr: KeyResult; workspaceId: string; memberMap: Map<string, string> }) {
  const [expanded, setExpanded] = useState(false);
  const history = useInfiniteQuery({
    queryKey: [...queryKeys.pm.keyResultActivity(workspaceId, kr.id), kr.updated_at],
    initialPageParam: 1,
    queryFn: async ({ pageParam }) => unwrap(await pmObjectiveService.keyResultActivity(workspaceId, kr.id, pageParam)),
    getNextPageParam: page => page.page < page.total_pages ? page.page + 1 : undefined,
    staleTime: 30_000,
  });
  const entries = history.data?.pages.flatMap(page => page.data) ?? [];
  const total = history.data?.pages[0]?.total ?? 0;
  const updatedBy = kr.updated_by ? memberMap.get(kr.updated_by) : undefined;
  return <div className="mt-2">
    {entries[0] ? <HistoryEntry entry={entries[0]} memberMap={memberMap} /> : <p className="text-xs leading-5 text-quiet-text-tertiary">
      {history.isPending ? 'Loading updates…' : <>Updated {formatDistanceToNow(parseISO(kr.updated_at), { addSuffix: true })}{updatedBy ? ` by ${updatedBy}` : ''}</>}
    </p>}
    {history.isError && <p role="alert" className="mt-1 flex flex-wrap items-center gap-2 text-xs text-destructive">Couldn’t load result history.<QuietTextAction onClick={() => void history.refetch()}>Retry</QuietTextAction></p>}
    {total > 1 && <QuietTextAction className="mt-1 text-xs text-quiet-text-tertiary" aria-expanded={expanded} aria-controls={`history-${kr.id}`} onClick={() => setExpanded(value => !value)}>
      {expanded ? 'Hide earlier updates' : `Show ${total - 1} earlier update${total === 2 ? '' : 's'}`}
    </QuietTextAction>}
    {expanded && <div id={`history-${kr.id}`} className="ml-2 mt-2 border-l border-quiet-divider-light pl-4">
      <ol aria-label={`Earlier updates for ${kr.name}`} className="space-y-2">
        {entries.slice(1).map(entry => <li key={entry.activity.id}><HistoryEntry entry={entry} memberMap={memberMap} /></li>)}
      </ol>
      {history.hasNextPage && <QuietTextAction className="mt-2 text-xs" disabled={history.isFetchingNextPage} onClick={() => void history.fetchNextPage()}>{history.isFetchingNextPage ? 'Loading…' : 'Show more updates'}</QuietTextAction>}
    </div>}
  </div>;
}
