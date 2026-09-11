import { useEffect, useId, useMemo, useRef, useState } from 'react';
import { createPortal } from 'react-dom';
import { Button } from '@/components/ui/button';
import { useBlocker } from '@tanstack/react-router';
import { toast } from 'sonner';
import { QuietSelect, QuietPrimaryAction, QuietSection, QuietTextAction, QuietUnderlineInput } from '@/components/design-system/quiet';
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
import { PlaybookError, PlaybookField, PlaybookHelp, PlaybookLabel, PlaybookTextarea } from './PlaybookUI';
import { playbookSetupIssues, playbookSetupSteps, type PlaybookSetupStep } from '@/lib/crmPlaybookSetup';
import { PlaybookSetupNav } from './PlaybookSetupNav';
import { PlaybookPublishAction } from './PlaybookPublishAction';

const recordFields: Record<string, CRMRecordTargetType> = { company_id: 'crm_company', contact_id: 'crm_contact', deal_id: 'crm_deal' };
const ownerRoles = [{ value: 'signal_owner', label: 'Signal owner' }, { value: 'account_owner', label: 'Company owner' }, { value: 'deal_owner', label: 'Deal owner' }, { value: 'customer_success_owner', label: 'Customer success owner' }];
const approvalOptions = [{ value: 'approval_required', label: 'Approval required' }, { value: 'not_allowed', label: 'Not allowed' }];
const stops: { value: CRMPlaybookDefinition['policy']['stop_conditions'][number]; label: string }[] = [
  { value: 'objective_achieved', label: 'Desired outcome achieved' }, { value: 'customer_declined', label: 'Customer declines' },
  { value: 'no_longer_eligible', label: 'Signal no longer matches' }, { value: 'contact_restricted', label: 'Contact restricted' },
];

