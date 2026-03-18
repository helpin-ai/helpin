import { createFileRoute } from '@tanstack/react-router';
import { SupportPage } from '@/pages/pm/Support';

export const Route = createFileRoute('/_authenticated/w/$slug/support/$conversationId')({
  component: SupportConversationRoute,
});

function SupportConversationRoute() {
  return (
    <div className="h-full overflow-hidden">
      <SupportPage />
    </div>
  );
}
