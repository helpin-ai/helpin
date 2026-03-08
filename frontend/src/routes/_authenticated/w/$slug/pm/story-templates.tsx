import { createFileRoute } from '@tanstack/react-router';
import { StoryTemplatesPage } from '@/pages/pm/StoryTemplates';

export const Route = createFileRoute('/_authenticated/w/$slug/pm/story-templates')({
  component: StoryTemplatesRoute,
});

function StoryTemplatesRoute() {
  return (
    <div className="h-full overflow-auto p-4 md:p-6">
      <StoryTemplatesPage />
    </div>
  );
}
