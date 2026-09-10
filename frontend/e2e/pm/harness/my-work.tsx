import { createRoot } from 'react-dom/client';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { createMemoryHistory, createRootRoute, createRoute, createRouter, Outlet, RouterProvider } from '@tanstack/react-router';
import { TooltipProvider } from '@/components/ui/tooltip';
import { Toaster } from '@/components/ui/sonner';
import { MyWorkPage } from '@/pages/pm/MyWork';
import { useWorkspaceStore } from '@/stores/workspaceStore';
import type { Workspace } from '@/lib/types';
import '@/index.css';

useWorkspaceStore.setState({ currentWorkspace: { id: 'ws-1', slug: 'acme', name: 'Acme' } as Workspace });
if (new URLSearchParams(location.search).get('theme') === 'dark') document.documentElement.classList.add('dark');
const root = createRootRoute({ component: () => <div className="h-screen bg-quiet-surface text-quiet-text-primary"><Outlet /></div> });
const work = createRoute({ getParentRoute: () => root, path: '/w/$slug/pm/my-work', component: MyWorkPage });
const router = createRouter({ routeTree: root.addChildren([work]), history: createMemoryHistory({ initialEntries: ['/w/acme/pm/my-work'] }) });
const client = new QueryClient({ defaultOptions: { queries: { retry: false }, mutations: { retry: false } } });
createRoot(document.getElementById('root')!).render(<QueryClientProvider client={client}><TooltipProvider><RouterProvider router={router} /><Toaster /></TooltipProvider></QueryClientProvider>);
