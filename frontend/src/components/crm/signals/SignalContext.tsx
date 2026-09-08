import { useState } from 'react';
import { Link } from '@tanstack/react-router';
import { QuietTextAction } from '@/components/design-system/quiet';
import { useCRMSituationHistory } from '@/hooks/queries/useCRMSituations';
import { useAssignableMembers } from '@/hooks/queries/useWorkspaces';
import { useCRMSignalFeedback } from '@/hooks/queries/useCRM';
import type { CRMSituationEvidence, CRMSituationItem } from '@/lib/crmSituationTypes';
import { PlaybookError, PlaybookLoading } from '../playbooks/PlaybookUI';

export function SignalEvidence({ ws, slug, item, canEdit }: { ws: string; slug: string; item: CRMSituationItem; canEdit: boolean }) {
  return <RecommendationEvidence ws={ws} slug={slug} evidence={item.evidence ?? []} canEdit={canEdit} />;
}
export function RecommendationEvidence({ ws, slug, evidence, canEdit }: { ws: string; slug: string; evidence: CRMSituationEvidence[]; canEdit: boolean }) {
  return <details className="border-b border-quiet-divider-strong py-4 text-sm"><summary className="cursor-pointer text-quiet-text-secondary">Evidence{evidence.length > 0 ? ` (${evidence.length})` : ''}</summary>
    <ul className="mt-3 divide-y divide-quiet-divider-light">{evidence.map((source) => <li key={source.id} className="py-3 first:pt-0"><p className="leading-6">{source.summary}</p>{source.evidence_excerpt && <blockquote className="mt-2 border-l border-quiet-divider-strong pl-3 text-xs leading-6 text-quiet-text-secondary">{source.evidence_excerpt}</blockquote>}<div className="mt-2 flex flex-wrap items-center gap-3 text-xs text-quiet-text-tertiary"><span>{source.source_type.replaceAll('_', ' ')} · {new Date(source.detected_at).toLocaleDateString()}</span><SourceLink slug={slug} source={source} /></div>{source.dismissed_at || source.superseded_at ? <p className="mt-2 text-xs text-quiet-accent">{source.dismissed_at ? 'Dismissed evidence' : 'Superseded evidence'} — do not rely on this as current information.</p> : <><EvidenceReview ws={ws} source={source} canEdit={canEdit} />{source.evidence_identity_trust !== 'verified' && <p className="mt-2 text-xs text-quiet-accent">Customer identity is not verified for this source.</p>}</>}</li>)}</ul>
    {!evidence.length && <p className="mt-3 text-xs text-quiet-text-tertiary">No linked source evidence is available for this signal.</p>}
  </details>;
}

function EvidenceReview({ ws, source, canEdit }: { ws: string; source: CRMSituationEvidence; canEdit: boolean }) {
  const review = useCRMSignalFeedback(ws, source.contact_id, undefined, source.company_id);
  if (source.reviewed_at) return <p className="mt-2 text-xs text-quiet-text-tertiary">Evidence reviewed</p>;
  if (!canEdit) return null;
  return <div className="mt-2"><QuietTextAction disabled={review.isPending} onClick={() => review.mutate({ signalId: source.id, action: 'reviewed' })}>{review.isPending ? 'Marking…' : 'Mark evidence reviewed'}</QuietTextAction>{review.isError && <PlaybookError error={review.error} />}</div>;
}

function SourceLink({ slug, source }: { slug: string; source: CRMSituationEvidence }) {
  const className = 'text-quiet-accent hover:underline';
  if (source.source_type === 'meeting' && source.source_id) return <Link className={className} to="/w/$slug/crm/meetings/$meetingId" params={{ slug, meetingId: source.source_id }}>Open meeting</Link>;
  if (source.source_type === 'support' && source.source_thread_id) return <Link className={className} to="/w/$slug/support/$conversationId" params={{ slug, conversationId: source.source_thread_id }}>Open conversation</Link>;
  if (source.source_type === 'email' && source.source_thread_id) {
    if (source.contact_id) return <Link className={className} to="/w/$slug/crm/contacts/$contactId" params={{ slug, contactId: source.contact_id }} search={{ tab: 'emails', thread: source.source_thread_id }}>Open email</Link>;
    if (source.company_id) return <Link className={className} to="/w/$slug/crm/companies/$companyId" params={{ slug, companyId: source.company_id }} search={{ tab: 'emails', thread: source.source_thread_id }}>Open email</Link>;
  }
  return null;
}

export function SignalHistory({ ws, id }: { ws: string; id: string }) {
  const [open, setOpen] = useState(false);
  return <details className="border-b border-quiet-divider-strong py-4 text-sm" onToggle={(event) => setOpen(event.currentTarget.open)}><summary className="cursor-pointer text-quiet-text-secondary">Activity</summary>{open && <HistoryEntries ws={ws} id={id} />}</details>;
}
function HistoryEntries({ ws, id }: { ws: string; id: string }) {
  const history = useCRMSituationHistory(ws, id);
  const members = useAssignableMembers(ws);
  const labels = { created: 'Signal created', update: 'Next step or ownership updated', pause: 'Signal paused', resume: 'Signal resumed', close: 'Outcome recorded', apply_playbook: 'Playbook applied', update_milestone: 'Milestone assessed' };
  if (history.isPending) return <PlaybookLoading />;
  if (history.isError) return <PlaybookError error={history.error} retry={() => void history.refetch()} />;
  return <><ol className="mt-3 space-y-4">{history.data.pages.flatMap((page) => page.data).map((change) => <li key={change.id}><p className="text-sm">{labels[change.operation]}</p><p className="mt-1 text-xs text-quiet-text-tertiary">{change.actor_kind === 'member' ? members.data?.find((member) => member.id === change.actor_member_id)?.display_name || 'Workspace member' : 'Helpin'} · {new Date(change.created_at).toLocaleString()}</p>{change.reason && <p className="mt-1 text-xs leading-5 text-quiet-text-secondary">{change.reason}</p>}{change.operation === 'close' && change.after.outcome_summary && <p className="mt-1 text-xs leading-5 text-quiet-text-secondary">{change.after.outcome_summary}</p>}</li>)}</ol>{!history.data.pages[0].data.length && <p className="mt-3 text-xs text-quiet-text-tertiary">No recorded activity.</p>}{history.hasNextPage && <QuietTextAction className="mt-3" disabled={history.isFetchingNextPage} onClick={() => void history.fetchNextPage()}>{history.isFetchingNextPage ? 'Loading…' : 'Load earlier activity'}</QuietTextAction>}</>;
}
