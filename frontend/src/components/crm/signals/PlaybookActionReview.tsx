import { useEffect, useMemo, useState } from 'react';
import { EditorContent, useEditor } from '@tiptap/react';
import StarterKit from '@tiptap/starter-kit';
import { Link } from '@tanstack/react-router';
import { toast } from 'sonner';
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog';
import { QuietComposerEditorSurface, QuietConversationComposer, QuietPrimaryAction, QuietPropertyRow, QuietTextAction, QuietUnderlineInput } from '@/components/design-system/quiet';
import { useCRMSituationDecision } from '@/hooks/queries/useCRMSituations';
import { useCRMPlaybookActionIntent, useCRMPlaybookAutomationWrite } from '@/hooks/queries/useCRMPlaybookAutomation';
import { useWorkspaceAccess, usePermissions } from '@/hooks/queries/useSession';
import { useAssignableMembers } from '@/hooks/queries/useWorkspaces';
import { useDeal, useEmailAccounts, usePipeline } from '@/hooks/queries/useCRM';
import { useWorkspaceTeams } from '@/hooks/useWorkspaceTeams';
import type { CRMPlaybookAction } from '@/lib/crmPlaybookTypes';
import type { CRMSuggestion } from '@/lib/crmTypes';
import { PlaybookError, PlaybookField, PlaybookHelp, PlaybookSelect, PlaybookTextarea } from '../playbooks/PlaybookUI';

const actionLabels: Record<CRMPlaybookAction['kind'], string> = { email: 'Send email', task: 'Create task', handoff: 'Accept handoff', milestone: 'Confirm milestone', deal_stage: 'Change stage' };
export function readTypedPlaybookAction(action: CRMSuggestion): CRMPlaybookAction | undefined {
  const value = action.context?.playbook_action;
  if (!value || typeof value !== 'object') return;
  const candidate = value as CRMPlaybookAction;
  if (candidate.version !== 1 || !Object.hasOwn(actionLabels, candidate.kind) || !candidate[candidate.kind]) return;
  return candidate;
}

