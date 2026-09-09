import { useState } from 'react';
import { Link } from '@tanstack/react-router';
import { toast } from 'sonner';
import { QuietDetailAction, QuietDetailHeader, QuietEmptyState, QuietPageViewport, QuietPrimaryAction, QuietStatusText, QuietTextAction } from '@/components/design-system/quiet';
import { PlusSignIcon, Tick01Icon } from '@/lib/icons';
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs';
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog';
import { useCRMPlaybook, useCRMPlaybookHistory, useCRMPlaybookWrite } from '@/hooks/queries/useCRMPlaybooks';
import { useAssignableMembers } from '@/hooks/queries/useWorkspaces';
import { usePermissions, useWorkspaceAccess } from '@/hooks/queries/useSession';
import { useWorkspaceStore } from '@/stores/workspaceStore';
import { useTitle } from '@/hooks/useTitle';
import { createPlaybookIntentKey, definitionFingerprint, playbookStatus, publishIssues } from '@/lib/crmPlaybookPresentation';
import type { CRMPlaybookItem } from '@/lib/crmPlaybookTypes';
import { playbookSetupIssues, type PlaybookSetupStep } from '@/lib/crmPlaybookSetup';
import { PlaybookEditor } from '@/components/crm/playbooks/PlaybookEditor';
import { PlaybookAutomation } from '@/components/crm/playbooks/PlaybookAutomation';
import { PlaybookExecutionActivity } from '@/components/crm/playbooks/PlaybookExecutionActivity';
import { PlaybookParticipants, PlaybookPreview } from '@/components/crm/playbooks/PlaybookSignals';
import { PlaybookError, PlaybookLoading, PlaybookNoAccess } from '@/components/crm/playbooks/PlaybookUI';

export function PlaybookDetailPage({ playbookId }: { playbookId: string }) {
  useTitle('Playbook');
  const workspace = useWorkspaceStore((state) => state.currentWorkspace);
  const access = useWorkspaceAccess(workspace?.id || '');
  const { has } = usePermissions(access.data);
  if (access.isPending) return <QuietPageViewport><PlaybookLoading /></QuietPageViewport>;
  if (access.isError) return <QuietPageViewport><PlaybookError error={access.error} retry={() => void access.refetch()} /></QuietPageViewport>;
  if (!workspace || !has('crm.read')) return <QuietPageViewport><PlaybookNoAccess /></QuietPageViewport>;
  return <PlaybookLoader key={`${workspace.id}:${playbookId}`} ws={workspace.id} slug={workspace.slug} id={playbookId} canAdmin={has('crm.admin')} canEdit={has('crm.edit')} />;
}

function PlaybookLoader({ ws, slug, id, canAdmin, canEdit }: { ws: string; slug: string; id: string; canAdmin: boolean; canEdit: boolean }) {
  const book = useCRMPlaybook(ws, id);
  if (!book.data) return <QuietPageViewport>{book.isError ? <><Link to="/w/$slug/crm/playbooks" params={{ slug }}>Back to Playbooks</Link><PlaybookError error={book.error} retry={() => void book.refetch()} /></> : <PlaybookLoading />}</QuietPageViewport>;
  return <PlaybookDetail ws={ws} slug={slug} item={book.data} canAdmin={canAdmin} canEdit={canEdit} onReload={() => void book.refetch()} />;
}