export function PlaybookEditor({ ws, item, canAdmin, onDirty, onReload, onPreview, step, onStepChange, onPublish, onSaveError, publishing, saveActionContainer }: {
  ws: string; item: CRMPlaybookItem; canAdmin: boolean; onDirty: (dirty: boolean) => void; onReload: () => void; onPreview: () => void;
  saveActionContainer: HTMLDivElement | null;
  step: PlaybookSetupStep; onStepChange: (step: PlaybookSetupStep) => void; onPublish: (revision: number) => void; onSaveError: () => void; publishing: boolean;
}) {
  const formId = useId();
  const form = useRef<HTMLFormElement>(null);
  const heading = useRef<HTMLHeadingElement>(null);
  const saving = useRef(false);
  const [validationError, setValidationError] = useState('');
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
  const disabled = !canAdmin || write.isPending || publishing;
  const validSelection = definition.eligibility.commercial_motions.length > 0 && definition.policy.stop_conditions.length > 0;
  const update = (patch: Partial<CRMPlaybookDefinition>) => { setValidationError(''); setDefinition((current) => ({ ...current, ...patch })); };
  const go = (next: PlaybookSetupStep) => {
    onStepChange(next);
    requestAnimationFrame(() => heading.current?.focus({ preventScroll: false }));
  };
  const issues = playbookSetupIssues(definition, members.isPending ? 'loading' : members.isError ? 'error' : !!members.data?.find((member) => member.id === definition.responsibilities.escalation_member_id && member.status === 'active'));
  const ready = Object.values(issues).every((entries) => entries.length === 0);
  const complete = { purpose: !issues.purpose.length, milestones: !issues.milestones.length, team: !issues.team.length, follow_up: !issues.follow_up.length };
  const index = playbookSetupSteps.findIndex((entry) => entry.id === step);
  const submit = () => {
    if (disabled || stale || !dirty || saving.current) return;
    const invalid = form.current?.querySelector<HTMLInputElement | HTMLTextAreaElement>('input:invalid, textarea:invalid');
    if (invalid) {
      onStepChange(invalid.closest<HTMLElement>('[data-setup-step]')!.dataset.setupStep as PlaybookSetupStep);
      setValidationError('Check the highlighted field before saving.');
      requestAnimationFrame(() => { invalid.focus(); invalid.reportValidity(); });
      return;
    }
    if (!validSelection) {
      go(definition.eligibility.commercial_motions.length ? 'follow_up' : 'purpose');
      setValidationError('Choose at least one matching stage and one stop condition before saving.');
      return;
    }
    setValidationError('');
    void save();
  };
  const save = async () => {
    if (disabled || stale || saving.current) return;
    saving.current = true;
    const intent = { expected_revision: baseline.revision, operation: 'update_draft' as const, definition };
    try {
      const result = await write.mutateAsync({ kind: 'command', id: item.playbook.id, body: { ...intent, command_key: intentKey(intent) } });
      if ('change' in result && 'draft' in result.change.after) {
        setBaseline(result.change.after);
        setDefinition(result.change.after.draft);
        toast.success('Draft saved');
        return result.change.after;
      }
    } catch {
      // Keep edits and reveal save errors even when publishing from another tab.
      onSaveError();
    } finally { saving.current = false; }
  };
  const escalationMember = members.data?.find((member) => member.id === definition.responsibilities.escalation_member_id);
  const showPublish = dirty || !item.published_version || definitionFingerprint(definition) !== definitionFingerprint(item.published_version.definition);
  const requestPublish = async () => {
    if (disabled || stale || !ready || saving.current) return;
    const saved = dirty ? await save() : baseline;
    if (saved) onPublish(saved.revision);
  };
  const renderConditionValue = (rule: QueryFilterRule, onChange: (value: string) => void) => {
    if (recordFields[rule.field]) return <CRMRecordPicker workspaceId={ws} targetType={recordFields[rule.field]} value={rule.value ?? ''} onChange={onChange} />;
    if (rule.field === 'owner_member_id') return <MemberPickerPopover members={members.data ?? []} value={rule.value || ''} disabled={members.isPending || members.isError} triggerLabel="Choose condition owner" noneLabel="Choose a person" onChange={(value) => onChange(value === '__none__' ? '' : value)} renderTrigger={() => <span>{members.data?.find((member) => member.id === rule.value)?.display_name || (rule.value ? 'Member unavailable' : 'Choose a person')}</span>} />;
    if (rule.field === 'pipeline_id' || rule.field === 'stage_id') {
      const field = fields.find((entry) => entry.field === rule.field)!;
      const options = [...(field.options ?? [])];
      if (rule.value && !options.some((option) => option.value === rule.value)) options.push({ value: rule.value, label: `${field.label} unavailable` });
      return <QuietSelect label={`Condition ${field.label.toLowerCase()}`} value={rule.value || '__none__'} disabled={pipelines.isPending || pipelines.isError} options={[{ value: '__none__', label: options.length ? `Choose ${field.label.toLowerCase()}` : `No ${rule.field === 'pipeline_id' ? 'pipelines' : 'stages'} available` }, ...options]} onChange={(value) => onChange(value === '__none__' ? '' : value)} />;
    }
    return undefined;
  };
  return <>
    <div className="max-w-6xl">
      <div className="sticky top-0 z-10 flex flex-wrap items-center justify-between gap-3 border-b border-quiet-divider-strong bg-quiet-surface py-4">
        <p className="text-xs text-quiet-text-tertiary">{canAdmin ? dirty ? 'Unsaved changes' : 'Saved draft' : 'Read-only configuration'}</p>
        {canAdmin && saveActionContainer && createPortal(<div className="flex items-center gap-2">
          <Button type="submit" variant="outline" size="sm" form={formId} disabled={!dirty || disabled || stale}>{write.isPending ? 'Saving…' : 'Save draft'}</Button>
          {showPublish && <PlaybookPublishAction issues={issues} reason={stale ? 'This playbook changed. Review the saved version before publishing.' : undefined} busy={write.isPending || publishing} onPublish={() => void requestPublish()} />}
        </div>, saveActionContainer)}
      </div>
      {stale && dirty && <div role="alert" className="border-b border-quiet-divider-strong py-3 text-sm text-quiet-accent">This playbook changed while you were editing. Your draft is still here.<QuietTextAction type="button" onClick={() => setDiscard(true)}>Review saved version</QuietTextAction></div>}
      {write.isError && <PlaybookError error={write.error} retry={onReload} />}
      {validationError && <p role="alert" className="py-3 text-sm text-quiet-accent">{validationError}</p>}
      <div className="grid min-w-0 gap-6 py-6 lg:grid-cols-[220px_minmax(0,1fr)] lg:gap-11">
      <PlaybookSetupNav current={step} complete={complete} onChange={go} />
      <div className="min-w-0 max-w-3xl">
      <div className="border-b border-quiet-divider-strong pb-5"><p className="mb-2 text-xs text-quiet-text-tertiary">Step {index + 1} of {playbookSetupSteps.length}</p><h2 ref={heading} tabIndex={-1} className="text-xl font-semibold tracking-tight text-quiet-text-primary focus-visible:outline-2 focus-visible:outline-quiet-field">{playbookSetupSteps[index].label}{step === 'milestones' && <PlaybookHelp label="About milestones">The checkpoints that show progress toward the desired outcome. Each needs a clear sign of completion.</PlaybookHelp>}{step === 'follow_up' && <PlaybookHelp label="About monitoring">Set when to check progress, escalate stalled signals, and stop automated work.</PlaybookHelp>}</h2></div>
      <form ref={form} id={formId} noValidate onSubmit={(event) => { event.preventDefault(); submit(); }}>
      <fieldset className="min-w-0 [&_input:disabled]:opacity-100 [&_textarea:disabled]:opacity-100 [&_button:disabled]:opacity-100">
        <div id="playbook-step-purpose" data-setup-step="purpose" hidden={step !== 'purpose'}>
        <QuietSection><div className="space-y-5">
          <PlaybookField label="Playbook name">{(id) => <QuietUnderlineInput disabled={disabled} id={id} value={definition.name} onChange={(event) => update({ name: event.target.value })} maxLength={160} required placeholder="e.g. Buying-intent follow-up" />}</PlaybookField>
          <PlaybookField label="Desired outcome" help="The result you want for the customer, such as agreeing on a demo or completing onboarding.">{(id) => <PlaybookTextarea disabled={disabled} id={id} value={definition.objective} onChange={(event) => update({ objective: event.target.value })} maxLength={4000} placeholder="e.g. Agree on a demo with the customer" />}</PlaybookField>
          <PlaybookField label="Description (optional)" help="Extra context to help your team understand this playbook.">{(id) => <PlaybookTextarea disabled={disabled} id={id} value={definition.description} onChange={(event) => update({ description: event.target.value })} maxLength={2000} />}</PlaybookField>
        </div></QuietSection>
        <QuietSection title={<PlaybookLabel label="Matching signals" help="Signals must match a selected customer stage and any conditions you add." />} action={<span className="flex items-center gap-1"><QuietTextAction type="button" disabled={dirty || write.isPending} onClick={onPreview}>Preview matches</QuietTextAction><PlaybookHelp label="About preview matches">See which signals match the saved draft. Save changes to preview them. Previewing does not add signals or start work.</PlaybookHelp></span>}><div className="space-y-4">
          <div className="flex flex-wrap gap-x-6 gap-y-3">{playbookMotions.map((motion) => <label key={motion.value} className="flex items-center gap-2 text-sm"><Checkbox disabled={disabled} checked={definition.eligibility.commercial_motions.includes(motion.value)} onCheckedChange={(checked) => update({ eligibility: { ...definition.eligibility, commercial_motions: checked ? [...definition.eligibility.commercial_motions, motion.value] : definition.eligibility.commercial_motions.filter((value) => value !== motion.value) } })} />{motion.label}</label>)}</div>
          {!definition.eligibility.commercial_motions.length && <p role="alert" className="text-xs text-quiet-accent">Select at least one stage.</p>}
          <div className="flex flex-wrap items-center gap-1">
            {!disabled && <QueryBuilderPopover fields={fields} value={definition.eligibility.filter} onApply={(filter) => update({ eligibility: { ...definition.eligibility, filter } })} allowOr maxRules={30} presentation="quiet" triggerLabel={definition.eligibility.filter?.rules.length ? 'Conditions' : 'Add conditions'} triggerVariant="ghost" emptyDescription="Choose which signals to include." renderValue={renderConditionValue} />}
            {!disabled && <PlaybookHelp label="About conditions">Narrow matching signals by details such as owner, company, or priority.</PlaybookHelp>}
            {!!definition.eligibility.filter?.rules.length && <span className="ml-2 text-xs text-quiet-text-tertiary">Match {definition.eligibility.filter.logic === 'and' ? 'all' : 'any'}</span>}
          </div>
          {definition.eligibility.filter?.rules.map((rule, index) => <p key={index} className="text-xs text-quiet-text-secondary">{fields.find((field) => field.field === rule.field)?.label || rule.field} {rule.operator.replaceAll('_', ' ')} {recordFields[rule.field] && rule.value ? <CRMRecordPicker workspaceId={ws} targetType={recordFields[rule.field]} value={rule.value} disabled onChange={() => {}} /> : fields.find((field) => field.field === rule.field)?.options?.find((option) => option.value === rule.value)?.label || rule.value || rule.values?.join(' to ')}</p>)}
          {(members.isError || pipelines.isError) && <PlaybookError error={members.error || pipelines.error} retry={() => { void members.refetch(); void pipelines.refetch(); }} />}
        </div></QuietSection>
        </div>
        <div id="playbook-step-milestones" data-setup-step="milestones" hidden={step !== 'milestones'}>
        <QuietSection>
          {!definition.milestones?.length && <p className="text-sm text-quiet-text-tertiary">No milestones yet.</p>}
          <div className="divide-y divide-quiet-divider-light">{definition.milestones?.map((milestone, index) => <div key={milestone.key} className="space-y-3 py-4 first:pt-0">
            <div className="flex items-start gap-4"><div className="min-w-0 flex-1 space-y-3">
              <PlaybookField label={`Milestone ${index + 1}`}>{(id) => <QuietUnderlineInput disabled={disabled} id={id} value={milestone.name} required maxLength={160} placeholder="e.g. Demo completed" onChange={(event) => update({ milestones: definition.milestones!.map((entry) => entry.key === milestone.key ? { ...entry, name: event.target.value } : entry) })} />}</PlaybookField>
              <PlaybookField label="Success criteria" help="The evidence that confirms this milestone is complete.">{(id) => <PlaybookTextarea disabled={disabled} id={id} value={milestone.success_criteria} maxLength={2000} placeholder="e.g. Customer attended the demo and confirmed their requirements" onChange={(event) => update({ milestones: definition.milestones!.map((entry) => entry.key === milestone.key ? { ...entry, success_criteria: event.target.value } : entry) })} />}</PlaybookField>
            </div>{!disabled && <QuietTextAction type="button" aria-label={`Remove milestone ${index + 1}`} onClick={() => update({ milestones: definition.milestones!.filter((entry) => entry.key !== milestone.key) })}>Remove</QuietTextAction>}</div>
          </div>)}</div>
          {!disabled && <QuietTextAction className="mt-3" type="button" disabled={(definition.milestones?.length ?? 0) >= 20} onClick={() => update({ milestones: [...(definition.milestones ?? []), { key: newMilestoneKey(), name: '', success_criteria: '' }] })}>Add milestone</QuietTextAction>}
        </QuietSection>
        </div>
        <div id="playbook-step-team" data-setup-step="team" hidden={step !== 'team'}>
        <QuietSection title="Responsibilities"><div className="grid gap-x-6 gap-y-5 sm:grid-cols-3 [&>div>div:first-child]:min-h-7">
          <PlaybookField label="Owner role" help="The role responsible for this playbook’s work. Existing signal owners stay the same.">{(id) => <QuietSelect id={id} label="Owner role" value={definition.responsibilities.owner_role} options={ownerRoles} disabled={disabled} onChange={(value) => update({ responsibilities: { ...definition.responsibilities, owner_role: value as CRMPlaybookDefinition['responsibilities']['owner_role'] } })} />}</PlaybookField>
          <PlaybookField label="Approvals go to" help="Who reviews proposed actions before they are carried out.">{(id) => <QuietSelect id={id} label="Approvals go to" value={definition.responsibilities.approver_role} options={[{ value: 'next_action_owner', label: 'Next action owner' }, { value: 'signal_owner', label: 'Signal owner' }]} disabled={disabled} onChange={(value) => update({ responsibilities: { ...definition.responsibilities, approver_role: value as 'signal_owner' | 'next_action_owner' } })} />}</PlaybookField>
          <PlaybookField label="Escalation owner" help="Receives signals that need attention when progress stalls. Choose an active workspace member.">{() => <MemberPickerPopover members={members.data ?? []} value={definition.responsibilities.escalation_member_id || ''} disabled={disabled || members.isPending || members.isError} triggerClassName="h-8" triggerLabel="Choose escalation owner" noneLabel="Not assigned" onChange={(value) => update({ responsibilities: { ...definition.responsibilities, escalation_member_id: value === '__none__' ? null : value } })} renderTrigger={() => <span>{members.isPending ? 'Loading members…' : escalationMember ? escalationMember.display_name || escalationMember.email : definition.responsibilities.escalation_member_id ? 'Member unavailable — choose another' : 'Choose a person'}</span>} />}</PlaybookField>
        </div></QuietSection>
        <QuietSection title={<PlaybookLabel label="Action permissions" help="Choose which actions may be proposed for approval. Saving or publishing does not start automation." />}><div className="space-y-4">
          <div className="grid gap-x-6 gap-y-5 sm:grid-cols-3 [&>div>div:first-child]:min-h-7">{([{ key: 'outbound_messages', label: 'Customer messages' }, { key: 'crm_changes', label: 'CRM changes' }, { key: 'pm_tasks', label: 'Tasks', help: 'Allow tasks to be proposed when someone needs to act. Tasks are not created for every signal.' }] as const).map((field) => <PlaybookField key={field.key} label={field.label} help={'help' in field ? field.help : undefined}>{(id) => <QuietSelect id={id} label={field.label} value={definition.policy[field.key]} disabled={disabled} options={approvalOptions} onChange={(value) => update({ policy: { ...definition.policy, [field.key]: value } })} />}</PlaybookField>)}</div>
        </div></QuietSection>
        </div>
        <div id="playbook-step-follow_up" data-setup-step="follow_up" hidden={step !== 'follow_up'}>
        <QuietSection><div className="space-y-5">
          <div className="grid gap-x-6 gap-y-5 sm:grid-cols-2">
          <PlaybookField label="Check after (hours)" help="How long to wait before checking a signal for progress. Scheduled checks require automation to be on.">{(id) => <QuietUnderlineInput disabled={disabled} id={id} type="number" min={1} max={8760} required value={definition.policy.check_after_hours || ''} onChange={(event) => update({ policy: { ...definition.policy, check_after_hours: Number(event.target.value) } })} />}</PlaybookField>
          <PlaybookField label="Escalate after (hours)" help="With automation on, signals with no progress for this long appear in the escalation owner’s attention queue.">{(id) => <QuietUnderlineInput disabled={disabled} id={id} type="number" min={definition.policy.check_after_hours || 1} max={8760} required value={definition.policy.escalate_after_hours || ''} onChange={(event) => update({ policy: { ...definition.policy, escalate_after_hours: Number(event.target.value) } })} />}</PlaybookField>
          </div>
          <fieldset className="space-y-3"><legend className="mb-3 text-sm font-medium text-quiet-text-secondary"><PlaybookLabel label="Stop when" help="Stop automated follow-up when any selected condition is met." /></legend>{stops.map((stop) => <label key={stop.value} className="flex items-center gap-2 text-sm"><Checkbox disabled={disabled} checked={definition.policy.stop_conditions.includes(stop.value)} onCheckedChange={(checked) => update({ policy: { ...definition.policy, stop_conditions: checked ? [...definition.policy.stop_conditions, stop.value] : definition.policy.stop_conditions.filter((value) => value !== stop.value) } })} />{stop.label}</label>)}</fieldset>
          {!definition.policy.stop_conditions.length && <p role="alert" className="text-xs text-quiet-accent">Select at least one stop condition.</p>}
        </div></QuietSection>
        </div>
      </fieldset>
    </form>
    <div className="flex items-center justify-between gap-3 border-t border-quiet-divider-strong py-4"><QuietTextAction type="button" disabled={index === 0} onClick={() => go(playbookSetupSteps[index - 1].id)}>Back</QuietTextAction>{index < playbookSetupSteps.length - 1 && <Button type="button" variant="outline" size="sm" onClick={() => go(playbookSetupSteps[index + 1].id)}>Continue</Button>}</div>
    </div></div></div>
    <Dialog open={discard || blocker.status === 'blocked'} onOpenChange={(open) => { if (!open) { setDiscard(false); blocker.reset?.(); } }}><DialogContent><DialogHeader><DialogTitle>Discard unsaved changes?</DialogTitle><DialogDescription>Your saved playbook is safe. The edits on this page will be lost.</DialogDescription></DialogHeader><DialogFooter><QuietTextAction onClick={() => { setDiscard(false); blocker.reset?.(); }}>Keep editing</QuietTextAction><QuietPrimaryAction onClick={() => { if (blocker.status === 'blocked') blocker.proceed(); else { setDefinition(item.playbook.draft); setBaseline(item.playbook); setDiscard(false); write.reset(); } }}>Discard changes</QuietPrimaryAction></DialogFooter></DialogContent></Dialog>
  </>;
}
