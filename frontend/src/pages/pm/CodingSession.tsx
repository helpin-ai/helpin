import { CodingSessionSurface } from '@/components/pm/CodingSession/CodingSessionSurface';
import { useTitle } from '@/hooks/useTitle';

export function CodingSessionPage({ sessionId }: { sessionId: string }) {
  useTitle('Agent Session');
  return <CodingSessionSurface sessionId={sessionId} />;
}
