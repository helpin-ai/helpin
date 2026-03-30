import { createFileRoute, Navigate } from '@tanstack/react-router';

export const Route = createFileRoute('/_authenticated/w/$slug/settings/chat-ai')({
  component: ChatAIRoute,
});

function ChatAIRoute() {
  const { slug } = Route.useParams();
  return <Navigate to="/w/$slug/settings/chat-general" params={{ slug }} replace />;
}
