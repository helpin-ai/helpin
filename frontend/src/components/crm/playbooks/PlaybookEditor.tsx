import { useEffect, useMemo, useState } from 'react';
import { useBlocker } from '@tanstack/react-router';
import { toast } from 'sonner';
import { QuietPrimaryAction, QuietSection, QuietTextAction, QuietUnderlineInput } from '@/components/design-system/quiet';
import { Checkbox } from '@/components/ui/checkbox';
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog';
import { QueryBuilderPopover } from '@/components/ui/query-builder/QueryBuilderPopover';
import { MemberPickerPopover } from '@/components/pm/MemberPickerPopover';
import { CRMRecordPicker } from '@/components/automation/CRMRecordPicker';
import { useAssignableMembers } from '@/hooks/queries/useWorkspaces';
import { usePipelines } from '@/hooks/queries/useCRM';
import { useCRMPlaybookWrite } from '@/hooks/queries/useCRMPlaybooks';
import { buildCRMPlaybookQueryFields } from '@/lib/crmPlaybookQueryBuilder';
import { createPlaybookIntentKey, definitionFingerprint, newMilestoneKey, playbookMotions } from '@/lib/crmPlaybookPresentation';
import type { CRMPlaybookDefinition, CRMPlaybookItem } from '@/lib/crmPlaybookTypes';
import type { CRMRecordTargetType } from '@/lib/agentCRMTargets';
import type { QueryFilterRule } from '@/lib/queryBuilder';
import { PlaybookError, PlaybookField, PlaybookSelect, PlaybookTextarea } from './PlaybookUI';

const recordFields: Record<string, CRMRecordTargetType> = { company_id: 'crm_company', contact_id: 'crm_contact', deal_id: 'crm_deal' };
const ownerRoles = [{ value: 'signal_owner', label: 'Signal owner' }, { value: 'account_owner', label: 'Company owner' }, { value: 'deal_owner', label: 'Deal owner' }, { value: 'customer_success_owner', label: 'Customer success owner' }];
const approvalOptions = [{ value: 'approval_required', label: 'Approval required' }, { value: 'not_allowed', label: 'Not allowed' }];
const stops: { value: CRMPlaybookDefinition['policy']['stop_conditions'][number]; label: string }[] = [
  { value: 'objective_achieved', label: 'The customer outcome is achieved' }, { value: 'customer_declined', label: 'The customer declines' },
  { value: 'no_longer_eligible', label: 'The signal no longer matches' }, { value: 'contact_restricted', label: 'Contact is restricted' },
];

