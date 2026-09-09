import { useState } from 'react';
import { Button } from '@/components/ui/button';
import { Link } from '@tanstack/react-router';
import { toast } from 'sonner';
import { QuietSelect, QuietPrimaryAction, QuietPropertyRow, QuietSection, QuietStatusText, QuietTextAction } from '@/components/design-system/quiet';
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog';
import { useCompany, useContact, useDeal, usePipeline } from '@/hooks/queries/useCRM';
import { useCRMSituationDecision } from '@/hooks/queries/useCRMSituations';
import { actionRequiresFollowThrough, actionStatus, approvalLabel, contextText, dismissalReasons } from '@/lib/crmSituationPresentation';
import type { CRMSignalDismissalReason, CRMSuggestion } from '@/lib/crmTypes';
import type { CRMSituationItem } from '@/lib/crmSituationTypes';
import { PlaybookError, PlaybookField } from '../playbooks/PlaybookUI';
import { PlaybookActionReview, PlaybookActionResult } from './PlaybookActionReview';
import { signalOverviewText, repeatsSignalText } from './SignalDrawerLayout';

export function SignalActions({ ws, slug, item, canEdit }: { ws: string; slug: string; item: CRMSituationItem; canEdit: boolean }) {
  return <RecommendationActions ws={ws} slug={slug} actions={item.actions ?? []} displayedText={signalOverviewText(item)} signalId={item.situation.id} lifecycle={item.situation.lifecycle} canEdit={canEdit} />;
}

export function RecommendationActions({ ws, slug, actions, signalId, lifecycle = 'open', canEdit, blockedReason, displayedText = [] }: { ws: string; slug: string; actions: CRMSuggestion[]; signalId?: string; lifecycle?: string; canEdit: boolean; blockedReason?: string; displayedText?: (string | null | undefined)[] }) {
  const [selection, setSelection] = useState<{ action: CRMSuggestion; kind: 'accept' | 'dismiss' }>();
  const pending = actions.filter((action) => action.status === 'pending');
  const active = actions.filter((action) => action.status === 'accepted' && (action.execution_status !== 'succeeded' || actionRequiresFollowThrough(action)));
  const completed = actions.filter((action) => ['dismissed', 'superseded', 'expired'].includes(action.status) || action.status === 'accepted' && action.execution_status === 'succeeded' && !actionRequiresFollowThrough(action));
  const titleShownAbove = pending.length + active.length === 1 && repeatsSignalText((pending[0] || active[0]).title, displayedText);
  if (!actions.length) return null;
  return <>
    {(pending.length > 0 || active.length > 0) && <QuietSection className="px-0 py-4 sm:px-0 lg:px-0" title={pending.length ? titleShownAbove ? 'Review next step' : 'Recommended action' : 'Action progress'}>
      <div className="divide-y divide-quiet-divider-light">{[...pending, ...active].map((action) => <div key={action.id} className="py-3 first:pt-0 last:pb-0">
        {!titleShownAbove && <p className="text-sm font-medium leading-6">{action.title}</p>}
        {action.description && !repeatsSignalText(action.description, [...displayedText, action.title]) && <p className="mt-1 whitespace-pre-wrap text-sm leading-6 text-quiet-text-secondary">{action.description}</p>}
        <RecommendationTarget slug={slug} action={action} />
        {action.suggestion_type !== 'playbook_action' && <RecommendationDetails action={action} />}
        {action.suggestion_type === 'playbook_action' && action.status === 'accepted' && <PlaybookActionResult ws={ws} slug={slug} action={action} canEdit={canEdit} />}
        {action.status !== 'pending' && <div className="mt-2"><QuietStatusText tone={action.execution_status === 'failed' || !action.execution_status ? 'blocker' : 'neutral'}>{actionStatus(action)}</QuietStatusText>
          {actionRequiresFollowThrough(action) && <p className="mt-1 text-xs leading-5 text-quiet-text-secondary">Your decision is recorded. No message was sent or customer record changed by this approval.</p>}
          {action.execution_status === 'failed' && <p className="mt-1 text-xs leading-5 text-quiet-text-secondary">Check the target record and configuration before taking another action.</p>}
        </div>}
        {action.status === 'pending' && <div className="mt-3 flex items-center gap-3">
          {canEdit && lifecycle === 'open' && !blockedReason ? <><Button type="button" variant="outline" size="sm" disabled={!action.revision} onClick={() => setSelection({ action, kind: 'accept' })}>Review action</Button><QuietTextAction disabled={!action.revision} onClick={() => setSelection({ action, kind: 'dismiss' })}>Dismiss</QuietTextAction></> : <p className="text-xs text-quiet-text-tertiary">{blockedReason || (lifecycle === 'paused' ? 'Resume this signal to review actions.' : lifecycle === 'closed' ? 'This signal is closed. No further actions can be approved.' : 'A teammate with CRM edit access can review this action.')}</p>}
          {!action.revision && <span className="text-xs text-quiet-accent">Refresh to load the latest recommendation.</span>}
        </div>}
      </div>)}</div>
    </QuietSection>}
    {completed.length > 0 && <details className="border-b border-quiet-divider-strong py-4 text-sm"><summary className="cursor-pointer text-quiet-text-secondary">Past decisions ({completed.length})</summary><ul className="mt-3 space-y-4">{completed.map((action) => <li key={action.id}><p>{action.title}</p><p className="mt-1 text-xs text-quiet-text-tertiary">{actionStatus(action)}{action.dismissal_reason && ` · ${dismissalReasons.find((reason) => reason.value === action.dismissal_reason)?.label || action.dismissal_reason.replaceAll('_', ' ')}`}</p>{action.suggestion_type === 'playbook_action' && action.execution_status === 'succeeded' && <PlaybookActionResult ws={ws} slug={slug} action={action} canEdit={canEdit} />}</li>)}</ul></details>}
    {selection && (selection.kind === 'accept' && selection.action.suggestion_type === 'playbook_action' ? <PlaybookActionReview key={`${selection.action.id}:${selection.action.revision}`} ws={ws} slug={slug} action={selection.action} signalId={signalId} stale={!canEdit || lifecycle !== 'open' || !!blockedReason || !actions.some((action) => action.id === selection.action.id && action.revision === selection.action.revision && action.status === 'pending')} onClose={() => setSelection(undefined)} /> : <SignalDecision key={`${selection.action.id}:${selection.action.revision}:${selection.kind}`} ws={ws} slug={slug} actions={actions} signalId={signalId} allowed={canEdit && lifecycle === 'open' && !blockedReason} selection={selection} onClose={() => setSelection(undefined)} />)}
  </>;
}

