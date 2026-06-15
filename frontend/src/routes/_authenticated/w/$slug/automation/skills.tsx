import { createFileRoute } from '@tanstack/react-router';
import { SkillCatalogPage } from '@/pages/automation/SkillCatalog';

export const Route = createFileRoute('/_authenticated/w/$slug/automation/skills')({
  component: SkillCatalogRoute,
});

function SkillCatalogRoute() {
  return (
    <div className="h-full overflow-auto p-4 pb-20 md:p-6 md:pb-24">
      <SkillCatalogPage />
    </div>
  );
}
