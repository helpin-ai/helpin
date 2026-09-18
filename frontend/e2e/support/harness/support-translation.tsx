import { createRoot } from 'react-dom/client';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import '@/index.css';
import { TooltipProvider } from '@/components/ui/tooltip';
import { Toaster } from '@/components/ui/sonner';
import { TranslatedMessageBubble } from '@/components/support/TranslatedMessageBubble';
import { SupportTranslationSettings } from '@/components/settings/SupportTranslationSettings';
import {
  createRootRoute,
  createRouter,
  createMemoryHistory,
  RouterProvider,
} from '@tanstack/react-router';
import { ReplyComposer } from '@/components/support/ReplyComposer';
import { useAuthStore } from '@/stores/authStore';
import { useWorkspaceStore } from '@/stores/workspaceStore';
import { useSupportInboxStore } from '@/stores/supportInboxStore';
import type { SupportMessage } from '@/lib/pmTypes';

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
useSupportInboxStore.setState({
  replyMode: 'reply',
  drafts: { conv: 'Hello, we can help.' },
});
const message: SupportMessage = {
  id: 'incoming',
  workspace_id: 'ws',
  conversation_id: 'conv',
  sender_type: 'customer',
  sender_display_name: 'Customer',
  message_type: 'reply',
  content: new URLSearchParams(window.location.search).has('english')
    ? 'Hello, I need help.'
    : 'Hallo, ich brauche Hilfe.',
  is_internal: false,
  created_at: '2026-09-18T12:00:00Z',
  updated_at: '2026-09-18T12:00:00Z',
};
const client = new QueryClient({
  defaultOptions: { queries: { retry: false } },
});
const showSettings = new URLSearchParams(window.location.search).has(
  'settings',
);
const editable = !new URLSearchParams(window.location.search).has('readonly');
const rootRoute = createRootRoute({
  component: () => (
    <QueryClientProvider client={client}>
      <TooltipProvider>
        <main className="mx-auto max-w-3xl space-y-4 p-3">
          {showSettings && (
            <section aria-label="Workspace translation settings">
              <SupportTranslationSettings
                workspaceId="ws"
                editable={editable}
              />
            </section>
          )}
          <TranslatedMessageBubble
            workspaceId="ws"
            message={message}
            source="widget"
          />
          <ReplyComposer workspaceId="ws" conversationId="conv" />
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