function PlaybookDetail({ ws, slug, item, canAdmin, canEdit, onReload }: { ws: string; slug: string; item: CRMPlaybookItem; canAdmin: boolean; canEdit: boolean; onReload: () => void }) {
  const [tab, setTab] = useState(item.published_version ? 'work' : 'setup');
  const [dirty, setDirty] = useState(false);
  const [setupStep, setSetupStep] = useState<PlaybookSetupStep>('purpose');
  const [preview, setPreview] = useState<'draft' | 'published'>();
  const [confirm, setConfirm] = useState<'publish' | 'start' | 'stop'>();
  const [confirmationRevision, setConfirmationRevision] = useState(item.playbook.revision);
  const [intentKey] = useState(createPlaybookIntentKey);
  const write = useCRMPlaybookWrite(ws);
  const members = useAssignableMembers(ws);
  const unpublished = !item.published_version || definitionFingerprint(item.playbook.draft) !== definitionFingerprint(item.published_version.definition);
  const issues = publishIssues(item.playbook.draft);
  if (item.playbook.draft.responsibilities.escalation_member_id && members.isSuccess && !members.data.some((member) => member.id === item.playbook.draft.responsibilities.escalation_member_id && member.status === 'active')) issues.push('Choose an active escalation owner.');
  const openConfirmation = (operation: 'publish' | 'start' | 'stop') => {
    setConfirmationRevision(item.playbook.revision);
    setConfirm(operation);
  };
  const applyCommand = async () => {
    if (!confirm) return;
    const intent = confirm === 'publish' ? { expected_revision: confirmationRevision, operation: 'publish' as const } : { expected_revision: confirmationRevision, operation: 'set_enrollment' as const, accepting_customers: confirm === 'start' };
    try {
      await write.mutateAsync({ kind: 'command', id: item.playbook.id, body: { ...intent, command_key: intentKey(intent) } });
      toast.success(confirm === 'publish' ? 'Playbook published' : confirm === 'start' ? 'Enrollment allowed' : 'New enrollment stopped');
      setConfirm(undefined);
    } catch { /* Keep the explicit decision visible. */ }
  };
  return <div className="flex h-full min-h-0 flex-col">
    <QuietDetailHeader breadcrumbs={<Link to="/w/$slug/crm/playbooks" params={{ slug }} className="text-xs text-quiet-text-tertiary hover:text-quiet-text-primary">Playbooks</Link>} title={item.playbook.draft.name} meta={<span className="text-xs text-quiet-text-tertiary">{item.published_version ? `Published version ${item.published_version.version}${unpublished ? ' · Unpublished changes' : ''}` : 'Not published'}</span>} status={<QuietStatusText tone={item.playbook.accepting_customers ? 'positive' : 'neutral'}>{playbookStatus(item)}</QuietStatusText>} actions={<>
      {canEdit && item.playbook.accepting_customers && <QuietDetailAction icon={<PlusSignIcon className="size-3.5" />} label="Add signals" tone={unpublished && canAdmin ? 'secondary' : 'primary'} onClick={() => setPreview('published')} />}
      {canAdmin && unpublished && <QuietDetailAction icon={<Tick01Icon className="size-3.5" />} label="Publish" tone="primary" disabled={dirty || write.isPending} onClick={() => openConfirmation('publish')} />}
    </>} />
    <div className="min-h-0 flex-1 overflow-auto px-4 pb-12 sm:px-6 lg:px-8">
      <Tabs value={tab} onValueChange={setTab}>
        <TabsList variant="quiet"><TabsTrigger value="work">Signals</TabsTrigger><TabsTrigger value="setup">Setup{dirty ? ' · Unsaved' : ''}</TabsTrigger><TabsTrigger value="activity">Activity</TabsTrigger></TabsList>
        <TabsContent value="work"><PlaybookParticipants ws={ws} slug={slug} item={item} canEdit={canEdit} /></TabsContent>
        <TabsContent value="setup" forceMount hidden={tab !== 'setup'}>
          <PlaybookEditor ws={ws} item={item} canAdmin={canAdmin} onDirty={setDirty} onReload={onReload} onPreview={() => setPreview('draft')} step={setupStep} onStepChange={setSetupStep} renderReview={(editing) => <>
            {canAdmin && unpublished && <div className="border-b border-quiet-divider-strong py-4"><QuietPrimaryAction disabled={editing || write.isPending} onClick={() => openConfirmation('publish')}>Review & publish</QuietPrimaryAction></div>}
            {item.published_version && <div className="flex flex-wrap items-center justify-between gap-3 border-b border-quiet-divider-strong py-4 text-sm"><div><p>New enrollment is {item.playbook.accepting_customers ? 'allowed' : 'stopped'}</p><p className="mt-1 text-xs text-quiet-text-tertiary">Existing work is controlled separately. Signals are added with confirmation.</p></div>{canAdmin && <QuietTextAction disabled={write.isPending || editing} onClick={() => openConfirmation(item.playbook.accepting_customers ? 'stop' : 'start')}>{item.playbook.accepting_customers ? 'Stop new enrollment' : 'Allow new enrollment'}</QuietTextAction>}</div>}
            <PlaybookAutomation ws={ws} slug={slug} item={item} dirty={editing} />
          </>} />
        </TabsContent>
        <TabsContent value="activity"><PlaybookExecutionActivity ws={ws} slug={slug} id={item.playbook.id} /><PlaybookActivity ws={ws} id={item.playbook.id} /></TabsContent>
      </Tabs>
    </div>
    {preview && <PlaybookPreview key={`${preview}:${item.playbook.revision}`} ws={ws} item={item} mode={preview} canEdit={canEdit} onClose={() => setPreview(undefined)} onReload={onReload} />}
    <Dialog open={!!confirm} onOpenChange={(open) => { if (!open && !write.isPending) { setConfirm(undefined); write.reset(); } }}><DialogContent><DialogHeader><DialogTitle>{confirm === 'publish' ? 'Publish playbook?' : confirm === 'start' ? 'Allow new enrollment?' : 'Stop new enrollment?'}</DialogTitle><DialogDescription>{confirm === 'publish' ? 'Publish the saved draft for new signals. Existing signals keep their current version. Publishing does not change enrollment or start automation.' : confirm === 'start' ? 'Your team can apply the published playbook to matching signals after confirming each one. Nothing is enrolled automatically.' : 'No new signals can enter this playbook. Existing signals will not be paused or closed.'}</DialogDescription></DialogHeader>
      {confirm === 'publish' && issues.length > 0 && <div className="text-sm"><p className="font-medium">Complete setup before publishing:</p><ul className="mt-2 list-inside list-disc space-y-1 text-quiet-text-secondary">{issues.map((issue) => <li key={issue}>{issue}</li>)}</ul></div>}
      {write.isError && <PlaybookError error={write.error} retry={onReload} />}
      {confirm && confirmationRevision !== item.playbook.revision && <p role="alert" className="text-sm text-quiet-accent">The playbook changed. Cancel and review the saved settings before confirming again.</p>}
      <DialogFooter><QuietTextAction disabled={write.isPending} onClick={() => { setConfirm(undefined); write.reset(); }}>Cancel</QuietTextAction>{confirm === 'publish' && issues.length ? <QuietPrimaryAction onClick={() => {
        const missing = playbookSetupIssues(item.playbook.draft, members.isSuccess && !!members.data.find((member) => member.id === item.playbook.draft.responsibilities.escalation_member_id && member.status === 'active'));
        setSetupStep((Object.keys(missing) as (keyof typeof missing)[]).find((key) => missing[key].length) || 'review');
        setConfirm(undefined); setTab('setup');
      }}>Continue setup</QuietPrimaryAction> : <QuietPrimaryAction disabled={write.isPending || dirty || !canAdmin || confirmationRevision !== item.playbook.revision} onClick={() => void applyCommand()}>{write.isPending ? 'Saving…' : confirm === 'publish' ? 'Publish playbook' : confirm === 'start' ? 'Allow enrollment' : 'Stop enrollment'}</QuietPrimaryAction>}</DialogFooter>
    </DialogContent></Dialog>
  </div>;
}

