import { StoriesPage } from '@/pages/pm/Stories';

interface StoryRouteFallbackBackgroundProps {
  teamId?: string;
}

export function StoryRouteFallbackBackground({
  teamId,
}: StoryRouteFallbackBackgroundProps) {
  return (
    <div className="h-full overflow-hidden pt-4 md:pt-6">
      <StoriesPage teamId={teamId} />
    </div>
  );
}
