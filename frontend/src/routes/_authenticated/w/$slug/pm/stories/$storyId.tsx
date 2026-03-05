import { createFileRoute } from '@tanstack/react-router';
import { StoryDetailPage } from '@/pages/pm/StoryDetail';

export const Route = createFileRoute('/_authenticated/w/$slug/pm/stories/$storyId')({
  component: () => (
    <div className="h-full overflow-hidden">
      <StoryDetailPage />
    </div>
  ),
});
