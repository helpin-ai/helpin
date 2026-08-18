import { renderToStaticMarkup } from 'react-dom/server';
import { describe, expect, it } from 'vitest';
import { HelpinLogo } from '@/components/layout/HelpinLogo';

describe('HelpinLogo', () => {
  it('uses the theme-correct icon-pack marks in the wordmark lockup', () => {
    const markup = renderToStaticMarkup(<HelpinLogo />);

    expect(markup).toContain('data-helpin-brand-lockup="true"');
    expect(markup).toContain('gap-1');
    expect(markup).toContain('/brand/helpin-icon-ink.svg');
    expect(markup).toContain('/brand/helpin-icon-white.svg');
    expect(markup).not.toContain('scale-125');
  });
});
