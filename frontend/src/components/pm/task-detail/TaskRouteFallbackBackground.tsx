import { StoriesPage } from '@/pages/pm/Stories';

interface TaskRouteFallbackBackgroundProps {
  teamId?: string;
}

export function TaskRouteFallbackBackground({
  teamId,
}: TaskRouteFallbackBackgroundProps) {
  return (
    <div className="h-full overflow-hidden pt-4 md:pt-6">
      <StoriesPage teamId={teamId} />
    </div>
  );
}
