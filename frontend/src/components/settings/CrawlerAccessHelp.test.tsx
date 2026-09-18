// @vitest-environment jsdom
import { act } from 'react';
import { createRoot } from 'react-dom/client';
import { expect, it } from 'vitest';
import { CrawlerAccessHelp } from './CrawlerAccessHelp';

(globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true;
it('opens crawler setup instructions with rules for both supported crawlers', async () => {
  const container = document.createElement('div');
  document.body.append(container);
  const root = createRoot(container);
  try {
    await act(async () => root.render(<CrawlerAccessHelp />));
    const details = container.querySelector('details')!;
    expect(details.open).toBe(false);
    expect(container.querySelector('summary')?.textContent).toBe('Allow Helpin to crawl your website');
    await act(async () => container.querySelector('summary')!.click());
    expect(details.open).toBe(true);
    expect(container.querySelector('pre')?.textContent).toContain('User-agent: Helpin-Crawler\nAllow: /docs/\nDisallow: /');
    expect(container.querySelector('pre')?.textContent).toContain('User-agent: CloudflareBrowserRenderingCrawler');
    expect(container.textContent).toContain('Then re-sync the source.');
  } finally {
    await act(async () => root.unmount());
    container.remove();
  }
});
