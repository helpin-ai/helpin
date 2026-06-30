import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';
import { describe, expect, it } from 'vitest';

const css = readFileSync(resolve(__dirname, '../styles/widget.css'), 'utf-8');

function ruleBody(selector: string): string {
  const escapedSelector = selector.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');
  const match = css.match(new RegExp(`${escapedSelector}\\s*\\{([^}]*)\\}`));
  return match?.[1] ?? '';
}

describe('widget scroll styles', () => {
  it('keeps help and docs headers outside the scrollable region', () => {
    expect(ruleBody('.helpin-view-container')).toContain('overflow: hidden');
    expect(ruleBody('.helpin-home-content')).toContain('overflow-y: auto');
    expect(ruleBody('.helpin-help-content')).toContain('overflow-y: auto');
    expect(ruleBody('.helpin-article-content')).toContain('overflow-y: auto');
  });

  it('animates Helpin branding underlines from left to right on hover', () => {
    expect(ruleBody('.helpin-powered-by-name')).toContain('background-size: 0 1px');
    expect(ruleBody('.helpin-powered-by:hover .helpin-powered-by-name')).toContain('background-size: 100% 1px');
    expect(ruleBody('.helpin-compose-footer-link')).toContain('background-size: 0 1px');
    expect(ruleBody('.helpin-compose-footer:hover .helpin-compose-footer-link')).toContain('background-size: 100% 1px');
  });

  it('slides help drilldown screens from right to left', () => {
    expect(ruleBody('.helpin-help-drilldown-view')).toContain('animation: helpin-drilldown-enter 0.38s');
    expect(css).toContain('@keyframes helpin-drilldown-enter');
    expect(css).toContain('translateX(28px)');
    expect(css).toContain('translateX(0)');
  });

  it('reveals the message thread without adding per-message stagger', () => {
    expect(css).toContain('.helpin-message-list--smooth-enter {\n  animation: helpin-message-list-in');
    expect(ruleBody('.helpin-message-row')).toContain('animation: helpin-bubble-in');
    expect(ruleBody('.helpin-message-row')).not.toContain('animation-delay');
    expect(css).toContain('@keyframes helpin-message-list-in');
  });

  it('honors reduced motion for widget entry animations', () => {
    expect(css).toContain('@media (prefers-reduced-motion: reduce)');
    expect(css).toContain('.helpin-help-drilldown-view,');
    expect(css).toContain('.helpin-message-list--smooth-enter,');
    expect(css).toContain('.helpin-help-link-skeleton-icon,');
    expect(css).toContain('.helpin-help-link-skeleton-line');
  });
});
