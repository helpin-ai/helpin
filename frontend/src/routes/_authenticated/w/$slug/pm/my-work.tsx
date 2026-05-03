import { createFileRoute } from '@tanstack/react-router';
import { MyWorkPage } from '@/pages/pm/MyWork';

type MyWorkSearch = { task?: string; run?: string };

export const Route = createFileRoute('/_authenticated/w/$slug/pm/my-work')({
  component: MyWorkRoute,
  validateSearch: (search: Record<string, unknown>): MyWorkSearch => ({
    task: search.task != null ? String(search.task) : undefined,
    run: search.run != null ? String(search.run) : undefined,
  }),
});

function MyWorkRoute() {
  return (
    <div className="h-full overflow-auto p-4 md:p-6">
      <MyWorkPage />
    </div>
  );
}
