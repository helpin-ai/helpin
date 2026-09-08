import { createRoot } from 'react-dom/client';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { createMemoryHistory, createRootRoute, createRoute, createRouter, Outlet, RouterProvider } from '@tanstack/react-router';
import { TooltipProvider } from '@/components/ui/tooltip';
import { Toaster } from '@/components/ui/sonner';
import { PlaybooksPage } from '@/pages/crm/Playbooks';
import { PlaybookDetailPage } from '@/pages/crm/PlaybookDetail';
import { SignalsPage } from '@/pages/crm/Signals';
import { parseSignalsSearch } from '@/lib/crmSignalInboxQueryBuilder';
import { crmReviewRouteOptions } from '@/lib/crmReviewRoute';
import { useWorkspaceStore } from '@/stores/workspaceStore';
import type { Workspace } from '@/lib/types';
import '@/index.css';

// Render production pages and transport through the production service/query layer.
// All API traffic is intercepted by Playwright; no app API or worker is needed.
useWorkspaceStore.setState({ currentWorkspace: { id: 'ws-playbooks', slug: 'playbooks-test', name: 'Playbooks test' } as Workspace });
const params = new URLSearchParams(window.location.search);
if (params.get('theme') === 'dark') document.documentElement.classList.add('dark');
const root = createRootRoute({ component: () => <div className="h-screen bg-quiet-surface text-quiet-text-primary"><Outlet /></div> });
const list = createRoute({ getParentRoute: () => root, path: '/w/$slug/crm/playbooks', component: PlaybooksPage });
const detail = createRoute({ getParentRoute: () => root, path: '/w/$slug/crm/playbooks/$playbookId', component: () => <PlaybookDetailPage playbookId={detail.useParams().playbookId} /> });
const signals = createRoute({ getParentRoute: () => root, path: '/w/$slug/crm/insights', validateSearch: parseSignalsSearch, component: () => <SignalsPage search={signals.useSearch()} onChange={(search, replace) => void router.navigate({ to: '/w/$slug/crm/insights', search, replace })} /> });
const review = createRoute({ getParentRoute: () => root, path: '/w/$slug/crm/review', ...crmReviewRouteOptions });
const initial = params.has('review') ? `/w/playbooks-test/crm/review?${params}` : params.has('signals') ? `/w/playbooks-test/crm/insights?${params}` : `/w/playbooks-test/crm/playbooks${params.has('detail') ? '/book-1' : ''}`;
const router = createRouter({ routeTree: root.addChildren([list, detail, signals, review]), history: createMemoryHistory({ initialEntries: [initial] }) });
const client = new QueryClient({ defaultOptions: { queries: { retry: false }, mutations: { retry: false } } });
createRoot(document.getElementById('root')!).render(<QueryClientProvider client={client}><TooltipProvider><RouterProvider router={router} /><Toaster /></TooltipProvider></QueryClientProvider>);
