import { createFileRoute } from '@tanstack/react-router';
import { CodingSessionPage } from '@/pages/pm/CodingSession';

export const Route = createFileRoute('/_authenticated/w/$slug/pm/coding-sessions/$sessionId')({
  component: CodingSessionRoute,
});

function CodingSessionRoute() {
  const { sessionId } = Route.useParams();
  return <CodingSessionPage sessionId={sessionId} />;
}
