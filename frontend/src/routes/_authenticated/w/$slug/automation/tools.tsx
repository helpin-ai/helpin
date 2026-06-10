import { createFileRoute } from '@tanstack/react-router';
import { ToolCatalogPage } from '@/pages/automation/ToolCatalog';

export const Route = createFileRoute('/_authenticated/w/$slug/automation/tools')({
  component: ToolCatalogRoute,
});

function ToolCatalogRoute() {
  return (
    <div className="h-full overflow-auto p-4 pb-20 md:p-6 md:pb-24">
      <ToolCatalogPage />
    </div>
  );
}
