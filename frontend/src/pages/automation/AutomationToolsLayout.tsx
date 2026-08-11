import { Outlet } from '@tanstack/react-router';
import { AutomationShell } from '@/components/automation/AutomationShell';

export function AutomationToolsLayout() {
  return (
    <div className="h-full overflow-auto p-4 pb-20 md:p-6 md:pb-24">
      <AutomationShell
        title="Tools"
        description="Browse Helpin capabilities and tools from connected external MCP servers."
        className="max-w-5xl"
      >
        <Outlet />
      </AutomationShell>
    </div>
  );
}
