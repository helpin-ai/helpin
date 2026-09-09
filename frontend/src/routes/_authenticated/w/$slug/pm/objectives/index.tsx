import { createFileRoute } from '@tanstack/react-router';
import { ObjectivesPage } from '@/pages/pm/Objectives';

export const Route = createFileRoute('/_authenticated/w/$slug/pm/objectives/')({
  component: ObjectivesRoute,
});

function ObjectivesRoute() {
  const { slug } = Route.useParams();
  return <div className="h-full overflow-hidden"><ObjectivesPage key={slug} /></div>;
}
