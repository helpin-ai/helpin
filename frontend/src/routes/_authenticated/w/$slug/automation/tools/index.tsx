import { createFileRoute } from '@tanstack/react-router';
import { ToolCatalogPage } from '@/pages/automation/ToolCatalog';

export const Route = createFileRoute('/_authenticated/w/$slug/automation/tools/')({
  component: ToolCatalogRoute,
});

function ToolCatalogRoute() {
  return <ToolCatalogPage embedded />;
}
