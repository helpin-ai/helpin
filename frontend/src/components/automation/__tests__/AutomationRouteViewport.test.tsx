import { renderToStaticMarkup } from 'react-dom/server';
import { describe, expect, it } from 'vitest';
import { AutomationRouteViewport } from '../AutomationRouteViewport';

describe('AutomationRouteViewport', () => {
  it('uses the shared workspace gutters and content width', () => {
    const markup = renderToStaticMarkup(
      <AutomationRouteViewport>
        <div>Automation page</div>
      </AutomationRouteViewport>,
    );

    expect(markup).toContain('p-4');
    expect(markup).toContain('md:p-6');
    expect(markup).toContain('[scrollbar-gutter:stable]');
    expect(markup).toContain('mx-auto w-full max-w-7xl');
  });
});
