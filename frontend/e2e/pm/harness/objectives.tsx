import { createRoot } from 'react-dom/client';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { createMemoryHistory, createRootRoute, createRoute, createRouter, Outlet, RouterProvider } from '@tanstack/react-router';
import { ObjectivesPage } from '@/pages/pm/Objectives';
import { GlobalCreateModals } from '@/components/pm/GlobalCreateModals';
import { ConfirmProvider } from '@/components/ui/confirm-dialog';
import { TooltipProvider } from '@/components/ui/tooltip';
import { useWorkspaceStore } from '@/stores/workspaceStore';
import type { Workspace } from '@/lib/types';
import '@/index.css';

if (new URLSearchParams(location.search).get('theme') === 'dark') document.documentElement.classList.add('dark');
useWorkspaceStore.setState({ currentWorkspace: { id: 'ws', slug: 'acme', name: 'Acme' } as Workspace });
const root = createRootRoute({ component: () => <div className="h-dvh bg-quiet-surface text-quiet-text-primary"><Outlet /><GlobalCreateModals workspaceId="ws" /></div> });
const index = createRoute({ getParentRoute: () => root, path: '/w/$slug/pm/objectives', component: ObjectivesPage });
const detail = createRoute({ getParentRoute: () => root, path: '/w/$slug/pm/objectives/$objectiveId', component: () => <h1>Objective detail destination</h1> });
const router = createRouter({ routeTree: root.addChildren([index, detail]), history: createMemoryHistory({ initialEntries: ['/w/acme/pm/objectives'] }) });
const client = new QueryClient({ defaultOptions: { queries: { retry: false }, mutations: { retry: false } } });
createRoot(document.getElementById('root')!).render(<QueryClientProvider client={client}><ConfirmProvider><TooltipProvider><RouterProvider router={router} /></TooltipProvider></ConfirmProvider></QueryClientProvider>);