export function PlaybookEditor({ ws, item, canAdmin, onDirty, onReload, onPreview }: {
  ws: string; item: CRMPlaybookItem; canAdmin: boolean; onDirty: (dirty: boolean) => void; onReload: () => void; onPreview: () => void;
}) {
  const [baseline, setBaseline] = useState(item.playbook);
  const [definition, setDefinition] = useState(item.playbook.draft);
  const [intentKey] = useState(createPlaybookIntentKey);
  const [discard, setDiscard] = useState(false);
  const write = useCRMPlaybookWrite(ws);
  const members = useAssignableMembers(ws);
  const pipelines = usePipelines(ws);
  const fields = useMemo(() => buildCRMPlaybookQueryFields(members.data ?? [], pipelines.data ?? []), [members.data, pipelines.data]);
  const dirty = definitionFingerprint(definition) !== definitionFingerprint(baseline.draft);
  const stale = baseline.revision !== item.playbook.revision;
  const blocker = useBlocker({ shouldBlockFn: () => dirty, enableBeforeUnload: dirty, withResolver: true });
  useEffect(() => { onDirty(dirty); }, [dirty, onDirty]);
  // Refresh a clean form with remote changes; never replace an unsaved draft.
  if (!dirty && !write.isPending && baseline.revision !== item.playbook.revision) {
    setBaseline(item.playbook);
    setDefinition(item.playbook.draft);
  }
  const disabled = !canAdmin || write.isPending;
  const validSelection = definition.eligibility.commercial_motions.length > 0 && definition.policy.stop_conditions.length > 0;
  const update = (patch: Partial<CRMPlaybookDefinition>) => setDefinition((current) => ({ ...current, ...patch }));
  const save = async () => {
    const intent = { expected_revision: baseline.revision, operation: 'update_draft' as const, definition };
    try {
      const result = await write.mutateAsync({ kind: 'command', id: item.playbook.id, body: { ...intent, command_key: intentKey(intent) } });
      if ('change' in result && 'draft' in result.change.after) {
        setBaseline(result.change.after);
        setDefinition(result.change.after.draft);
      }
      toast.success('Draft saved');
    } catch { /* Preserve unsaved inputs on validation, network, or revision errors. */ }
  };
  const escalationMember = members.data?.find((member) => member.id === definition.responsibilities.escalation_member_id);
  const renderConditionValue = (rule: QueryFilterRule, onChange: (value: string) => void) => {
    if (recordFields[rule.field]) return <CRMRecordPicker workspaceId={ws} targetType={recordFields[rule.field]} value={rule.value ?? ''} onChange={onChange} />;
    if (rule.field === 'owner_member_id') return <MemberPickerPopover members={members.data ?? []} value={rule.value || ''} disabled={members.isPending || members.isError} triggerLabel="Choose condition owner" noneLabel="Choose a person" onChange={(value) => onChange(value === '__none__' ? '' : value)} renderTrigger={() => <span>{members.data?.find((member) => member.id === rule.value)?.display_name || (rule.value ? 'Member unavailable' : 'Choose a person')}</span>} />;
    if (rule.field === 'pipeline_id' || rule.field === 'stage_id') {
      const field = fields.find((entry) => entry.field === rule.field)!;
      const options = [...(field.options ?? [])];
      if (rule.value && !options.some((option) => option.value === rule.value)) options.push({ value: rule.value, label: `${field.label} unavailable` });
      return <PlaybookSelect label={`Condition ${field.label.toLowerCase()}`} value={rule.value || '__none__'} disabled={pipelines.isPending || pipelines.isError} options={[{ value: '__none__', label: options.length ? `Choose ${field.label.toLowerCase()}` : `No ${rule.field === 'pipeline_id' ? 'pipelines' : 'stages'} available` }, ...options]} onChange={(value) => onChange(value === '__none__' ? '' : value)} />;
    }
    return undefined;
  };
  return <>
    <form onSubmit={(event) => { event.preventDefault(); if (!disabled && !stale && validSelection) void save(); }} className="max-w-3xl">
      <div className="sticky top-0 z-10 flex flex-wrap items-center justify-between gap-3 border-b border-quiet-divider-strong bg-quiet-surface py-4">
        <p className="text-xs text-quiet-text-tertiary">{canAdmin ? dirty ? 'Unsaved changes' : 'Saved draft' : 'Read-only configuration'}{item.published_version && ' · Existing signals keep their published version'}</p>
        <div className="flex items-center gap-2"><QuietTextAction type="button" disabled={dirty || write.isPending} onClick={onPreview}>Preview matches</QuietTextAction>{canAdmin && <QuietPrimaryAction type="submit" disabled={!dirty || disabled || stale || !validSelection}>{write.isPending ? 'Saving…' : 'Save draft'}</QuietPrimaryAction>}</div>
      </div>
      {stale && dirty && <div role="alert" className="border-b border-quiet-divider-strong py-3 text-sm text-quiet-accent">This playbook changed while you were editing. Your draft is still here.<QuietTextAction type="button" onClick={() => setDiscard(true)}>Review saved version</QuietTextAction></div>}
      {write.isError && <PlaybookError error={write.error} retry={onReload} />}
      <fieldset disabled={disabled} className="min-w-0 [&_input:disabled]:opacity-100 [&_textarea:disabled]:opacity-100 [&_button:disabled]:opacity-100">
        <QuietSection title="Customer outcome"><div className="space-y-5">
          <PlaybookField label="Name">{(id) => <QuietUnderlineInput id={id} value={definition.name} onChange={(event) => update({ name: event.target.value })} maxLength={160} required />}</PlaybookField>
          <PlaybookField label="Outcome" help="Describe what changes for the customer. A completed task or Agent run is not proof of an outcome.">{(id) => <PlaybookTextarea id={id} value={definition.objective} onChange={(event) => update({ objective: event.target.value })} maxLength={4000} placeholder="What should be true when this work succeeds?" />}</PlaybookField>
          <PlaybookField label="Description (optional)">{(id) => <PlaybookTextarea id={id} value={definition.description} onChange={(event) => update({ description: event.target.value })} maxLength={2000} placeholder="Useful context for your team" />}</PlaybookField>
        </div></QuietSection>
        <QuietSection title="Which signals qualify"><div className="space-y-4">
          <div className="flex flex-wrap gap-x-6 gap-y-3">{playbookMotions.map((motion) => <label key={motion.value} className="flex items-center gap-2 text-sm"><Checkbox checked={definition.eligibility.commercial_motions.includes(motion.value)} onCheckedChange={(checked) => update({ eligibility: { ...definition.eligibility, commercial_motions: checked ? [...definition.eligibility.commercial_motions, motion.value] : definition.eligibility.commercial_motions.filter((value) => value !== motion.value) } })} />{motion.label}</label>)}</div>
          {!definition.eligibility.commercial_motions.length && <p role="alert" className="text-xs text-quiet-accent">Choose at least one stage before saving.</p>}
          <div className="flex flex-wrap items-center gap-3">
            {!disabled && <QueryBuilderPopover fields={fields} value={definition.eligibility.filter} onApply={(filter) => update({ eligibility: { ...definition.eligibility, filter } })} allowOr maxRules={30} presentation="quiet" triggerLabel="Conditions" triggerVariant="ghost" emptyDescription="Add conditions to narrow which signals qualify." renderValue={renderConditionValue} />}
            <span className="text-xs text-quiet-text-tertiary">{definition.eligibility.filter?.rules.length ? `${definition.eligibility.filter.rules.length} conditions · match ${definition.eligibility.filter.logic === 'and' ? 'all' : 'any'}` : 'No additional conditions'}</span>
          </div>
          {definition.eligibility.filter?.rules.map((rule, index) => <p key={index} className="text-xs text-quiet-text-secondary">{fields.find((field) => field.field === rule.field)?.label || rule.field} {rule.operator.replaceAll('_', ' ')} {recordFields[rule.field] && rule.value ? <CRMRecordPicker workspaceId={ws} targetType={recordFields[rule.field]} value={rule.value} disabled onChange={() => {}} /> : fields.find((field) => field.field === rule.field)?.options?.find((option) => option.value === rule.value)?.label || rule.value || rule.values?.join(' to ')}</p>)}
          {(members.isError || pipelines.isError) && <PlaybookError error={members.error || pipelines.error} retry={() => { void members.refetch(); void pipelines.refetch(); }} />}
        </div></QuietSection>
        <QuietSection title="Milestones" action={!disabled && <QuietTextAction type="button" disabled={(definition.milestones?.length ?? 0) >= 20} onClick={() => update({ milestones: [...(definition.milestones ?? []), { key: newMilestoneKey(), name: '', success_criteria: '' }] })}>Add milestone</QuietTextAction>}>
          {!definition.milestones?.length && <p className="text-sm text-quiet-text-tertiary">Add the checkpoints that demonstrate progress toward the customer outcome.</p>}
          <div className="divide-y divide-quiet-divider-light">{definition.milestones?.map((milestone, index) => <div key={milestone.key} className="space-y-3 py-4 first:pt-0">
            <div className="flex items-start gap-4"><span className="pt-2 text-xs tabular-nums text-quiet-text-tertiary">{index + 1}</span><div className="min-w-0 flex-1 space-y-3">
              <PlaybookField label={`Milestone ${index + 1}`}>{(id) => <QuietUnderlineInput id={id} value={milestone.name} required maxLength={160} placeholder="Name the milestone" onChange={(event) => update({ milestones: definition.milestones!.map((entry) => entry.key === milestone.key ? { ...entry, name: event.target.value } : entry) })} />}</PlaybookField>
              <PlaybookField label="Success criteria">{(id) => <PlaybookTextarea id={id} value={milestone.success_criteria} maxLength={2000} placeholder="What evidence confirms this milestone?" onChange={(event) => update({ milestones: definition.milestones!.map((entry) => entry.key === milestone.key ? { ...entry, success_criteria: event.target.value } : entry) })} />}</PlaybookField>
            </div>{!disabled && <QuietTextAction type="button" aria-label={`Remove milestone ${index + 1}`} onClick={() => update({ milestones: definition.milestones!.filter((entry) => entry.key !== milestone.key) })}>Remove</QuietTextAction>}</div>
          </div>)}</div>
        </QuietSection>
        <QuietSection title="Responsibilities"><div className="space-y-5">
          <PlaybookField label="Responsible owner" help="A role in the playbook, not a reassignment of existing work. Applying a playbook preserves each signal’s current owner.">{(id) => <PlaybookSelect id={id} label="Responsible owner" value={definition.responsibilities.owner_role} options={ownerRoles} disabled={disabled} onChange={(value) => update({ responsibilities: { ...definition.responsibilities, owner_role: value as CRMPlaybookDefinition['responsibilities']['owner_role'] } })} />}</PlaybookField>
          <PlaybookField label="Approvals go to">{(id) => <PlaybookSelect id={id} label="Approvals go to" value={definition.responsibilities.approver_role} options={[{ value: 'next_action_owner', label: 'Next action owner' }, { value: 'signal_owner', label: 'Signal owner' }]} disabled={disabled} onChange={(value) => update({ responsibilities: { ...definition.responsibilities, approver_role: value as 'signal_owner' | 'next_action_owner' } })} />}</PlaybookField>
          <PlaybookField label="Escalation owner" help="The person responsible when work needs help. An active workspace member is required to publish.">{() => <MemberPickerPopover members={members.data ?? []} value={definition.responsibilities.escalation_member_id || ''} disabled={disabled || members.isPending || members.isError} triggerLabel="Choose escalation owner" noneLabel="Not assigned" onChange={(value) => update({ responsibilities: { ...definition.responsibilities, escalation_member_id: value === '__none__' ? null : value } })} renderTrigger={() => <span>{members.isPending ? 'Loading members…' : escalationMember ? escalationMember.display_name || escalationMember.email : definition.responsibilities.escalation_member_id ? 'Member unavailable — choose another' : 'Choose a person'}</span>} />}</PlaybookField>
        </div></QuietSection>
        <QuietSection title="Action permissions"><div className="space-y-4">
          <p className="text-sm leading-6 text-quiet-text-tertiary">These are the playbook’s rules. Saving or publishing them does not start automation.</p>
          {([{ key: 'outbound_messages', label: 'Customer messages' }, { key: 'crm_changes', label: 'CRM changes' }, { key: 'pm_tasks', label: 'Tasks', help: 'Allow a task only when someone needs to do concrete work. This does not create a task for every signal.' }] as const).map((field) => <PlaybookField key={field.key} label={field.label} help={'help' in field ? field.help : undefined}>{(id) => <PlaybookSelect id={id} label={field.label} value={definition.policy[field.key]} disabled={disabled} options={approvalOptions} onChange={(value) => update({ policy: { ...definition.policy, [field.key]: value } })} />}</PlaybookField>)}
        </div></QuietSection>
        <QuietSection title="Follow-up and stopping"><div className="space-y-5">
          <PlaybookField label="Check after (hours)" help="The intended review interval. Playbook settings do not schedule checks until execution is connected.">{(id) => <QuietUnderlineInput id={id} type="number" min={1} max={8760} required value={definition.policy.check_after_hours || ''} onChange={(event) => update({ policy: { ...definition.policy, check_after_hours: Number(event.target.value) } })} />}</PlaybookField>
          <PlaybookField label="Escalate after (hours)">{(id) => <QuietUnderlineInput id={id} type="number" min={definition.policy.check_after_hours || 1} max={8760} required value={definition.policy.escalate_after_hours || ''} onChange={(event) => update({ policy: { ...definition.policy, escalate_after_hours: Number(event.target.value) } })} />}</PlaybookField>
          <fieldset className="space-y-3"><legend className="mb-3 text-sm font-medium text-quiet-text-secondary">Stop when</legend>{stops.map((stop) => <label key={stop.value} className="flex items-center gap-2 text-sm"><Checkbox checked={definition.policy.stop_conditions.includes(stop.value)} onCheckedChange={(checked) => update({ policy: { ...definition.policy, stop_conditions: checked ? [...definition.policy.stop_conditions, stop.value] : definition.policy.stop_conditions.filter((value) => value !== stop.value) } })} />{stop.label}</label>)}</fieldset>
          {!definition.policy.stop_conditions.length && <p role="alert" className="text-xs text-quiet-accent">Choose at least one stop condition before saving.</p>}
        </div></QuietSection>
      </fieldset>
    </form>
    <Dialog open={discard || blocker.status === 'blocked'} onOpenChange={(open) => { if (!open) { setDiscard(false); blocker.reset?.(); } }}><DialogContent><DialogHeader><DialogTitle>Discard unsaved changes?</DialogTitle><DialogDescription>Your saved playbook is safe. The edits on this page will be lost.</DialogDescription></DialogHeader><DialogFooter><QuietTextAction onClick={() => { setDiscard(false); blocker.reset?.(); }}>Keep editing</QuietTextAction><QuietPrimaryAction onClick={() => { if (blocker.status === 'blocked') blocker.proceed(); else { setDefinition(item.playbook.draft); setBaseline(item.playbook); setDiscard(false); write.reset(); } }}>Discard changes</QuietPrimaryAction></DialogFooter></DialogContent></Dialog>
  </>;
}
