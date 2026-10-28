import { useState } from 'react';
import { createRoot } from 'react-dom/client';
import { SupportRailNav } from '@/components/layout/sidebar/SupportRailNav';
import { SupportViewEditDialog } from '@/components/support/SupportViewEditDialog';
import { SidebarProvider, Sidebar, SidebarHeader, SidebarContent } from '@/components/ui/sidebar';
import { TooltipProvider } from '@/components/ui/tooltip';
import { Button } from '@/components/ui/button';
import { Switch } from '@/components/ui/switch';
import { Label } from '@/components/ui/label';
import { Toaster, toast } from 'sonner';
import { HelpinLogo } from '@/components/layout/HelpinLogo';
import type { SupportInboxView, SupportTag } from '@/lib/pmTypes';
import '@/index.css';
const initial: SupportInboxView = { id: 'view-1', workspace_id: 'demo', name: 'High priority', is_shared: true, view_type: 'custom', created_by: 'me', created_at: '', updated_at: '', filters: { nav_filter: 'inbox', states: 'open', priority: 'high,urgent' } };
const tags = [{ id: 'billing', name: 'Billing', color: '#8b5cf6' }, { id: 'bug', name: 'Bug', color: '#ef4444' }] as SupportTag[];
const mailboxes = [{ id: 'success', name: 'Customer success', unread_count: 0 }, { id: 'engineering', name: 'Engineering', unread_count: 0 }];
function Preview() {
  const [view, setView] = useState(initial);
  const [editing, setEditing] = useState<'edit' | 'create' | null>('create');
  const [slaEnabled, setSlaEnabled] = useState(true);
  const noop = () => {};
  return <TooltipProvider><SidebarProvider><Sidebar collapsible="none" className="border-r bg-[#f0f0f2]">
    <SidebarHeader className="p-4"><HelpinLogo /></SidebarHeader>
    <SidebarContent><SupportRailNav navFilter="inbox" selectedMailboxId="all" activeCustomViewId={view.id} currentUserId="me" customViews={[view]} canManageSettings wsSlug="demo" pathname="/w/demo/support" inboxScopes={{ mailboxes }} onNavFilterChange={noop} onMailboxSelect={noop} onCustomViewSelect={noop} onCreateCustomView={() => setEditing('create')} onEditCustomView={() => setEditing('edit')} onDeleteCustomView={() => toast('Preview only')} onCreateMailbox={noop} onEditMailbox={noop} onArchiveMailbox={noop} onNavigate={noop} /></SidebarContent>
  </Sidebar><main className="min-w-0 flex-1 bg-background">
    <header className="flex h-14 items-center justify-between border-b px-6"><h1 className="text-sm font-semibold">{view.name}</h1><span className="text-xs text-muted-foreground">Component preview</span></header>
    <div className="flex flex-wrap items-center gap-3 border-b px-6 py-4"><Label htmlFor="preview-sla">SLA enabled in settings</Label><Switch id="preview-sla" checked={slaEnabled} onCheckedChange={setSlaEnabled} /><Button variant="outline" size="sm" onClick={() => setEditing('edit')}>Edit view</Button></div>
    <div className="max-w-2xl p-6"><p className="text-sm text-muted-foreground">Use the view’s three-dot menu to edit its saved filters.</p>
      <div className="mt-6 divide-y rounded-lg border">{['Invoice payment failed', 'Unable to invite teammates', 'Account access issue'].map((subject, i) => <div className="flex items-center gap-3 p-4" key={subject}><div className="flex size-8 items-center justify-center rounded-full bg-muted text-xs text-muted-foreground">{['JL','SK','AM'][i]}</div><div><div className="text-sm font-medium">{subject}</div><div className="mt-1 text-xs text-muted-foreground">Sample conversation</div></div></div>)}</div>
    </div>
  </main>
  {editing && <SupportViewEditDialog key={editing === 'create' ? 'create' : view.id} view={editing === 'create' ? undefined : view} tags={tags} mailboxes={mailboxes} slaEnabled={slaEnabled} canManageSettings isPending={false} onClose={() => setEditing(null)} onSave={values => { setView(current => ({ ...current, ...values, id: editing === 'create' ? `preview-${Date.now()}` : current.id })); setEditing(null); toast.success(editing === 'create' ? 'View created' : 'View updated'); }} />}
  <Toaster /></SidebarProvider></TooltipProvider>;
}
createRoot(document.getElementById('root')!).render(<Preview />);
