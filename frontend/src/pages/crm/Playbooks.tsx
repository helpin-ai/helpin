import { useDeferredValue, useState } from 'react';
import { Link, useNavigate } from '@tanstack/react-router';
import { QuietEmptyState, QuietPageHeader, QuietPageViewport, QuietPrimaryAction, QuietSearchInput, QuietStatusText, QuietTextAction } from '@/components/design-system/quiet';
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table';
import { Dialog, DialogContent, DialogDescription, DialogHeader, DialogTitle } from '@/components/ui/dialog';
import { useCRMPlaybooks, useCRMPlaybookTemplates, useCRMPlaybookWrite } from '@/hooks/queries/useCRMPlaybooks';
import { usePermissions, useWorkspaceAccess } from '@/hooks/queries/useSession';
import { useWorkspaceStore } from '@/stores/workspaceStore';
import { useTitle } from '@/hooks/useTitle';
import { blankPlaybook, createPlaybookIntentKey, playbookStatus } from '@/lib/crmPlaybookPresentation';
import type { CRMPlaybookDefinition } from '@/lib/crmPlaybookTypes';
import type { PlaybookListFilters } from '@/lib/services/crmPlaybookService';
import { PlaybookError, PlaybookHelp, PlaybookLoading, PlaybookNoAccess, PlaybookPagination, PlaybookSelect } from '@/components/crm/playbooks/PlaybookUI';

export function PlaybooksPage() {
  useTitle('Playbooks');
  const workspace = useWorkspaceStore((state) => state.currentWorkspace);
  const access = useWorkspaceAccess(workspace?.id || '');
  const { has } = usePermissions(access.data);
  if (access.isPending) return <QuietPageViewport><PlaybookLoading /></QuietPageViewport>;
  if (access.isError) return <QuietPageViewport><PlaybookError error={access.error} retry={() => void access.refetch()} /></QuietPageViewport>;
  if (!workspace || !has('crm.read')) return <QuietPageViewport><PlaybookNoAccess /></QuietPageViewport>;
  return <PlaybooksList key={workspace.id} ws={workspace.id} slug={workspace.slug} canAdmin={has('crm.admin')} />;
}