export function PlaybookActionReview({ ws, slug, action, signalId, stale, onClose }: { ws: string; slug: string; action: CRMSuggestion; signalId?: string; stale: boolean; onClose: () => void }) {
  const original = readTypedPlaybookAction(action);
  const [draft, setDraft] = useState(original);
  const intent = useCRMPlaybookActionIntent(ws, action.id);
  const access = useWorkspaceAccess(ws);
  const members = useAssignableMembers(ws);
  const write = useCRMSituationDecision(ws, signalId);
  const memberID = access.data?.membership?.id;
  const approver = intent.data?.approver_member_id;
  const expired = !!intent.data && Date.parse(intent.data.expires_at) <= Date.now();
  const allowed = !!memberID && memberID === approver && !stale && !expired;
  const update = (next: CRMPlaybookAction) => setDraft(next);
  const valid = !!draft && (draft.kind !== 'email' || !!draft.email?.account_id && !!draft.email.subject.trim() && !!draft.email.body_html.trim()) && (draft.kind !== 'task' || !!draft.task?.team_id && !!draft.task.owner_member_id && !!draft.task.name.trim());
  const submit = async () => {
    if (!draft || !valid || !allowed || !action.revision || write.isPending || write.isError) return;
    try {
      const result = await write.mutateAsync({ kind: 'accept', actionId: action.id, revision: action.revision, edits: { playbook_action: draft } });
      if (result.execution_status === 'succeeded') toast.success(draft.kind === 'email' ? 'Email sent' : draft.kind === 'task' ? 'Task created' : 'Action completed');
      else if (result.execution_status === 'failed') toast.error('The action could not be completed. Review its status.');
      else toast.info('Decision recorded. Check the action’s result before doing anything else.');
      onClose();
    } catch { /* Refresh runs on settlement. Never replay a possibly completed effect. */ }
  };
  return <Dialog open onOpenChange={(open) => { if (!open && !write.isPending) onClose(); }}><DialogContent className="max-h-[90vh] overflow-y-auto sm:max-w-2xl"><DialogHeader><DialogTitle>{draft ? actionLabels[draft.kind] : 'Review action'}</DialogTitle><DialogDescription>{draft?.kind === 'handoff' ? 'Accepting makes you responsible for this customer’s next step.' : 'Review the exact change below. Confirming performs this action once.'}</DialogDescription></DialogHeader>
    {!draft ? <PlaybookError error={new Error('This action could not be read. Refresh the signal.')} /> : <fieldset disabled={!allowed || write.isPending || write.isError} className="min-w-0 space-y-4">
      {draft.kind === 'email' && draft.email && <EmailReview ws={ws} draft={draft} update={update} disabled={!allowed || write.isPending || write.isError} />}
      {draft.kind === 'task' && draft.task && <TaskReview ws={ws} draft={draft} update={update} disabled={!allowed || write.isPending || write.isError} />}
      {draft.handoff && <><QuietPropertyRow label="Receiving owner" value={members.data?.find((member) => member.id === draft.handoff?.receiving_member_id)?.display_name || 'Loading owner…'} /><PlaybookField label="Customer handoff">{(id) => <PlaybookTextarea id={id} value={draft.handoff!.summary} onChange={(event) => update({ ...draft, handoff: { ...draft.handoff!, summary: event.target.value } })} maxLength={2000} />}</PlaybookField></>}
      {draft.milestone && <><QuietPropertyRow label="Milestone" value={draft.milestone.key.replaceAll('_', ' ')} /><PlaybookField label="Evidence supporting this milestone">{(id) => <PlaybookTextarea id={id} value={draft.milestone!.evidence} onChange={(event) => update({ ...draft, milestone: { ...draft.milestone!, evidence: event.target.value } })} maxLength={2000} />}</PlaybookField></>}
      {draft.deal_stage && <DealStageReview ws={ws} slug={slug} action={draft} />}
    </fieldset>}
    {intent.isPending ? <p className="text-xs text-quiet-text-tertiary">Checking approval…</p> : intent.isError ? <PlaybookError error={intent.error} retry={() => void intent.refetch()} /> : <div className="flex items-center gap-1 text-xs text-quiet-text-tertiary">{approver === memberID ? 'Your approval is required' : `Approval assigned to ${members.data?.find((member) => member.id === approver)?.display_name || 'another teammate'}`}<PlaybookHelp label="About this approval">Approval applies only to this action and the current customer context. Changed context or an expired approval requires a fresh review.</PlaybookHelp></div>}
    {(stale || expired) && <p role="alert" className="text-sm text-quiet-accent">{expired ? 'This approval has expired.' : 'This signal or action changed.'} Close and review the latest state.</p>}
    {write.isError && <><PlaybookError error={write.error} /><p className="text-xs text-quiet-text-secondary">Close and check the refreshed action status. Your request may already have reached the server.</p></>}
    <DialogFooter><QuietTextAction disabled={write.isPending} onClick={onClose}>{write.isError || stale || expired ? 'Close' : 'Cancel'}</QuietTextAction><QuietPrimaryAction disabled={!allowed || !valid || write.isPending || write.isError} onClick={() => void submit()}>{write.isPending ? 'Confirming…' : draft ? actionLabels[draft.kind] : 'Confirm'}</QuietPrimaryAction></DialogFooter>
  </DialogContent></Dialog>;
}

function EmailReview({ ws, draft, update, disabled }: { ws: string; draft: CRMPlaybookAction; update: (action: CRMPlaybookAction) => void; disabled: boolean }) {
  const email = draft.email!;
  const accounts = useEmailAccounts(ws);
  const senders = (accounts.data || []).filter((account) => account.can_send === true && account.is_active && account.status === 'connected' && account.provider === 'gmail');
  const extensions = useMemo(() => [StarterKit.configure({ heading: false, codeBlock: false, horizontalRule: false, link: { openOnClick: false, HTMLAttributes: { rel: 'noopener noreferrer nofollow', target: '_blank' } } })], []);
  const editor = useEditor({ extensions, content: email.body_html, immediatelyRender: false, editable: !disabled, editorProps: { attributes: { role: 'textbox', 'aria-label': 'Email message', 'aria-multiline': 'true', class: 'rich-text-soft prose prose-sm dark:prose-invert max-w-none min-h-[180px] p-3 text-sm focus:outline-none' } }, onUpdate: ({ editor: value }) => update({ ...draft, email: { ...email, body_html: value.getHTML() } }) });
  useEffect(() => { editor?.setEditable(!disabled); }, [editor, disabled]);
  return <>
    <QuietPropertyRow label="To" value={email.to} />
    <PlaybookField label="From">{(id) => <PlaybookSelect id={id} label="Sending account" value={email.account_id || 'choose'} onChange={(value) => update({ ...draft, email: { ...email, account_id: value === 'choose' ? '' : value } })} disabled={disabled} options={[{ value: 'choose', label: 'Choose your sending account' }, ...senders.map((account) => ({ value: account.id, label: account.email_address }))]} />}</PlaybookField>
    {accounts.isError && <PlaybookError error={accounts.error} retry={() => void accounts.refetch()} />}
    {accounts.isSuccess && !senders.length && <p className="text-xs text-quiet-accent">Connect your Gmail account in CRM email settings to send this message.</p>}
    <PlaybookField label="Subject">{(id) => <QuietUnderlineInput id={id} value={email.subject} maxLength={300} onChange={(event) => update({ ...draft, email: { ...email, subject: event.target.value } })} />}</PlaybookField>
    <QuietConversationComposer><QuietComposerEditorSurface><EditorContent editor={editor} /></QuietComposerEditorSurface></QuietConversationComposer>
    <p className="text-xs text-quiet-text-tertiary">One recipient · No attachments<PlaybookHelp label="About coordinated outreach">Helpin checks for an unresolved or recently sent playbook email to this contact before sending. A second playbook cannot send another email within 24 hours.</PlaybookHelp></p>
  </>;
}

