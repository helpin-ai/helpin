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
});
