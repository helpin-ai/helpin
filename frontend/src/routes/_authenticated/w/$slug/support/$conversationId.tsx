import { createFileRoute } from '@tanstack/react-router';
import { SupportPage } from '@/pages/pm/Support';
import { normalizeSupportInboxRouteSearch, type SupportInboxRouteSearch } from '@/lib/supportInboxRouting';

export const Route = createFileRoute('/_authenticated/w/$slug/support/$conversationId')({
  component: SupportConversationRoute,
  validateSearch: (search: Record<string, unknown>): SupportInboxRouteSearch =>
    normalizeSupportInboxRouteSearch(search),
});

function SupportConversationRoute() {
  return (
    <div className="h-full overflow-hidden">
      <SupportPage />
    </div>
  );
}