function TaskReview({ ws, draft, update, disabled }: { ws: string; draft: CRMPlaybookAction; update: (action: CRMPlaybookAction) => void; disabled: boolean }) {
  const task = draft.task!;
  const access = useWorkspaceAccess(ws);
  const { canAccessModule } = usePermissions(access.data);
  const teams = useWorkspaceTeams(canAccessModule('pm') ? ws : undefined);
  const members = useAssignableMembers(ws);
  const ownTeams = teams.teams.filter((team) => access.data?.team_memberships.some((membership) => membership.team_id === team.id));
  return <>
    <PlaybookField label="Task">{(id) => <QuietUnderlineInput id={id} value={task.name} maxLength={240} onChange={(event) => update({ ...draft, task: { ...task, name: event.target.value } })} />}</PlaybookField>
    <PlaybookField label="Team">{(id) => <PlaybookSelect id={id} label="Task team" value={task.team_id || 'choose'} disabled={disabled} onChange={(value) => update({ ...draft, task: { ...task, team_id: value === 'choose' ? '' : value } })} options={[{ value: 'choose', label: 'Choose team' }, ...ownTeams.map((team) => ({ value: team.id, label: team.name }))]} />}</PlaybookField>
    <PlaybookField label="Owner" help="The task owner must have access to the selected team and PM.">{(id) => <PlaybookSelect id={id} label="Task owner" value={task.owner_member_id || 'choose'} disabled={disabled} onChange={(value) => update({ ...draft, task: { ...task, owner_member_id: value === 'choose' ? '' : value } })} options={[{ value: 'choose', label: 'Choose owner' }, ...(members.data || []).filter((member) => member.status === 'active').map((member) => ({ value: member.id, label: member.display_name }))]} />}</PlaybookField>
    <PlaybookField label="Deliverable">{(id) => <PlaybookTextarea id={id} value={task.description} maxLength={5000} onChange={(event) => update({ ...draft, task: { ...task, description: event.target.value } })} />}</PlaybookField>
    <QuietPropertyRow label="Due" value={task.deadline ? new Date(task.deadline).toLocaleString() : 'No due date'} />
    {!canAccessModule('pm') && <p className="text-xs text-quiet-accent">PM access is required to create this task.</p>}
  </>;
}

function DealStageReview({ ws, slug, action }: { ws: string; slug: string; action: CRMPlaybookAction }) {
  const change = action.deal_stage!;
  const deal = useDeal(ws, change.deal_id);
  const pipeline = usePipeline(ws, deal.data?.pipeline_id || '');
  return <><QuietPropertyRow label="Deal" value={<Link to="/w/$slug/crm/deals/$dealId" params={{ slug, dealId: change.deal_id }} className="text-quiet-accent hover:underline">{deal.data?.name || 'View deal'}</Link>} /><QuietPropertyRow label="Move to" value={pipeline.data?.stages?.find((stage) => stage.id === change.stage_id)?.name || 'Stage unavailable — review the deal'} /></>;
}

