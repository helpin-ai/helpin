import { createFileRoute } from '@tanstack/react-router';
import { MyWorkPage } from '@/pages/pm/MyWork';

export const Route = createFileRoute('/_authenticated/w/$slug/pm/my-work')({
  component: MyWorkRoute,
});

function MyWorkRoute() {
  return (
    <div className="h-full overflow-auto p-4 md:p-6">
      <MyWorkPage />
    </div>
  );
}
