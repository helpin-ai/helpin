// @vitest-environment jsdom
import { describe, expect, it } from 'vitest';
import { sanitizeSvg, SVG_EXAMPLE, svgDataUrl } from '../svgRenderer';

describe('static SVG assets', () => {
  it('retains shapes, Unicode text and local marker references while removing handlers', () => {
    const safe = sanitizeSvg(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 100 100" onload="alert(1)"><title>Qatar → Cloud</title><defs><marker id="arrow"><path d="M0 0L10 5L0 10z"/></marker></defs><path d="M0 0L50 50" marker-end="url(#arrow)"/></svg>`);
    expect(safe).not.toContain('onload');
    expect(safe).toContain('marker-end="url(#arrow)"');
    expect(safe).toContain('Qatar → Cloud');
    const decoded = new TextDecoder().decode(Uint8Array.from(atob(svgDataUrl(safe).split(',')[1]), (c) => c.charCodeAt(0)));
    expect(decoded).toBe(safe);
    expect(sanitizeSvg(SVG_EXAMPLE)).toContain('Request');
  });

  it.each([
    '', '<p>text</p>', '<svg><g></svg>',
    '<!DOCTYPE svg><svg/>',
    '<svg><script>alert(1)</script></svg>',
    '<svg><foreignObject><div>HTML</div></foreignObject></svg>',
    '<svg><animate attributeName="x"/></svg>',
    '<svg><image href="https://example.com/image.png"/></svg>',
    '<svg><use href="data:image/svg+xml,anything"/></svg>',
    '<svg><style>@import "https://example.com/style.css";</style></svg>',
    '<svg><path fill="url(https://example.com/paint.svg#x)"/></svg>',
  ])('rejects invalid or nonportable source: %s', (source) => {
    expect(() => sanitizeSvg(source)).toThrow();
  });
});
