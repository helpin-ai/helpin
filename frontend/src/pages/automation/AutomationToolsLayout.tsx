import { Link, Outlet, useLocation, useParams } from '@tanstack/react-router';
import { AutomationShell } from '@/components/automation/AutomationShell';
import { buildAutomationToolConnectionsPath, buildAutomationToolsPath } from '@/lib/automationUi';
import { cn } from '@/lib/utils';

export function AutomationToolsLayout() {
  const { slug } = useParams({ from: '/_authenticated/w/$slug/automation/tools' });
  const pathname = useLocation({ select: (location) => location.pathname });
  const connectionsPath = buildAutomationToolConnectionsPath(slug);
  const catalogPath = buildAutomationToolsPath(slug);
  const connectionsActive = pathname === connectionsPath;

  return (
    <div className="h-full overflow-auto p-4 pb-20 md:p-6 md:pb-24">
      <AutomationShell
        title="Tools"
        description="Browse Helpin capabilities and connect external systems for your agents."
        className="max-w-5xl"
      >
        <nav aria-label="Tool sections" className="border-b border-border/70">
          <div className="flex items-center gap-6">
            <Link
              to={catalogPath}
              aria-current={!connectionsActive ? 'page' : undefined}
              className={cn(
                'relative -mb-px py-2.5 text-sm font-medium transition-colors',
                !connectionsActive
                  ? 'text-foreground after:absolute after:inset-x-0 after:bottom-0 after:h-0.5 after:bg-foreground'
                  : 'text-muted-foreground hover:text-foreground',
              )}
            >
              Catalog
            </Link>
            <Link
              to={connectionsPath}
              aria-current={connectionsActive ? 'page' : undefined}
              className={cn(
                'relative -mb-px py-2.5 text-sm font-medium transition-colors',
                connectionsActive
                  ? 'text-foreground after:absolute after:inset-x-0 after:bottom-0 after:h-0.5 after:bg-foreground'
                  : 'text-muted-foreground hover:text-foreground',
              )}
            >
              Connections
            </Link>
          </div>
        </nav>
        <Outlet />
      </AutomationShell>
    </div>
  );
}
