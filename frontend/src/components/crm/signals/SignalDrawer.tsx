import { useState } from 'react';
import { Link } from '@tanstack/react-router';
import { toast } from 'sonner';
import { QuietSelect, QuietPropertyRow, QuietSection, QuietStatusText, QuietTextAction, QuietPrimaryAction } from '@/components/design-system/quiet';
import { currentRecommendationText, signalOverviewText, SignalArrival, SignalDrawerLayout, repeatsSignalText } from './SignalDrawerLayout';
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog';
import { useCRMPlaybookSignal, useCRMPlaybookVersion, useCRMPlaybookWrite } from '@/hooks/queries/useCRMPlaybooks';
import { useAssignableMembers } from '@/hooks/queries/useWorkspaces';
import { createPlaybookIntentKey } from '@/lib/crmPlaybookPresentation';
import type { CRMPlaybookMilestone, CRMPlaybookMilestoneProgress } from '@/lib/crmPlaybookTypes';
import type { CRMSituationItem } from '@/lib/crmSituationTypes';
import { PlaybookError, PlaybookField, PlaybookHelp, PlaybookLoading, PlaybookTextarea } from '../playbooks/PlaybookUI';
import { PlaybookSignalEdit } from '../playbooks/PlaybookSignalEdit';
import { SignalActions } from './SignalActions';
import { SignalAutomation } from './SignalAutomation';
import { SignalEvidence, SignalHistory } from './SignalContext';
import { signalStatus } from '@/lib/crmSituationPresentation';

export function SignalDrawer({ ws, slug, playbookId, signalId, canEdit, onClose }: { ws: string; slug: string; playbookId?: string; signalId: string; canEdit: boolean; onClose: () => void }) {
  const signal = useCRMPlaybookSignal(ws, signalId);
  return <SignalDrawerLayout title={signal.data?.situation.title || 'Signal'} description="Customer progress, milestones, and recorded outcomes." onClose={onClose}>
    {signal.isPending ? <PlaybookLoading /> : signal.isError ? <PlaybookError error={signal.error} retry={() => void signal.refetch()} /> : playbookId && signal.data.situation.playbook_id !== playbookId ? <PlaybookError error={new Error('This signal is not in this playbook. Refresh the list.')} /> : <SignalProgress ws={ws} slug={slug} playbookId={signal.data.situation.playbook_id} expandMilestones={!!playbookId} item={signal.data} canEdit={canEdit} onReload={() => void signal.refetch()} />}
  </SignalDrawerLayout>;
}

