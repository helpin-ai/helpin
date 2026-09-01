import { renderToStaticMarkup } from 'react-dom/server';
import { describe, expect, it } from 'vitest';
import { AutomationRouteViewport } from '../AutomationRouteViewport';
import { AutomationShell } from '../AutomationShell';

describe('AutomationRouteViewport', () => {
  it('lets the shared shell own the compact header, workspace gutters, and scrolling content', () => {
    const markup = renderToStaticMarkup(
      <AutomationRouteViewport>
        <AutomationShell title="Flows">
          <div>Automation page</div>
        </AutomationShell>
      </AutomationRouteViewport>,
    );

    expect(markup).toContain('border-b border-quiet-divider-strong');
    expect(markup).toContain('p-4');
    expect(markup).toContain('md:p-6');
    expect(markup).toContain('[scrollbar-gutter:stable]');
    expect(markup).toContain('mx-auto w-full max-w-7xl');
  });
});
