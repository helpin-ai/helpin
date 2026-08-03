import { renderToStaticMarkup } from 'react-dom/server';
import { describe, expect, it } from 'vitest';
import { HelpinLogo } from '@/components/layout/HelpinLogo';

describe('HelpinLogo', () => {
  it('uses an optically tight mark and wordmark lockup', () => {
    const markup = renderToStaticMarkup(<HelpinLogo />);

    expect(markup).toContain('data-helpin-brand-lockup="true"');
    expect(markup).toContain('gap-1');
    expect(markup).not.toContain('gap-2.5');
    expect(markup.match(/scale-125/g)).toHaveLength(2);
  });
});