function SignalProgress({ ws, slug, playbookId, expandMilestones, item, canEdit, onReload }: { ws: string; slug: string; playbookId?: string; expandMilestones: boolean; item: CRMSituationItem; canEdit: boolean; onReload: () => void }) {
  const signal = item.situation;
  const version = useCRMPlaybookVersion(ws, playbookId || '', signal.playbook_version_id);
  const members = useAssignableMembers(ws);
  const [assess, setAssess] = useState<{ milestone: CRMPlaybookMilestone; progress?: CRMPlaybookMilestoneProgress; revision: number }>();
  const [lifecycle, setLifecycle] = useState<{ operation: 'pause' | 'resume' | 'close'; revision: number }>();
  const [editing, setEditing] = useState<CRMSituationItem>();
  const [milestonesOpen, setMilestonesOpen] = useState(expandMilestones);
  const name = (id: string | null) => members.data?.find((member) => member.id === id)?.display_name || 'Workspace member';
  return <>
    <div className="flex flex-wrap items-center gap-3 pb-3 text-sm">
      {signal.company_id ? <Link to="/w/$slug/crm/companies/$companyId" params={{ slug, companyId: signal.company_id }} className="text-quiet-accent hover:underline">{item.company_name || 'View company'}</Link> : signal.contact_id ? <Link to="/w/$slug/crm/contacts/$contactId" params={{ slug, contactId: signal.contact_id }} className="text-quiet-accent hover:underline">{item.contact_name || 'View contact'}</Link> : signal.deal_id ? <Link to="/w/$slug/crm/deals/$dealId" params={{ slug, dealId: signal.deal_id }} className="text-quiet-accent hover:underline">{item.deal_name || 'View deal'}</Link> : <span className="text-quiet-text-tertiary">Customer unavailable</span>}
      <QuietStatusText>{signalStatus(item)}</QuietStatusText>
      <SignalArrival at={signal.created_at} />
    </div>
    <QuietSection className="px-0 py-4 sm:px-0 lg:px-0" title={signal.lifecycle === 'closed' ? 'Recorded outcome' : 'Next step'} action={canEdit && signal.lifecycle !== 'closed' && <QuietTextAction onClick={() => setEditing(item)}>Edit</QuietTextAction>}>
      <p className="text-sm font-medium leading-6 text-quiet-text-primary">{signal.lifecycle === 'closed' ? signal.outcome_summary : signal.next_step || 'No next step set.'}</p>
      {signal.lifecycle === 'closed' ? <p className="mt-2 text-xs text-quiet-text-tertiary">{signal.outcome_kind === 'achieved' ? 'Outcome achieved' : signal.outcome_kind?.replaceAll('_', ' ')} · Recorded by {name(signal.closed_by_member_id)}{signal.closed_at && ` · ${new Date(signal.closed_at).toLocaleDateString()}`}</p> : <>
        <QuietPropertyRow label="Owner" value={item.owner_name || 'Unassigned'} />
        {signal.next_action_owner_member_id !== signal.owner_member_id && <QuietPropertyRow label="Next action" value={item.next_action_owner_name || 'Unassigned'} />}
        {signal.next_checkpoint_at && <QuietPropertyRow label="Next check" value={new Date(signal.next_checkpoint_at).toLocaleString()} />}
      </>}
      {canEdit && signal.lifecycle !== 'closed' && <div className="mt-3 flex gap-2"><QuietTextAction onClick={() => setLifecycle({ operation: signal.lifecycle === 'paused' ? 'resume' : 'pause', revision: signal.revision })}>{signal.lifecycle === 'paused' ? 'Resume' : 'Pause'}</QuietTextAction><QuietTextAction onClick={() => setLifecycle({ operation: 'close', revision: signal.revision })}>Record outcome</QuietTextAction></div>}
    </QuietSection>
    <SignalActions ws={ws} slug={slug} item={item} canEdit={canEdit} />
    {playbookId && <SignalAutomation ws={ws} item={item} canEdit={canEdit} />}
    {playbookId && <details open={milestonesOpen} onToggle={(event) => setMilestonesOpen(event.currentTarget.open)} className="border-b border-quiet-divider-strong py-4 text-sm"><summary className="cursor-pointer text-quiet-text-secondary">Playbook progress · {(signal.playbook_milestones ?? []).filter((entry) => entry.status === 'achieved').length} / {signal.playbook_milestones?.length ?? 0} milestones achieved</summary><QuietSection className="border-0 px-0 sm:px-0 lg:px-0" title="Milestones" action={<PlaybookHelp label="About milestone progress">Milestones are assessed by your team against the published criteria. Completing every milestone does not automatically close a signal.</PlaybookHelp>}>
      {version.isPending ? <PlaybookLoading /> : version.isError ? <PlaybookError error={version.error} retry={() => void version.refetch()} /> : <>
        <p className="mb-4 text-xs text-quiet-text-tertiary"><Link className="text-quiet-accent hover:underline" to="/w/$slug/crm/playbooks/$playbookId" params={{ slug, playbookId }}>{version.data.definition.name}</Link> · Version {version.data.version}</p>
        <ol className="divide-y divide-quiet-divider-light">{version.data.definition.milestones?.map((milestone) => {
          const progress = signal.playbook_milestones?.find((entry) => entry.key === milestone.key);
          const status = progress?.status ?? 'pending';
          return <li key={milestone.key} className="py-4 first:pt-0"><div className="flex items-start justify-between gap-4"><p className="text-sm font-semibold">{milestone.name}</p><QuietStatusText tone={status === 'achieved' ? 'positive' : 'neutral'}>{status === 'achieved' ? 'Achieved' : status === 'not_applicable' ? 'Not applicable' : 'Pending'}</QuietStatusText></div><p className="mt-1 text-xs leading-6 text-quiet-text-secondary">{milestone.success_criteria}</p>{progress?.summary && <div className="mt-2 text-xs leading-5 text-quiet-text-tertiary"><p>{progress.summary}</p><p className="mt-1">Assessed by {name(progress.assessed_by_member_id)}{progress.assessed_at && ` · ${new Date(progress.assessed_at).toLocaleDateString()}`}</p></div>}{canEdit && signal.lifecycle === 'open' && <QuietTextAction className="mt-2" onClick={() => setAssess({ milestone, progress, revision: signal.revision })}>{status === 'pending' ? 'Record progress' : 'Update assessment'}</QuietTextAction>}</li>;
        })}</ol>
      </>}
    </QuietSection></details>}
    <SignalEvidence ws={ws} slug={slug} item={item} canEdit={canEdit} />
    {signal.objective?.trim() && !repeatsSignalText(signal.objective, [...signalOverviewText(item), ...currentRecommendationText(item.actions ?? [])]) && <details className="border-b border-quiet-divider-strong py-4 text-sm"><summary className="cursor-pointer text-quiet-text-secondary">Customer objective</summary><p className="mt-3 leading-7">{signal.objective}</p></details>}
    <SignalHistory ws={ws} id={signal.id} origin={signal.origin_kind === 'manual' ? 'Created by a workspace member.' : signal.origin_kind === 'signal' ? 'Created from CRM signal evidence. Open the customer record for context.' : 'Created from a CRM suggestion. Open the customer record for context.'} />
    {canEdit && assess && playbookId && <MilestoneAssessment ws={ws} playbookId={playbookId} signalId={signal.id} selection={assess} onClose={() => setAssess(undefined)} onReload={onReload} />}
    {canEdit && lifecycle && <SignalLifecycle ws={ws} signalId={signal.id} selection={lifecycle} onClose={() => setLifecycle(undefined)} onReload={onReload} />}
    {canEdit && editing && <PlaybookSignalEdit ws={ws} item={editing} onClose={() => setEditing(undefined)} onReload={onReload} />}
  </>;
}

