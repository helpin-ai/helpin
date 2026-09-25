import { renderToStaticMarkup } from 'react-dom/server';
import { describe, expect, it } from 'vitest';
import { TooltipProvider } from '@/components/ui/tooltip';
import { SidebarRail } from '../SidebarRail';

describe('SidebarRail Ask Agents entry', () => {
  const render = (canUseAskAgents: boolean) => renderToStaticMarkup(
    <TooltipProvider>
      <SidebarRail
        railItems={[]}
        activeRail="support"
        accessibleModules={['support']}
        canUseAskAgents={canUseAskAgents}
        onRailSelect={() => {}}
        onToggleTheme={() => {}}
        accountMenu={null}
      />
    </TooltipProvider>,
  );

  it('shows Ask Agents to a support member without Automation access', () => {
    expect(render(true)).toContain('aria-label="Ask Agents"');
  });

  it('hides Ask Agents when the deployment disables it', () => {
    expect(render(false)).not.toContain('aria-label="Ask Agents"');
  });
});
