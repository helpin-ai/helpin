import { useState } from 'react';
import { format } from 'date-fns';
import { toast } from 'sonner';
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog';
import { QuietPrimaryAction, QuietTextAction } from '@/components/design-system/quiet';
import { DatePicker } from '@/components/ui/date-picker';
import { Input } from '@/components/ui/input';
import { MemberPickerPopover } from '@/components/pm/MemberPickerPopover';
import { useAssignableMembers } from '@/hooks/queries/useWorkspaces';
import { useCRMPlaybookWrite } from '@/hooks/queries/useCRMPlaybooks';
import type { CRMSituationChanges, CRMSituationItem } from '@/lib/crmSituationTypes';
import { createPlaybookIntentKey } from '@/lib/crmPlaybookPresentation';
import { PlaybookError, PlaybookField, PlaybookSelect, PlaybookTextarea } from './PlaybookUI';
import { PlaybookRequestError } from '@/lib/services/crmPlaybookService';

export function PlaybookSignalEdit({ ws, item, onClose, onReload }: { ws: string; item: CRMSituationItem; onClose: () => void; onReload: () => void }) {
  const [owner, setOwner] = useState(item.situation.owner_member_id);
  const [nextOwner, setNextOwner] = useState(item.situation.next_action_owner_member_id);
  const [nextStep, setNextStep] = useState(item.situation.next_step);
  const originalDate = item.situation.next_checkpoint_at ? format(new Date(item.situation.next_checkpoint_at), 'yyyy-MM-dd') : '';
  const originalTime = item.situation.next_checkpoint_at ? format(new Date(item.situation.next_checkpoint_at), 'HH:mm') : '09:00';
  const [date, setDate] = useState(originalDate);
  const [time, setTime] = useState(originalTime);
  const [attention, setAttention] = useState(item.situation.attention);
  const [intentKey] = useState(createPlaybookIntentKey);
  const members = useAssignableMembers(ws);
  const write = useCRMPlaybookWrite(ws);
  const changes: CRMSituationChanges = {};
  if (owner !== item.situation.owner_member_id) changes.owner = { member_id: owner };
  if (nextOwner !== item.situation.next_action_owner_member_id) changes.next_action_owner = { member_id: nextOwner };
  if (nextStep.trim() !== item.situation.next_step) changes.next_step = nextStep.trim();
  if (attention !== item.situation.attention) changes.attention = attention as CRMSituationChanges['attention'];
  const checkpoint = date && time ? new Date(`${date}T${time}`) : null;
  const validCheckpoint = !date || !!checkpoint && Number.isFinite(checkpoint.getTime());
  if (validCheckpoint && (date !== originalDate || time !== originalTime)) changes.checkpoint = { at: checkpoint?.toISOString() || null };
  const waiting = item.situation.lifecycle === 'open' && ['waiting_customer', 'waiting_work'].includes(attention);
  const responsible = nextOwner || owner;
  const valid = validCheckpoint && (!waiting || (!!nextStep.trim() && !!responsible && !!date && !!members.data?.some((member) => member.id === responsible && member.status === 'active')));
  const conflict = write.error instanceof PlaybookRequestError && write.error.status === 409;
  const name = (id: string | null) => !id ? 'Unassigned' : members.data?.find((member) => member.id === id)?.display_name || (id === item.situation.owner_member_id ? item.owner_name : id === item.situation.next_action_owner_member_id ? item.next_action_owner_name : '') || 'Member unavailable';
  const save = async () => {
    if (write.isPending || conflict || !valid) return;
    const intent = { expected_revision: item.situation.revision, operation: 'update' as const, changes };
    try { await write.mutateAsync({ kind: 'signal', signalId: item.situation.id, body: { ...intent, command_key: intentKey(intent) } }); toast.success('Signal updated'); onClose(); } catch { /* Keep the draft and observed revision. */ }
  };
  return <Dialog open onOpenChange={(open) => { if (!open && !write.isPending) onClose(); }}><DialogContent><DialogHeader><DialogTitle>Update next step</DialogTitle><DialogDescription>Set the follow-up and who is responsible. No messages or tasks will be created.</DialogDescription></DialogHeader>
    <form onSubmit={(event) => { event.preventDefault(); if (valid && Object.keys(changes).length) void save(); }} className="space-y-5">
      <PlaybookField label="Next step">{(id) => <PlaybookTextarea id={id} value={nextStep} disabled={write.isPending} maxLength={1000} onChange={(event) => setNextStep(event.target.value)} required={waiting} />}</PlaybookField>
      <div className="grid gap-5 sm:grid-cols-2"><PlaybookField label="Signal owner">{() => <MemberPickerPopover members={members.data?.filter((member) => member.status === 'active') ?? []} value={owner || ''} disabled={write.isPending || members.isPending || members.isError} onChange={(id) => setOwner(id === '__none__' ? null : id)} triggerLabel="Choose signal owner" noneLabel="Unassigned" renderTrigger={() => <span>{name(owner)}</span>} />}</PlaybookField>
      <PlaybookField label="Next action owner" help="Responsible for the next step. When unassigned, the signal owner is responsible.">{() => <MemberPickerPopover members={members.data?.filter((member) => member.status === 'active') ?? []} value={nextOwner || ''} disabled={write.isPending || members.isPending || members.isError} onChange={(id) => setNextOwner(id === '__none__' ? null : id)} triggerLabel="Choose next action owner" noneLabel="Signal owner" renderTrigger={() => <span>{nextOwner ? name(nextOwner) : 'Signal owner'}</span>} />}</PlaybookField></div>
      <PlaybookField label="Follow-up status" help="Approvals and action failures remain visible until they are resolved.">{(id) => <PlaybookSelect id={id} label="Follow-up status" value={attention} disabled={write.isPending} onChange={(value) => setAttention(value as typeof attention)} options={[{ value: 'needs_context', label: 'Needs context' }, { value: 'follow_up_due', label: 'Follow-up due' }, { value: 'waiting_customer', label: 'Waiting on customer' }, { value: 'waiting_work', label: 'Work in progress' }, ...(['needs_approval', 'automation_failed'].includes(item.situation.attention) ? [{ value: item.situation.attention, label: 'Keep current status' }] : [])]} />}</PlaybookField>
      <fieldset disabled={write.isPending} className="flex flex-wrap items-end gap-5"><PlaybookField label="Next check" help="Schedules an attention check, not an outbound message. Times use your current time zone.">{() => <DatePicker label="Next check" value={date} onChange={(value) => { if (!write.isPending) setDate(value); }} placeholder="Choose a date" />}</PlaybookField>{date && <PlaybookField label="Time">{(id) => <Input id={id} aria-label="Next check time" type="time" className="w-32" value={time} onChange={(event) => setTime(event.target.value)} required />}</PlaybookField>}</fieldset>
      {!valid && <p role="alert" className="text-sm text-quiet-accent">Waiting signals need a next step, an active responsible person, and a valid next check.</p>}
      {members.isError && <PlaybookError error={members.error} retry={() => void members.refetch()} />}
      {write.isError && <PlaybookError error={write.error} retry={conflict ? undefined : onReload} />}
      {conflict && <p className="text-xs leading-5 text-quiet-text-secondary">Your draft is still here. Copy anything you want to keep, then close and reopen the editor to review the latest signal.</p>}
      <DialogFooter><QuietTextAction type="button" disabled={write.isPending} onClick={() => { if (conflict) onReload(); onClose(); }}>Cancel</QuietTextAction><QuietPrimaryAction type="submit" disabled={write.isPending || conflict || !valid || !Object.keys(changes).length}>{write.isPending ? 'Saving…' : 'Save changes'}</QuietPrimaryAction></DialogFooter>
    </form>
  </DialogContent></Dialog>;
}