function MilestoneAssessment({ ws, playbookId, signalId, selection, onClose, onReload }: { ws: string; playbookId: string; signalId: string; selection: { milestone: CRMPlaybookMilestone; progress?: CRMPlaybookMilestoneProgress; revision: number }; onClose: () => void; onReload: () => void }) {
  const [status, setStatus] = useState<CRMPlaybookMilestoneProgress['status']>(selection.progress?.status ?? 'pending');
  const [summary, setSummary] = useState(selection.progress?.summary || '');
  const [intentKey] = useState(createPlaybookIntentKey);
  const write = useCRMPlaybookWrite(ws);
  const save = async () => {
    const intent = { expected_revision: selection.revision, milestone_key: selection.milestone.key, status, summary: summary.trim() };
    try { await write.mutateAsync({ kind: 'milestone', id: playbookId, signalId, body: { ...intent, command_key: intentKey(intent) } }); toast.success('Progress recorded'); onClose(); } catch { /* Retain assessment and original revision. */ }
  };
  return <Dialog open onOpenChange={(open) => { if (!open && !write.isPending) onClose(); }}><DialogContent><DialogHeader><DialogTitle>{selection.milestone.name}</DialogTitle><DialogDescription>{selection.milestone.success_criteria}</DialogDescription></DialogHeader>
    <form onSubmit={(event) => { event.preventDefault(); void save(); }} className="space-y-5">
      <PlaybookField label="Status">{(id) => <QuietSelect id={id} label="Milestone status" value={status} disabled={write.isPending} options={[{ value: 'achieved', label: 'Achieved' }, { value: 'pending', label: 'Pending' }, { value: 'not_applicable', label: 'Not applicable' }]} onChange={(value) => setStatus(value as CRMPlaybookMilestoneProgress['status'])} />}</PlaybookField>
      <PlaybookField label="What supports this assessment?">{(id) => <PlaybookTextarea id={id} value={summary} disabled={write.isPending} required maxLength={2000} onChange={(event) => setSummary(event.target.value)} placeholder="Reference the customer’s confirmation or explain your decision." />}</PlaybookField>
      {write.isError && <PlaybookError error={write.error} retry={onReload} />}
      <DialogFooter><QuietTextAction type="button" disabled={write.isPending} onClick={onClose}>Cancel</QuietTextAction><QuietPrimaryAction type="submit" disabled={write.isPending || !summary.trim()}>{write.isPending ? 'Saving…' : 'Record progress'}</QuietPrimaryAction></DialogFooter>
    </form>
  </DialogContent></Dialog>;
}

