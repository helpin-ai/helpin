import { createFileRoute } from '@tanstack/react-router';
import { StoriesPage } from '@/pages/pm/Stories';

export const Route = createFileRoute('/_authenticated/w/$slug/pm/stories')({
  component: StoriesPage,
});
