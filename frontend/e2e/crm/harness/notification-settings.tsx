import { createRoot } from 'react-dom/client';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { TooltipProvider } from '@/components/ui/tooltip';
import NotificationSettings from '@/pages/NotificationSettings';
import { useWorkspaceStore } from '@/stores/workspaceStore';
import '@/index.css';
useWorkspaceStore.setState({ currentWorkspace: { id: 'ws-settings', slug: 'usermaven', name: 'Usermaven' } });
if (location.search.includes('dark')) document.documentElement.classList.add('dark');
createRoot(document.getElementById('root')!).render(
  <QueryClientProvider client={new QueryClient({ defaultOptions: { queries: { retry: false } } })}>
    <TooltipProvider><main className="mx-auto max-w-3xl bg-background p-5 text-foreground sm:p-8"><NotificationSettings /></main></TooltipProvider>
  </QueryClientProvider>,
);