function SignalLifecycle({ ws, signalId, selection, onClose, onReload }: { ws: string; signalId: string; selection: { operation: 'pause' | 'resume' | 'close'; revision: number }; onClose: () => void; onReload: () => void }) {
  const [summary, setSummary] = useState('');
  const [kind, setKind] = useState<'achieved' | 'not_pursued' | 'invalid'>('achieved');
  const [intentKey] = useState(createPlaybookIntentKey);
  const write = useCRMPlaybookWrite(ws);
  const operation = selection.operation;
  const save = async () => {
    const intent = operation === 'close' ? { expected_revision: selection.revision, operation, outcome: { kind, summary: summary.trim() } } : operation === 'pause' ? { expected_revision: selection.revision, operation, reason: summary.trim() } : { expected_revision: selection.revision, operation };
    try { await write.mutateAsync({ kind: 'signal', signalId, body: { ...intent, command_key: intentKey(intent) } }); toast.success(operation === 'close' ? 'Outcome recorded' : operation === 'pause' ? 'Signal paused' : 'Signal resumed'); onClose(); } catch { /* Keep the explicit lifecycle decision. */ }
  };
  return <Dialog open onOpenChange={(open) => { if (!open && !write.isPending) onClose(); }}><DialogContent><DialogHeader><DialogTitle>{operation === 'close' ? 'Record outcome' : operation === 'pause' ? 'Pause this signal?' : 'Resume this signal?'}</DialogTitle><DialogDescription>{operation === 'close' ? 'Record what happened for the customer. This closes the signal; a completed run alone is not an outcome.' : operation === 'pause' ? 'Pause this signal and its pending checks. Work already executing may still finish.' : 'Return this signal to open work with the same playbook version.'}</DialogDescription></DialogHeader>
    <form onSubmit={(event) => { event.preventDefault(); void save(); }} className="space-y-5">
      {operation === 'close' && <PlaybookField label="Outcome">{(id) => <QuietSelect id={id} label="Outcome" value={kind} disabled={write.isPending} options={[{ value: 'achieved', label: 'Customer outcome achieved' }, { value: 'not_pursued', label: 'Not pursued' }, { value: 'invalid', label: 'Not a valid signal' }]} onChange={(value) => setKind(value as typeof kind)} />}</PlaybookField>}
      {operation !== 'resume' && <PlaybookField label={operation === 'close' ? 'What happened?' : 'Reason for pausing'}>{(id) => <PlaybookTextarea id={id} value={summary} disabled={write.isPending} required maxLength={2000} onChange={(event) => setSummary(event.target.value)} />}</PlaybookField>}
      {write.isError && <PlaybookError error={write.error} retry={onReload} />}
      <DialogFooter><QuietTextAction type="button" disabled={write.isPending} onClick={onClose}>Cancel</QuietTextAction><QuietPrimaryAction type="submit" disabled={write.isPending || (operation !== 'resume' && !summary.trim())}>{write.isPending ? 'Saving…' : operation === 'close' ? 'Record and close' : operation === 'pause' ? 'Pause signal' : 'Resume signal'}</QuietPrimaryAction></DialogFooter>
    </form>
  </DialogContent></Dialog>;
}
