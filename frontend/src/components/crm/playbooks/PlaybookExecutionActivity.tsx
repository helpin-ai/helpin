import { useState } from 'react';
import { Link } from '@tanstack/react-router';
import { QuietSection } from '@/components/design-system/quiet';
import { useCRMPlaybookAutomationActivity } from '@/hooks/queries/useCRMPlaybookAutomation';
import { PlaybookError, PlaybookLoading, PlaybookPagination } from './PlaybookUI';

export function PlaybookExecutionActivity({ ws, slug, id }: { ws: string; slug: string; id: string }) {
  const [page, setPage] = useState(1);
  const activity = useCRMPlaybookAutomationActivity(ws, id, page);
  const status: Record<string, string> = { queued: 'Check queued', running: 'Beacon is checking', completed: 'Check completed', failed: 'Check failed', cancelled: 'Check cancelled', paused: 'Check paused' };
  return <QuietSection title="Automation" className="max-w-3xl">
    {activity.isPending ? <PlaybookLoading /> : activity.isError ? <PlaybookError error={activity.error} retry={() => void activity.refetch()} /> : <>
      {!activity.data.total && <p className="text-sm text-quiet-text-tertiary">Connections, automation changes and Beacon’s checks will appear here.</p>}
      <ol className="divide-y divide-quiet-divider-light">{activity.data.data.map((event) => <li key={event.id} className="py-3">
        <p className="text-sm font-medium">{event.kind === 'connection' ? 'Connection published' : event.kind === 'check' ? status[event.status] || 'Check updated' : event.kind === 'signal_automation' ? event.status === 'enabled' ? 'Signal automation started' : 'Signal automation paused' : event.status === 'enabled' ? 'Automation settings saved · On' : 'Automation settings saved · Off'}</p>
        {event.situation_id && <Link to="/w/$slug/crm/insights" params={{ slug }} search={{ signal: event.situation_id }} className="mt-1 block text-sm text-quiet-accent hover:underline">{event.situation_title || 'Open signal'}</Link>}
        <p className="mt-1 text-xs text-quiet-text-tertiary">{new Date(event.occurred_at).toLocaleString()}</p>
      </li>)}</ol>
      <PlaybookPagination page={page} total={activity.data.total} busy={activity.isFetching} onChange={setPage} />
    </>}
  </QuietSection>;
}
