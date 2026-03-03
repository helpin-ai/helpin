import { Fragment, useMemo } from 'react';
import { useLocation, useNavigate } from '@tanstack/react-router';
import { SidebarTrigger } from '@/components/ui/sidebar';
import { QuarterSelector } from '@/components/quarter/QuarterSelector';
import { useWorkspaceStore } from '@/stores/workspaceStore';

type Crumb = {
  label: string;
  to?: string;
};

export function Header() {
  const navigate = useNavigate();
  const location = useLocation();
  const { currentWorkspace } = useWorkspaceStore();

  const breadcrumbs = useMemo<Crumb[]>(() => {
    const segments = location.pathname.split('/').filter(Boolean);
    if (segments.length < 2 || segments[0] !== 'w') return [];

    const slug = segments[1];
    const subRoute = segments.slice(2);
    const workspaceLabel = currentWorkspace?.name ?? 'Workspace';

    const crumbs: Crumb[] = [{ label: workspaceLabel, to: `/w/${slug}/dashboard` }];
    if (subRoute.length === 0) {
      crumbs.push({ label: 'Dashboard' });
      return crumbs;
    }

    const section = subRoute[0];
    const sectionMap: Record<string, string> = {
      dashboard: 'Dashboard',
      goals: 'Company Goals',
      'team-goals': 'Team Goals',
      sprints: 'Sprints',
      bonus: 'Bonus Dashboard',
      'my-quarter': 'My Quarter',
      settings: 'Settings',
      tasks: 'Tasks',
    };

    if (section === 'sprints' && subRoute[1]) {
      crumbs.push({ label: 'Sprints', to: `/w/${slug}/sprints` });
      crumbs.push({ label: 'Sprint Detail' });
      return crumbs;
    }

    crumbs.push({ label: sectionMap[section] ?? section.replace(/-/g, ' ') });
    return crumbs;
  }, [location.pathname, currentWorkspace?.name]);

  return (
    <header className="h-14 border-b bg-background flex items-center px-4 gap-4">
      <SidebarTrigger className="-ml-1" />
      {breadcrumbs.length > 0 && (
        <nav aria-label="Breadcrumb" className="min-w-0 flex items-center gap-1 text-sm">
          {breadcrumbs.map((crumb, index) => {
            const isLast = index === breadcrumbs.length - 1;
            return (
              <Fragment key={`${crumb.label}-${index}`}>
                {index > 0 && <span className="text-muted-foreground">/</span>}
                {crumb.to && !isLast ? (
                  <button
                    type="button"
                    className="max-w-[14rem] truncate text-muted-foreground hover:text-foreground transition-colors"
                    onClick={() => navigate({ to: crumb.to as string })}
                  >
                    {crumb.label}
                  </button>
                ) : (
                  <span className="max-w-[14rem] truncate font-medium text-foreground">
                    {crumb.label}
                  </span>
                )}
              </Fragment>
            );
          })}
        </nav>
      )}
      <div className="flex-1" />
      <QuarterSelector />
    </header>
  );
}
