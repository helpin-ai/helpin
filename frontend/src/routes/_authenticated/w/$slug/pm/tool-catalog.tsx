import { createFileRoute } from '@tanstack/react-router';
import { ToolCatalogPage } from '@/pages/pm/ToolCatalog';

export const Route = createFileRoute('/_authenticated/w/$slug/pm/tool-catalog')({
  component: ToolCatalogRoute,
});

function ToolCatalogRoute() {
  return (
    <div className="h-full overflow-auto p-4 md:p-6">
      <ToolCatalogPage />
    </div>
  );
}