export function PlaybookActionResult({ ws, slug, action, canEdit }: { ws: string; slug: string; action: CRMSuggestion; canEdit: boolean }) {
  const intent = useCRMPlaybookActionIntent(ws, action.id);
  const access = useWorkspaceAccess(ws);
  const write = useCRMPlaybookAutomationWrite(ws);
  const [inspecting, setInspecting] = useState<CRMSuggestion>();
  const unresolved = action.execution_status === 'in_progress';
  const canInspect = unresolved && canEdit && !!intent.data?.approved_by_member_id && intent.data.approved_by_member_id === access.data?.membership.id && !!intent.data.approved_at && Date.now() - Date.parse(intent.data.approved_at) >= 5 * 60_000;
  const inspection = action.context?.playbook_action_inspection as { evidence?: string; outcome?: string } | undefined;
  return <div className="mt-2 text-xs text-quiet-text-secondary">
    {action.execution_error && <p className="leading-5">{action.execution_error}</p>}
    {intent.data?.result_type === 'task' && intent.data.result_id && <Link to="/w/$slug/pm/tasks/$taskId" params={{ slug, taskId: intent.data.result_id }} search={{ team: undefined, run: undefined }} className="text-quiet-accent hover:underline">Open task</Link>}
    {intent.data?.result_type === 'crm_deal' && intent.data.result_id && <Link to="/w/$slug/crm/deals/$dealId" params={{ slug, dealId: intent.data.result_id }} className="text-quiet-accent hover:underline">Open deal</Link>}
    {typeof inspection?.evidence === 'string' && <details className="mt-2"><summary className="cursor-pointer">Recorded inspection</summary><p className="mt-2 whitespace-pre-wrap leading-5">{inspection.evidence}</p></details>}
    {unresolved && canEdit && !!intent.data?.approved_by_member_id && intent.data.approved_by_member_id === access.data?.membership.id && <QuietTextAction disabled={write.isPending} onClick={() => { void write.mutateAsync({ kind: 'reconcile', id: action.id }).catch(() => {}); }}>{write.isPending ? 'Checking…' : 'Check result'}</QuietTextAction>}
    {write.isSuccess && unresolved && 'execution_status' in write.data && write.data.execution_status === 'in_progress' && <p role="status">The result is still unconfirmed. Nothing was sent or created again.</p>}
    {canInspect && <QuietTextAction disabled={write.isPending} onClick={() => setInspecting(action)}>Record inspected result</QuietTextAction>}
    {(write.isError || intent.isError) && <PlaybookError error={write.error || intent.error} />}
    {inspecting && <ActionInspection key={`${inspecting.id}:${inspecting.revision}`} ws={ws} action={inspecting} stale={!canInspect || inspecting.revision !== action.revision} onClose={() => setInspecting(undefined)} />}
  </div>;
}

function ActionInspection({ ws, action, stale, onClose }: { ws: string; action: CRMSuggestion; stale: boolean; onClose: () => void }) {
  const [outcome, setOutcome] = useState<'completed' | 'not_completed' | ''>('');
  const [evidence, setEvidence] = useState('');
  const write = useCRMPlaybookAutomationWrite(ws);
  const save = async () => {
    if (!outcome || !action.revision || stale || evidence.trim().length < 20) return;
    try { await write.mutateAsync({ kind: 'inspect', id: action.id, body: { revision: action.revision, outcome, evidence: evidence.trim(), confirmed: true } }); onClose(); } catch { /* Keep the explicit inspection, never retry the original operation. */ }
  };
  return <Dialog open onOpenChange={(open) => { if (!open && !write.isPending) onClose(); }}><DialogContent><DialogHeader><DialogTitle>Record the inspected result</DialogTitle><DialogDescription>Check the actual mailbox, task, or customer record first. This records your finding; it does not send, create, change, or retry anything.</DialogDescription></DialogHeader>
    <PlaybookField label="What did you confirm?">{(id) => <PlaybookSelect id={id} label="Inspected outcome" value={outcome || 'choose'} onChange={(value) => setOutcome(value === 'choose' ? '' : value as typeof outcome)} disabled={write.isPending} options={[{ value: 'choose', label: 'Choose the confirmed result' }, { value: 'completed', label: 'The action completed' }, { value: 'not_completed', label: 'The action did not complete' }]} />}</PlaybookField>
    <PlaybookField label="Where did you check, and what did you find?">{(id) => <PlaybookTextarea id={id} value={evidence} maxLength={2000} disabled={write.isPending} onChange={(event) => setEvidence(event.target.value)} placeholder="Reference the sent message, task, or record you inspected." />}</PlaybookField>
    {stale && <p role="alert" className="text-sm text-quiet-accent">The action changed. Close and check its current result.</p>}
    {write.isError && <PlaybookError error={write.error} />}
    <DialogFooter><QuietTextAction disabled={write.isPending} onClick={onClose}>Cancel</QuietTextAction><QuietPrimaryAction disabled={stale || write.isPending || write.isError || !outcome || evidence.trim().length < 20} onClick={() => void save()}>{write.isPending ? 'Recording…' : 'Record my finding'}</QuietPrimaryAction></DialogFooter>
  </DialogContent></Dialog>;
}
