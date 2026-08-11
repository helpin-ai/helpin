import { createFileRoute } from '@tanstack/react-router';
import { AutomationRouteViewport } from '@/components/automation/AutomationRouteViewport';
import { SkillCatalogPage } from '@/pages/automation/SkillCatalog';

export const Route = createFileRoute('/_authenticated/w/$slug/automation/skills')({
  component: SkillCatalogRoute,
});

function SkillCatalogRoute() {
  return (
    <AutomationRouteViewport>
      <SkillCatalogPage />
    </AutomationRouteViewport>
  );
}
