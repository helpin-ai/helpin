import { createRoot } from 'react-dom/client'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import '@/index.css'
import { MessageBubble } from '@/components/support/MessageBubble'
import { TooltipProvider } from '@/components/ui/tooltip'
import type { SupportMessage } from '@/lib/pmTypes'

const messages: SupportMessage[] = [true, false].map((helpful, index) => ({
  id: `answer-${index}`, workspace_id: 'ws-1', conversation_id: 'conv-1',
  sender_type: 'ai', message_type: 'reply', sender_display_name: 'Helpin AI',
  content: index
    ? 'You can export the report from the workspace settings. Choose the date range, then download the export.'
    : 'Yes, that is supported.',
  is_internal: false, via_channel: 'widget',
  created_at: '2026-09-13T12:00:00Z', updated_at: '2026-09-13T12:00:00Z',
  metadata: JSON.stringify({ visitor_feedback: { helpful, submitted_at: '2026-09-13T12:01:00Z' } }),
}))

createRoot(document.getElementById('root')!).render(
  <QueryClientProvider client={new QueryClient({ defaultOptions: { queries: { enabled: false } } })}>
    <TooltipProvider>
      <main className="mx-auto flex max-w-3xl flex-col gap-3 p-4">
        {messages.map(message => <MessageBubble key={message.id} message={message} source="widget" receiptStatus="delivered" />)}
      </main>
    </TooltipProvider>
  </QueryClientProvider>,
)
