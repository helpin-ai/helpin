import { useState } from 'react';
import { Link } from '@tanstack/react-router';
import { toast } from 'sonner';
import { QuietSelect, QuietPrimaryAction, QuietPropertyRow, QuietSection, QuietStatusText, QuietTextAction, QuietUnderlineInput } from '@/components/design-system/quiet';
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog';
import { useCRMPlaybookAutomation, useCRMPlaybookAutomationWrite } from '@/hooks/queries/useCRMPlaybookAutomation';
import { usePermissions, useWorkspaceAccess } from '@/hooks/queries/useSession';
import { createPlaybookIntentKey } from '@/lib/crmPlaybookPresentation';
import type { CRMPlaybookAutomationOverview, CRMPlaybookConnectionReview, CRMPlaybookItem } from '@/lib/crmPlaybookTypes';
import { PlaybookError, PlaybookField, PlaybookHelp, PlaybookLabel } from './PlaybookUI';

export function PlaybookAutomation({ ws, slug, item, dirty }: { ws: string; slug: string; item: CRMPlaybookItem; dirty: boolean }) {
  const access = useWorkspaceAccess(ws);
  const { has, canAccessModule } = usePermissions(access.data);
  const canConfigure = has('crm.admin') && has('pm.admin.automations') && canAccessModule('automation');
  const overview = useCRMPlaybookAutomation(ws, item.playbook.id);
  if (overview.isPending) return <p className="py-4 text-xs text-quiet-text-tertiary">Loading automation…</p>;
  if (overview.isError) return <PlaybookError error={overview.error} retry={() => void overview.refetch()} />;
  return <AutomationSettings key={`${overview.data.settings?.revision || 0}:${overview.data.connection?.id || ''}`} ws={ws} slug={slug} item={item} overview={overview.data} canConfigure={canConfigure} dirty={dirty} />;
}

