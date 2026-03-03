import { Fragment, useMemo } from 'react';
import { useLocation, useNavigate } from '@tanstack/react-router';
import { Bell, CircleHelp, Search } from 'lucide-react';
import { SidebarTrigger } from '@/components/ui/sidebar';
import { QuarterSelector } from '@/components/quarter/QuarterSelector';
import { useWorkspaceStore } from '@/stores/workspaceStore';
import { Input } from '@/components/ui/input';
import { Button } from '@/components/ui/button';

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
    <header className="h-14 border-b border-border/70 bg-background/95 px-3 flex items-center gap-3">
      <div className="flex min-w-0 items-center gap-2">
        <SidebarTrigger className="-ml-1" />
        {breadcrumbs.length > 0 && (
          <nav aria-label="Breadcrumb" className="hidden min-w-0 items-center gap-1 text-sm md:flex">
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
      </div>

      <div className="hidden lg:flex flex-1 max-w-xl items-center">
        <div className="relative w-full">
          <Search className="pointer-events-none absolute left-2.5 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
          <Input
            aria-label="Search workspace"
            placeholder={`Search ${currentWorkspace?.name ?? 'workspace'}...`}
            className="h-8 pl-8 bg-muted/40 border-border/70"
          />
        </div>
      </div>

      <div className="ml-auto flex items-center gap-1">
        <Button variant="ghost" size="icon" className="size-8 text-muted-foreground">
          <Bell className="h-4 w-4" />
        </Button>
        <Button variant="ghost" size="icon" className="size-8 text-muted-foreground">
          <CircleHelp className="h-4 w-4" />
        </Button>
        <QuarterSelector />
      </div>
    </header>
  );
}
