import { useState } from 'react';
import { QuietStatusText, QuietTextAction, QuietPrimaryAction } from '@/components/design-system/quiet';
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog';
import { useCRMPlaybookAutomation, useCRMSignalAutomation, useCRMPlaybookAutomationWrite } from '@/hooks/queries/useCRMPlaybookAutomation';
import { createPlaybookIntentKey } from '@/lib/crmPlaybookPresentation';
import { playbookAutomationStatus } from '@/lib/crmPlaybookAutomationPresentation';
import type { CRMSituationItem } from '@/lib/crmSituationTypes';
import type { CRMPlaybookAutomationAdoption } from '@/lib/crmPlaybookTypes';
import { PlaybookError, PlaybookHelp } from '../playbooks/PlaybookUI';

export function SignalAutomation({ ws, item, canEdit }: { ws: string; item: CRMSituationItem; canEdit: boolean }) {
  const signal = item.situation;
  const overview = useCRMPlaybookAutomation(ws, signal.playbook_id || '');
  const execution = useCRMSignalAutomation(ws, signal.id);
  const write = useCRMPlaybookAutomationWrite(ws);
  const [confirmation, setConfirmation] = useState<CRMPlaybookAutomationAdoption>();
  const [intentKey] = useState(createPlaybookIntentKey);
  if (overview.isPending || execution.isPending) return <p className="py-3 text-xs text-quiet-text-tertiary">Loading automation…</p>;
  if (overview.isError || execution.isError) return <PlaybookError error={overview.error || execution.error} retry={() => { void overview.refetch(); void execution.refetch(); }} />;
  const binding = execution.data;
  const connection = overview.data.connection;
  if (!connection && !binding) return null;
  const ready = overview.data.runtime_available && overview.data.settings?.enabled && (!!binding || connection?.playbook_version_id === signal.playbook_version_id);
  const request = () => {
    const body = { expected_revision: signal.revision, expected_generation: binding?.generation || 0, connection_id: binding?.connection_id || connection?.id || '', enabled: !binding?.enabled, confirmed: true as const };
    setConfirmation({ ...body, command_key: intentKey(body) });
  };
  const stale = confirmation && (confirmation.expected_revision !== signal.revision || confirmation.expected_generation !== (binding?.generation || 0));
  const save = async () => {
    if (!confirmation || stale) return;
    try { await write.mutateAsync({ kind: 'adopt', id: signal.id, body: confirmation }); setConfirmation(undefined); } catch { /* Keep explicit approval and receipt identity. */ }
  };
  return <div className="border-b border-quiet-divider-strong py-4 text-xs">
    <div className="flex items-center justify-between gap-3"><div className="flex items-center gap-1"><QuietStatusText tone={binding?.blocker && binding.enabled ? 'blocker' : 'neutral'}>{playbookAutomationStatus(binding)}</QuietStatusText><PlaybookHelp label="About Beacon’s checks">Beacon reviews the playbook and current customer updates. A completed check is not customer progress. Messages, tasks and record changes need the responsible teammate’s approval.</PlaybookHelp></div>
      {canEdit && signal.lifecycle === 'open' && (binding?.enabled || ready) && <QuietTextAction disabled={write.isPending} onClick={request}>{binding?.enabled ? 'Pause automation' : binding ? 'Resume automation' : 'Start Beacon'}</QuietTextAction>}
    </div>
    {binding?.last_checked_at && <p className="mt-2 text-quiet-text-tertiary">Last checked {new Date(binding.last_checked_at).toLocaleString()}</p>}
    {binding?.escalated_at && binding.enabled && <p className="mt-2 text-quiet-accent">Needs a teammate’s review. This signal is also in the escalation owner’s attention queue.</p>}
    {!binding?.enabled && !ready && signal.lifecycle === 'open' && <p className="mt-2 text-quiet-text-tertiary">{!overview.data.settings?.enabled ? 'Turn on automation in Playbook Setup to start Beacon.' : !overview.data.runtime_available ? 'Automation is currently unavailable.' : 'This signal uses a different playbook version. Review its connection before starting automation.'}</p>}
    <Dialog open={!!confirmation} onOpenChange={(open) => { if (!open && !write.isPending) { setConfirmation(undefined); write.reset(); } }}><DialogContent><DialogHeader><DialogTitle>{confirmation?.enabled ? 'Start Beacon for this signal?' : 'Pause automation for this signal?'}</DialogTitle><DialogDescription>{confirmation?.enabled ? 'Beacon will review this customer against the signal’s playbook and prepare the next action for your team. No message is sent or task created without approval.' : 'Stops future checks and requests cancellation of any active check. Actions already executing may still finish. The signal stays open.'}</DialogDescription></DialogHeader>
      {stale && <p role="alert" className="text-sm text-quiet-accent">This signal changed. Close and review it before confirming again.</p>}
      {write.isError && <PlaybookError error={write.error} />}
      <DialogFooter><QuietTextAction disabled={write.isPending} onClick={() => { setConfirmation(undefined); write.reset(); }}>Cancel</QuietTextAction><QuietPrimaryAction disabled={write.isPending || !!stale} onClick={() => void save()}>{write.isPending ? 'Saving…' : confirmation?.enabled ? 'Start Beacon' : 'Pause automation'}</QuietPrimaryAction></DialogFooter>
    </DialogContent></Dialog>
  </div>;
}