function RecommendationTarget({ slug, action }: { slug: string; action: CRMSuggestion }) {
  const ctx = action.context || {};
  const explicit = action.object_id && ['deal', 'contact', 'company', 'meeting'].includes(action.object_type || '');
  const type = explicit ? action.object_type : ['deal', 'contact', 'company'].find((kind) => contextText(ctx[`${kind}_id`]));
  const id = explicit ? action.object_id! : contextText(ctx[`${type}_id`]);
  const className = 'mt-2 inline-block text-xs text-quiet-accent hover:underline';
  if (!id) return null;
  if (type === 'deal') return <Link className={className} to="/w/$slug/crm/deals/$dealId" params={{ slug, dealId: id }}>Open deal</Link>;
  if (type === 'contact') return <Link className={className} to="/w/$slug/crm/contacts/$contactId" params={{ slug, contactId: id }}>Open contact</Link>;
  if (type === 'company') return <Link className={className} to="/w/$slug/crm/companies/$companyId" params={{ slug, companyId: id }}>Open company</Link>;
  if (type === 'meeting') return <Link className={className} to="/w/$slug/crm/meetings/$meetingId" params={{ slug, meetingId: id }}>Open meeting</Link>;
  return null;
}

function RecommendationDetails({ action }: { action: CRMSuggestion }) {
  const ctx = action.context || {};
  const rows = [
    ['Contact', contextText(ctx.contact_name)], ['Company', contextText(ctx.company_name)],
    ['Deal', contextText(ctx.deal_name)], ['Estimated amount', typeof ctx.amount === 'number' ? ctx.amount.toLocaleString(undefined, { style: 'currency', currency: 'USD' }) : contextText(ctx.amount)],
    ['Stage at detection', contextText(ctx.current_stage)], ['Proposed stage', contextText(ctx.recommended_stage)], ['Source', contextText(ctx.source)],
  ].filter(([, value]) => !!value);
  return <details className="mt-3 text-xs"><summary className="cursor-pointer text-quiet-text-tertiary">Recommendation details</summary>
    <p className="mt-2 text-quiet-text-tertiary">Context recorded when this recommendation was created. Check the customer record for current information.</p>
    {rows.map(([label, value]) => <QuietPropertyRow key={label} label={label} value={value} />)}
    {Number.isFinite(action.confidence) && <QuietPropertyRow label="Recommendation confidence" value={`${Math.round(action.confidence * 100)}%`} />}
    <QuietPropertyRow label="Created" value={new Date(action.created_at).toLocaleString()} />
  </details>;
}

