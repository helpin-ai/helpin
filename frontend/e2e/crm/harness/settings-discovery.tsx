import { useState } from 'react';
import { createRoot } from 'react-dom/client';
import { createRootRoute, createRoute, createRouter, RouterProvider, Outlet, useLocation } from '@tanstack/react-router';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { TooltipProvider } from '@/components/ui/tooltip';
import { SidebarProvider } from '@/components/ui/sidebar';
import { SettingsRailNav } from '@/components/layout/sidebar/SettingsRailNav';
import { getCollapsedSettingsGroups } from '@/components/layout/sidebar/state';
import { buildPanelNavGroups } from '@/components/layout/sidebar/config';
import { SettingsHomePage } from '@/pages/settings/SettingsHomePage';
import { SettingsRouteViewport } from '@/pages/settings/SettingsRouteViewport';
import { CRMEmailSettingsTab } from '@/components/settings/CRMEmailSettingsTab';
import { useWorkspaceStore } from '@/stores/workspaceStore';
import { useAuthStore } from '@/stores/authStore';
import '@/index.css';
useWorkspaceStore.setState({ currentWorkspace: { id: 'ws-settings', slug: 'settings-test', name: 'Acme' } });
useAuthStore.setState({ user: { id: 'owner' } as never });
if (new URLSearchParams(location.search).has('dark')) document.documentElement.classList.add('dark');
const permissions = new Set(['crm.read', 'crm.edit', 'settings.read', 'workspace.read', 'module_access.manage']);
function Shell() {
  const location = useLocation();
  const [collapsed, setCollapsed] = useState(getCollapsedSettingsGroups);
  return <SidebarProvider><div className="flex min-h-screen w-full bg-background text-foreground">
    <aside className="hidden w-60 shrink-0 bg-sidebar p-4 md:block"><SettingsRailNav workspaceSlug="settings-test" groups={buildPanelNavGroups('settings-test', true, permissions).settings} isActive={href => href.replace(/\/$/, '') === location.pathname.replace(/\/$/, '')} collapsedGroups={collapsed} toggleGroup={group => setCollapsed(previous => { const next = new Set(previous); if (next.has(group)) next.delete(group); else next.add(group); return next; })} onNavigate={href => void router.navigate({ to: href })} /></aside>
    <main className="min-w-0 flex-1 p-5 sm:p-8"><Outlet /></main>
  </div></SidebarProvider>;
}
function Home() { return <SettingsRouteViewport><SettingsHomePage autoFocus={new URLSearchParams(location.search).get('search') === '1'} /></SettingsRouteViewport>; }
const root = createRootRoute({ component: Shell });
const harness = createRoute({ getParentRoute: () => root, path: '/e2e/crm/harness/settings-discovery.html', component: Home });
const home = createRoute({ getParentRoute: () => root, path: '/w/$slug/settings/', component: Home });
const email = createRoute({ getParentRoute: () => root, path: '/w/$slug/settings/crm-email', component: () => <SettingsRouteViewport><CRMEmailSettingsTab workspaceId="ws-settings" /></SettingsRouteViewport> });
const router = createRouter({ routeTree: root.addChildren([harness, home, email]) });
createRoot(document.getElementById('root')!).render(<QueryClientProvider client={new QueryClient({ defaultOptions: { queries: { retry: false } } })}><TooltipProvider><RouterProvider router={router} /></TooltipProvider></QueryClientProvider>);
