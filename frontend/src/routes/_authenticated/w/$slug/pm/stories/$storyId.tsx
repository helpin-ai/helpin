import { createFileRoute } from '@tanstack/react-router';
import { StoryRouteFallbackBackground } from '@/components/pm/story-detail/StoryRouteFallbackBackground';

export const Route = createFileRoute('/_authenticated/w/$slug/pm/stories/$storyId')({
  validateSearch: (search: Record<string, unknown>) => ({
    team: typeof search.team === 'string' ? search.team : undefined,
  }),
  component: StoryRouteComponent,
});

function StoryRouteComponent() {
  const { team } = Route.useSearch();
  return <StoryRouteFallbackBackground teamId={team} />;
}
