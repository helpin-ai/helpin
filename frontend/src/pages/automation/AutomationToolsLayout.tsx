import { Outlet } from '@tanstack/react-router';
import { AutomationRouteViewport } from '@/components/automation/AutomationRouteViewport';
import { AutomationShell } from '@/components/automation/AutomationShell';

export function AutomationToolsLayout() {
  return (
    <AutomationRouteViewport>
      <AutomationShell
        title="Tools"
        description="Browse Helpin capabilities and tools from connected external MCP servers."
      >
        <Outlet />
      </AutomationShell>
    </AutomationRouteViewport>
  );
}