function AutomationSettings({ ws, slug, item, overview, canConfigure, dirty }: { ws: string; slug: string; item: CRMPlaybookItem; overview: CRMPlaybookAutomationOverview; canConfigure: boolean; dirty: boolean }) {
  const write = useCRMPlaybookAutomationWrite(ws);
  const [review, setReview] = useState<CRMPlaybookConnectionReview>();
  const [confirm, setConfirm] = useState<'enable' | 'disable' | 'save'>();
  const [intentKey] = useState(createPlaybookIntentKey);
  const settings = overview.settings;
  const [entry, setEntry] = useState(settings?.entry_mode || 'manual');
  const [maxRuns, setMaxRuns] = useState(settings?.max_runs_per_day || 4);
  const [maxNoProgress, setMaxNoProgress] = useState(settings?.max_no_progress_runs || 3);
  const connectionChanged = !!settings && overview.connection?.id !== settings.connection_id;
  const changed = connectionChanged || entry !== (settings?.entry_mode || 'manual') || maxRuns !== (settings?.max_runs_per_day || 4) || maxNoProgress !== (settings?.max_no_progress_runs || 3);
  const connectionCurrent = overview.connection?.playbook_version_id === item.published_version?.id;
  const policy = item.published_version?.definition;
  const setupIssue = policy?.journey === 'custom' ? 'This playbook is managed manually. Automation supports buying-intent follow-up, sales handoff and renewal-risk recovery.' : policy?.journey === 'sales_handoff' && !policy.milestones?.some((milestone) => milestone.key === 'handoff_accepted') ? 'Include the Handoff accepted milestone to connect handoff automation.' : undefined;
  const valid = maxRuns >= 1 && maxRuns <= 24 && maxNoProgress >= 1 && maxNoProgress <= 10 && (entry !== 'automatic' || item.playbook.accepting_customers);
  const prepare = async () => {
    if (!item.published_version || setupIssue) return;
    try {
      const result = await write.mutateAsync({ kind: 'prepare', id: item.playbook.id, body: { expected_revision: item.playbook.revision, playbook_version_id: item.published_version.id } });
      if ('review_fingerprint' in result) setReview(result);
    } catch { /* Keep errors beside the configuration. */ }
  };
  const publish = async () => {
    if (!review) return;
    const body = { playbook_version_id: review.playbook_version_id, expected_revision: review.expected_revision, flow_id: review.flow_id, agent_id: review.agent_id, expected_connection_version: review.connection_version, review_fingerprint: review.review_fingerprint };
    try {
      await write.mutateAsync({ kind: 'publish', id: item.playbook.id, body: { ...body, command_key: intentKey(body) } });
      setReview(undefined); toast.success('Automation connected. Turn it on when you’re ready.');
    } catch { /* The exact reviewed configuration must still match. */ }
  };
  const configure = async () => {
    if (!overview.connection || !confirm) return;
    const body = { expected_revision: settings?.revision || 0, connection_id: confirm === 'disable' && settings ? settings.connection_id : overview.connection.id, enabled: confirm === 'disable' ? false : confirm === 'enable' ? true : !!settings?.enabled, entry_mode: entry, max_runs_per_day: maxRuns, max_no_progress_runs: maxNoProgress, confirmed: true as const };
    try {
      await write.mutateAsync({ kind: 'configure', id: item.playbook.id, body: { ...body, command_key: intentKey(body) } });
      setConfirm(undefined); toast.success(body.enabled ? 'Automation settings saved' : 'Automation paused');
    } catch { /* No automatic retry of activation. */ }
  };
  return <QuietSection className="max-w-3xl" title={<PlaybookLabel label="Automation" help="Optional: Beacon reviews updates and proposes next actions for your team to approve. Connect it, then turn it on when ready." />} action={<QuietStatusText tone={settings?.enabled ? 'positive' : 'neutral'}>{settings?.enabled ? 'On' : overview.connection ? 'Off' : 'Not connected'}</QuietStatusText>}>
    {setupIssue && <p className="mb-3 text-xs text-quiet-text-secondary">{setupIssue}</p>}
    {!item.published_version ? <p className="text-xs text-quiet-text-tertiary">Publish the playbook to connect automation.</p> : !overview.connection ? <>{canConfigure ? <QuietPrimaryAction disabled={write.isPending || dirty || !!setupIssue} onClick={() => void prepare()}>{write.isPending ? 'Preparing…' : 'Set up automation'}</QuietPrimaryAction> : <p className="text-xs text-quiet-text-tertiary">An admin with Automation access can connect this playbook.</p>}</> : <>
      <QuietPropertyRow label="Agent" value={canConfigure ? <Link to="/w/$slug/automation/agents" params={{ slug }} search={{ agent_id: undefined }} className="text-quiet-accent hover:underline">{overview.agent_name || 'Beacon'}</Link> : overview.agent_name || 'Beacon'} />
      <QuietPropertyRow label="Flow" value={<span>{overview.flow_name}<PlaybookHelp label="About the connected flow">Runs Beacon when new information arrives or a check is due. Managed through this playbook.</PlaybookHelp></span>} />
      {canConfigure && <div className="mt-4 space-y-4">
        <PlaybookField label="Start automation for" help="Choose whether new matching signals start automatically or your team starts each one. Existing and paused signals must be started individually.">{(id) => <QuietSelect id={id} label="Start automation for" value={entry} onChange={(value) => setEntry(value as typeof entry)} disabled={write.isPending} options={[{ value: 'manual', label: 'Signals the team starts' }, { value: 'automatic', label: 'New matching signals' }]} />}</PlaybookField>
        {entry === 'automatic' && !item.playbook.accepting_customers && <p className="text-xs text-quiet-accent">Allow new enrollment above before choosing automatic entry.</p>}
        <details className="text-xs text-quiet-text-secondary"><summary className="cursor-pointer">Usage limits</summary><div className="mt-3 grid grid-cols-2 gap-5">
          <PlaybookField label="Checks per signal / day" help="Maximum daily checks for each signal. Waiting for approval does not require another check.">{(id) => <QuietUnderlineInput id={id} type="number" min={1} max={24} value={maxRuns} onChange={(event) => setMaxRuns(Number(event.target.value))} disabled={write.isPending} />}</PlaybookField>
          <PlaybookField label="Checks without progress" help="Flag the signal for attention after this many checks without progress.">{(id) => <QuietUnderlineInput id={id} type="number" min={1} max={10} value={maxNoProgress} onChange={(event) => setMaxNoProgress(Number(event.target.value))} disabled={write.isPending} />}</PlaybookField>
        </div></details>
        {!overview.runtime_available && <p role="status" className="text-xs text-quiet-accent">Automation is currently unavailable. You can review setup, but cannot start new work.</p>}
        {!connectionCurrent && <p className="text-xs text-quiet-accent">Automation uses an earlier playbook version. Connect the published version before starting new signals.</p>}
        {connectionChanged && connectionCurrent && <p className="text-xs text-quiet-text-secondary">An updated connection is ready. Save settings to use it for newly started signals. Work in progress keeps its reviewed connection.</p>}
        <div className="flex flex-wrap items-center gap-3">
          <QuietPrimaryAction disabled={write.isPending || dirty || !settings?.enabled && (!valid || !overview.runtime_available)} onClick={() => setConfirm(settings?.enabled ? 'disable' : 'enable')}>{settings?.enabled ? 'Pause automation' : 'Turn on automation'}</QuietPrimaryAction>
          {changed && <QuietTextAction disabled={write.isPending || dirty || !valid} onClick={() => setConfirm('save')}>Save settings</QuietTextAction>}
          <QuietTextAction disabled={write.isPending || dirty || !!setupIssue} onClick={() => void prepare()}>{connectionCurrent ? 'Review connection' : 'Review updated connection'}</QuietTextAction>
        </div>
      </div>}
    </>}
    {write.isError && !review && !confirm && <PlaybookError error={write.error} />}
    <Dialog open={!!review} onOpenChange={(open) => { if (!open && !write.isPending) { setReview(undefined); write.reset(); } }}><DialogContent><DialogHeader><DialogTitle>Connect automation</DialogTitle><DialogDescription>A dedicated Flow will use the existing CRM agent and these playbook skills. Connecting starts no work.</DialogDescription></DialogHeader>
      {review && <><QuietPropertyRow label="Flow" value={review.flow_name} /><QuietPropertyRow label="Agent" value={review.agent_name} /><QuietPropertyRow label="Skills" value={<ul className="space-y-1">{review.skills.map((skill) => <li key={skill.key}>{skill.title}</li>)}</ul>} /></>}
      {write.isError && <PlaybookError error={write.error} />}
      <DialogFooter><QuietTextAction disabled={write.isPending} onClick={() => { setReview(undefined); write.reset(); }}>Cancel</QuietTextAction><QuietPrimaryAction disabled={write.isPending || !review || review.expected_revision !== item.playbook.revision} onClick={() => void publish()}>{write.isPending ? 'Connecting…' : 'Connect automation'}</QuietPrimaryAction></DialogFooter>
    </DialogContent></Dialog>
    <Dialog open={!!confirm} onOpenChange={(open) => { if (!open && !write.isPending) { setConfirm(undefined); write.reset(); } }}><DialogContent><DialogHeader><DialogTitle>{confirm === 'disable' ? 'Pause automation?' : confirm === 'enable' ? 'Turn on automation?' : 'Save automation settings?'}</DialogTitle><DialogDescription>{confirm === 'disable' ? 'Stops future checks and requests cancellation of active runs. Work already sent or created cannot be recalled. Existing signals remain open.' : entry === 'automatic' ? 'New qualifying signals can enter this playbook and start Beacon. Existing and paused signals will not be started. Every proposed action still needs approval.' : 'The team can start Beacon from individual signals. No existing or paused signal starts automatically.'}</DialogDescription></DialogHeader>
      {write.isError && <PlaybookError error={write.error} />}
      {confirm !== 'disable' && connectionChanged && <p className="text-sm text-quiet-text-secondary">This adopts the updated connection for newly started signals only. Existing signals keep their pinned configuration.</p>}
      <DialogFooter><QuietTextAction disabled={write.isPending} onClick={() => { setConfirm(undefined); write.reset(); }}>Cancel</QuietTextAction><QuietPrimaryAction disabled={write.isPending || confirm !== 'disable' && !valid} onClick={() => void configure()}>{write.isPending ? 'Saving…' : confirm === 'disable' ? 'Pause automation' : confirm === 'enable' ? 'Turn on automation' : 'Save settings'}</QuietPrimaryAction></DialogFooter>
    </DialogContent></Dialog>
  </QuietSection>;
}
