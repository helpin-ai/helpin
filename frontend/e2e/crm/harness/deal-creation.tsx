import { useState } from 'react';
import { createRoot } from 'react-dom/client';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { createMemoryHistory, createRootRoute, createRoute, createRouter, Outlet, RouterProvider } from '@tanstack/react-router';
import { TooltipProvider } from '@/components/ui/tooltip';
import { Toaster } from '@/components/ui/sonner';
import { CreateDealDialog } from '@/components/crm/CreateDealDialog';
import { DealsPage } from '@/pages/crm/Deals';
import { useWorkspaceStore } from '@/stores/workspaceStore';
import { useDealPanelStore } from '@/stores/dealPanelStore';
import { useAuthStore } from '@/stores/authStore';
import type { Workspace, User } from '@/lib/types';
import '@/index.css';
useWorkspaceStore.setState({ currentWorkspace: { id:'ws-deals',slug:'deals-test',name:'Deals test' } as Workspace });
useAuthStore.setState({ user: {id:'user-1'} as User });
const params = new URLSearchParams(window.location.search);
function FormHarness() {
 const [open,setOpen] = useState(true);
 const openedDeal = useDealPanelStore(s => s.dealId);
 return <>{openedDeal && <output>Opened deal {openedDeal}</output>}<button onClick={() => setOpen(true)}>Reopen</button><CreateDealDialog open={open} onOpenChange={setOpen} initialPipelineId={params.has('context') ? 'sales' : undefined} initialStageId={params.has('context') ? 'proposal' : undefined} /></>;
}
const root = createRootRoute({component: () => <div className="h-screen bg-background text-foreground"><Outlet /></div>});
const route = createRoute({getParentRoute: () => root,path:'/w/$slug/crm/deals',component: params.has('page') ? DealsPage : FormHarness});
const router = createRouter({routeTree:root.addChildren([route]),history:createMemoryHistory({initialEntries:['/w/deals-test/crm/deals']})});
const client = new QueryClient({defaultOptions:{queries:{retry:false},mutations:{retry:false}}});
createRoot(document.getElementById('root')!).render(<QueryClientProvider client={client}><TooltipProvider><RouterProvider router={router}/><Toaster/></TooltipProvider></QueryClientProvider>);
