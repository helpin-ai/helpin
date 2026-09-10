// @vitest-environment jsdom
import { describe, expect, it } from 'vitest';
import { NWDIAG_EXAMPLE, renderNwdiagSvg } from '../nwdiagRenderer';

describe('renderNwdiagSvg', () => {
  it('renders the starter network with distinct light and dark palettes', async () => {
    const light = await renderNwdiagSvg(NWDIAG_EXAMPLE);
    const dark = await renderNwdiagSvg(NWDIAG_EXAMPLE, 'dark');
    const svg = new DOMParser().parseFromString(light, 'image/svg+xml');
    expect(svg.querySelector('parsererror')).toBeNull();
    expect(svg.documentElement.localName).toBe('svg');
    expect(svg.documentElement.getAttribute('viewBox')).toBeTruthy();
    expect(svg.documentElement.textContent).toContain('web01');
    expect(svg.documentElement.textContent).toContain('web02');
    expect(dark).not.toBe(light);
    expect(dark).toContain('#f8fafc');
  });

  it.each(['', ' ', 'not a diagram', 'nwdiag { network dmz {'])('rejects invalid source %j', async (source) => {
    await expect(renderNwdiagSvg(source)).rejects.toThrow();
  });

  it('keeps user labels as text', async () => {
    const svg = await renderNwdiagSvg('nwdiag { network dmz { web [label = "<script>alert(1)</script>"]; } }');
    const host = document.createElement('div');
    host.innerHTML = svg;
    expect(host.querySelector('script')).toBeNull();
    expect(host.textContent).toContain('<script>alert(1)</script>');
  });
});