function SignalDecision({ ws, slug, actions, signalId, allowed, selection, onClose }: { ws: string; slug: string; actions: CRMSuggestion[]; signalId?: string; allowed: boolean; selection: { action: CRMSuggestion; kind: 'accept' | 'dismiss' }; onClose: () => void }) {
  const action = selection.action;
  const ctx = action.context || {};
  const accepting = selection.kind === 'accept';
  const creating = accepting && action.suggestion_type === 'deal_create';
  const advancing = accepting && action.suggestion_type === 'deal_advance';
  const dealId = contextText(ctx.deal_id);
  const companyId = contextText(ctx.company_id);
  const contactId = contextText(ctx.contact_id);
  const deal = useDeal(ws, advancing ? dealId : '');
  const pipeline = usePipeline(ws, creating ? contextText(ctx.pipeline_id) : advancing ? deal.data?.pipeline_id || '' : '');
  const company = useCompany(ws, creating ? companyId : '');
  const contact = useContact(ws, creating ? contactId : '');
  const stageId = contextText(creating ? ctx.stage_id : ctx.target_stage_id);
  const stage = pipeline.data?.stages?.find((candidate) => candidate.id === stageId);
  const [reason, setReason] = useState<CRMSignalDismissalReason | ''>('');
  const write = useCRMSituationDecision(ws, signalId);
  const current = actions.find((candidate) => candidate.id === action.id);
  const stale = !current || current.revision !== action.revision || current.status !== 'pending' || !allowed;
  const available = (!creating || (!!contextText(ctx.deal_name) && !!stage && (!companyId || company.isSuccess && !!company.data) && (!contactId || contact.isSuccess && !!contact.data) && !!(companyId || contactId))) && (!advancing || (!!deal.data && !!stage));
  const decide = async () => {
    if (!action.revision || stale || !available || write.isPending || write.isError || !accepting && !reason) return;
    try {
      const result = await write.mutateAsync(accepting ? { kind: 'accept', actionId: action.id, revision: action.revision } : { kind: 'dismiss', actionId: action.id, revision: action.revision, reason: reason as CRMSignalDismissalReason });
      if (result.execution_status === 'failed') toast.error('Action failed. Check its status before taking another action.');
      else toast.success(result.status === 'dismissed' ? 'Recommendation dismissed' : result.execution_status === 'succeeded' ? 'Action completed' : result.execution_status === 'manual_required' ? 'Approved. Follow-through is still needed.' : 'Decision recorded. Check action progress.');
      onClose();
    } catch { /* Do not retry a possibly executed action. Query refresh runs on settlement. */ }
  };
  return <Dialog open onOpenChange={(open) => { if (!open && !write.isPending) onClose(); }}><DialogContent><DialogHeader>
    <DialogTitle>{accepting ? `${approvalLabel(action)}?` : 'Dismiss this recommendation?'}</DialogTitle>
    <DialogDescription>{creating ? 'This immediately creates the deal below.' : advancing ? 'This immediately changes the deal’s stage.' : accepting ? 'Records your approval only. It does not send a message, update a customer record, or create a task.' : 'Dismisses this recommendation, not the signal or its evidence.'}</DialogDescription>
  </DialogHeader>
    <p className="text-sm font-medium leading-6">{action.title}</p>
    {creating && <div><QuietPropertyRow label="Deal" value={contextText(ctx.deal_name) || 'Not specified'} />{companyId && <QuietPropertyRow label="Company" value={company.data?.name || 'Loading company…'} />}{contactId && <QuietPropertyRow label="Contact" value={contact.data ? [contact.data.first_name, contact.data.last_name].filter(Boolean).join(' ') || contact.data.email : 'Loading contact…'} />}<QuietPropertyRow label="Amount" value={typeof ctx.amount === 'number' ? ctx.amount.toLocaleString() : 'Not set'} /></div>}
    {advancing && deal.data && <QuietPropertyRow label="Deal" value={<Link to="/w/$slug/crm/deals/$dealId" params={{ slug, dealId }} className="text-quiet-accent hover:underline">{deal.data.name}</Link>} />}
    {(creating || advancing) && <div><QuietPropertyRow label="Pipeline" value={pipeline.data?.name || 'Loading pipeline…'} /><QuietPropertyRow label={advancing ? 'Move to' : 'Stage'} value={stage?.name || 'Stage unavailable'} /></div>}
    {!available && <p role="alert" className="text-sm text-quiet-accent">The proposed deal, customer, pipeline, and stage must be available before you can approve. Check the recommendation’s configuration if they cannot be loaded.</p>}
    {!accepting && <PlaybookField label="Reason">{(id) => <QuietSelect id={id} label="Dismissal reason" value={reason || 'choose'} onChange={(value) => setReason(value as CRMSignalDismissalReason)} disabled={write.isPending} options={[{ value: 'choose', label: 'Choose a reason' }, ...dismissalReasons]} />}</PlaybookField>}
    {stale && !write.isPending && <p role="alert" className="text-sm text-quiet-accent">This recommendation or signal has changed. Close this dialog and review its current state.</p>}
    {write.isError && <><PlaybookError error={write.error} /><p className="text-xs leading-5 text-quiet-text-secondary">Close this dialog and check the refreshed action status before deciding again. Your request may already have reached the server.</p></>}
    <DialogFooter><QuietTextAction disabled={write.isPending} onClick={onClose}>{write.isError || stale ? 'Close' : 'Cancel'}</QuietTextAction><QuietPrimaryAction disabled={write.isPending || write.isError || stale || !available || !accepting && (!reason || !dismissalReasons.some((entry) => entry.value === reason))} onClick={() => void decide()}>{write.isPending ? 'Recording…' : accepting ? approvalLabel(action) : 'Dismiss recommendation'}</QuietPrimaryAction></DialogFooter>
  </DialogContent></Dialog>;
}
