import { createRoot } from 'react-dom/client';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import {
  createRootRoute,
  createRouter,
  createMemoryHistory,
  RouterProvider,
} from '@tanstack/react-router';
import '@/index.css';
import { TooltipProvider } from '@/components/ui/tooltip';
import { Toaster } from '@/components/ui/sonner';
import { SupportAIAssistantSettingsPage } from '@/pages/settings/SupportAIAssistantSettingsPage';
import { ConversationActionsMenu } from '@/components/support/ConversationActionsMenu';
import { useConversation } from '@/hooks/queries/useSupport';
import { useAuthStore } from '@/stores/authStore';
import { useWorkspaceStore } from '@/stores/workspaceStore';
import { Button } from '@/components/ui/button';
import { MoreHorizontalIcon } from '@/lib/icons';

useAuthStore.setState({
  user: {
    id: 'agent',
    full_name: 'Support teammate',
    email: 'agent@example.invalid',
  } as never,
  loading: false,
});
useWorkspaceStore
  .getState()
  .setCurrentWorkspace({ id: 'ws', name: 'Support', slug: 'support' } as never);
const client = new QueryClient({
  defaultOptions: { queries: { retry: false } },
});
function MenuHarness() {
  const { data: conversation } = useConversation('ws', 'conv');
  return conversation ? (
    <ConversationActionsMenu
      workspaceId="ws"
      conversation={conversation}
      trigger={
        <Button
          variant="ghost"
          size="icon"
          aria-label="Open conversation actions"
        >
          <MoreHorizontalIcon />
        </Button>
      }
    />
  ) : null;
}
const rootRoute = createRootRoute({
  component: () => (
    <QueryClientProvider client={client}>
      <TooltipProvider>
        <main className="mx-auto max-w-3xl space-y-4 p-4 text-foreground">
          {new URLSearchParams(window.location.search).has('menu') ? (
            <MenuHarness />
          ) : (
            <SupportAIAssistantSettingsPage />
          )}
        </main>
        <Toaster />
      </TooltipProvider>
    </QueryClientProvider>
  ),
});
const router = createRouter({
  routeTree: rootRoute,
  history: createMemoryHistory({ initialEntries: ['/'] }),
});
createRoot(document.getElementById('root')!).render(
  <RouterProvider router={router} />,
);
