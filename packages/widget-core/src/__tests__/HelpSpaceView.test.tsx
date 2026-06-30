import { render } from '@testing-library/preact';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { HelpSpaceView } from '../components/HelpSpaceView';

describe('HelpSpaceView', () => {
  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it('reserves right-side header space when the selected docs space has a subtitle', () => {
    vi.stubGlobal(
      'fetch',
      vi.fn(async () => new Response(JSON.stringify([]), { status: 200 })),
    );

    const { container } = render(
      <HelpSpaceView
        host="https://client.helpin.ai"
        widgetKey="wk_123"
        space={{
          id: 'space-1',
          name: 'A very long customer education and implementation docs space',
          slug: 'implementation-docs',
        }}
        showBack
        onBack={() => {}}
        onSelectCollection={() => {}}
      />,
    );

    const header = container.querySelector('.helpin-help-header');

    expect(header?.querySelector('.helpin-help-back')).toBeTruthy();
    expect(header?.querySelector('.helpin-help-header-copy .helpin-help-title')?.textContent).toBe(
      'A very long customer education and implementation docs space',
    );
    expect(header?.querySelector('.helpin-help-header-copy .helpin-help-subtitle')?.textContent).toBe(
      'Browse collections',
    );
    expect(header?.querySelector('.helpin-help-header-spacer')).toBeTruthy();
  });
});