function PlaybookActivity({ ws, id }: { ws: string; id: string }) {
  const history = useCRMPlaybookHistory(ws, id);
  const members = useAssignableMembers(ws);
  if (history.isPending) return <PlaybookLoading />;
  if (history.isError) return <PlaybookError error={history.error} retry={() => void history.refetch()} />;
  const changes = history.data.pages.flatMap((page) => page.data);
  return <div className="max-w-3xl py-4">
    {!changes.length && <QuietEmptyState title="No changes recorded" description="Saved drafts, publications, and enrollment changes appear here." />}
    <ol className="divide-y divide-quiet-divider-light">{changes.map((change) => <li key={change.id} className="py-4"><p className="text-sm font-medium">{change.operation === 'created' ? 'Draft created' : change.operation === 'update_draft' ? 'Draft updated' : change.operation === 'publish' ? 'Playbook published' : change.after.accepting_customers ? 'New enrollment allowed' : 'New enrollment stopped'}</p><p className="mt-1 text-xs text-quiet-text-tertiary">{members.data?.find((member) => member.id === change.actor_member_id)?.display_name || 'Workspace member'} · {new Date(change.created_at).toLocaleString()}</p>{change.reason && <p className="mt-2 text-sm text-quiet-text-secondary">{change.reason}</p>}<details className="mt-2 text-xs text-quiet-text-secondary"><summary className="cursor-pointer py-1">View saved settings</summary><p className="mt-2 font-medium">{change.after.draft.name}</p><p className="mt-1 leading-5">{change.after.draft.objective || 'No customer outcome set'}</p><ol className="mt-2 space-y-2">{change.after.draft.milestones?.map((milestone) => <li key={milestone.key}><span className="font-medium">{milestone.name}</span><p className="mt-1 leading-5">{milestone.success_criteria || 'No success criteria set'}</p></li>)}</ol></details></li>)}</ol>
    {history.hasNextPage && <QuietTextAction disabled={history.isFetchingNextPage} onClick={() => void history.fetchNextPage()}>{history.isFetchingNextPage ? 'Loading…' : 'Load earlier changes'}</QuietTextAction>}
  </div>;
}