function PlaybooksList({ ws, slug, canAdmin }: { ws: string; slug: string; canAdmin: boolean }) {
  const [search, setSearch] = useState('');
  const [state, setState] = useState<PlaybookListFilters['state']>('all');
  const [page, setPage] = useState(1);
  const [creating, setCreating] = useState(false);
  const query = useDeferredValue(search.trim());
  const list = useCRMPlaybooks(ws, { q: query, state, page });
  const navigate = useNavigate();
  const filtered = !!search || state !== 'all';
  const clear = () => { setSearch(''); setState('all'); setPage(1); };
  return <QuietPageViewport>
    <QuietPageHeader title="Playbooks" description="Repeatable sales and success processes, with clear ownership and outcomes." actions={canAdmin && <QuietPrimaryAction onClick={() => setCreating(true)}>Create playbook</QuietPrimaryAction>} />
    <div className="mt-6 flex flex-wrap items-center gap-3 border-b border-quiet-divider-strong pb-3">
      <QuietSearchInput aria-label="Search playbooks" placeholder="Search playbooks…" value={search} onChange={(event) => { setSearch(event.target.value); setPage(1); }} containerClassName="w-64 max-w-full" />
      <PlaybookSelect label="Playbook status" value={state || 'all'} onChange={(value) => { setState(value as PlaybookListFilters['state']); setPage(1); }} options={[{ value: 'all', label: 'All statuses' }, { value: 'draft', label: 'Draft' }, { value: 'accepting', label: 'Accepting signals' }, { value: 'stopped', label: 'Enrollment stopped' }]} />
      {filtered && <QuietTextAction onClick={clear}>Clear filters</QuietTextAction>}
    </div>
    {list.isPending ? <PlaybookLoading /> : list.isError ? <PlaybookError error={list.error} retry={() => void list.refetch()} /> : <>
      {list.data.data.length ? <Table aria-label="Playbooks" aria-busy={list.isFetching || search.trim() !== query}>
        <TableHeader><TableRow><TableHead>Playbook</TableHead><TableHead>Status</TableHead><TableHead><span className="inline-flex items-center">Open signals<PlaybookHelp label="About signal counts">Counts are customer objectives, not unique customers. One customer can have more than one objective.</PlaybookHelp></span></TableHead><TableHead>Paused</TableHead><TableHead>Closed</TableHead></TableRow></TableHeader>
        <TableBody>{list.data.data.map((item) => <TableRow key={item.playbook.id}>
          <TableCell className="min-w-64 max-w-xl whitespace-normal"><Link to="/w/$slug/crm/playbooks/$playbookId" params={{ slug, playbookId: item.playbook.id }} className="font-semibold text-quiet-text-primary underline-offset-4 hover:underline focus-visible:outline-2 focus-visible:outline-quiet-field">{item.playbook.draft.name}</Link><p className="mt-1 line-clamp-2 text-xs leading-5 text-quiet-text-tertiary">{item.playbook.draft.objective || 'Customer outcome not set'}</p></TableCell>
          <TableCell><QuietStatusText tone={item.playbook.accepting_customers ? 'positive' : 'neutral'}>{playbookStatus(item)}</QuietStatusText></TableCell>
          <TableCell className="tabular-nums">{item.open_count}</TableCell><TableCell className="tabular-nums text-quiet-text-secondary">{item.paused_count}</TableCell><TableCell className="tabular-nums text-quiet-text-secondary">{item.closed_count}</TableCell>
        </TableRow>)}</TableBody>
      </Table> : <QuietEmptyState title={filtered ? 'No matching playbooks' : page > 1 ? 'No playbooks on this page' : 'Give your team a repeatable way forward'} description={filtered ? 'Try another name or status.' : 'Define the customer outcome, milestones, and who steps in when help is needed.'} action={filtered ? <QuietTextAction onClick={clear}>Clear filters</QuietTextAction> : undefined} />}
      <PlaybookPagination page={page} total={list.data.total} pageSize={list.data.page_size} onChange={setPage} busy={list.isFetching} />
    </>}
    {creating && <CreatePlaybook ws={ws} onClose={() => setCreating(false)} onCreated={(id) => { setCreating(false); void navigate({ to: '/w/$slug/crm/playbooks/$playbookId', params: { slug, playbookId: id } }); }} />}
  </QuietPageViewport>;
}

function CreatePlaybook({ ws, onClose, onCreated }: { ws: string; onClose: () => void; onCreated: (id: string) => void }) {
  const templates = useCRMPlaybookTemplates(ws);
  const write = useCRMPlaybookWrite(ws);
  const [intentKey] = useState(createPlaybookIntentKey);
  const create = async (definition: CRMPlaybookDefinition) => {
    try {
      const result = await write.mutateAsync({ kind: 'create', body: { creation_key: intentKey(definition), definition } });
      if ('id' in result) onCreated(result.id);
    } catch { /* Retain the dialog and retry key. */ }
  };
  return <Dialog open onOpenChange={(open) => { if (!open && !write.isPending) onClose(); }}><DialogContent>
    <DialogHeader><DialogTitle>Create playbook</DialogTitle><DialogDescription>Start with a familiar process or define your own. Nothing runs when you create a draft.</DialogDescription></DialogHeader>
    {templates.isPending ? <PlaybookLoading /> : templates.isError ? <PlaybookError error={templates.error} retry={() => void templates.refetch()} /> : <div className="divide-y divide-quiet-divider-light">{templates.data.map((definition) => <button key={definition.journey} type="button" disabled={write.isPending} onClick={() => void create(definition)} className="w-full py-4 text-left hover:bg-quiet-row-hover focus-visible:outline-2 focus-visible:outline-quiet-field disabled:opacity-50"><span className="block text-sm font-semibold">{definition.name}</span><span className="mt-1 block text-xs leading-5 text-quiet-text-tertiary">{definition.objective}</span></button>)}</div>}
    {write.isError && <PlaybookError error={write.error} />}
    <QuietTextAction disabled={write.isPending} onClick={() => void create(blankPlaybook())}>{write.isPending ? 'Creating…' : 'Start from scratch'}</QuietTextAction>
  </DialogContent></Dialog>;
}
