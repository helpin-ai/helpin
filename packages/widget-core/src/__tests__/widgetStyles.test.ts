import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';
import { describe, expect, it } from 'vitest';

const css = readFileSync(resolve(__dirname, '../styles/widget.css'), 'utf-8');

function ruleBody(selector: string): string {
  const escapedSelector = selector.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');
  const match = css.match(new RegExp(`(?:^|})\\s*${escapedSelector}\\s*\\{([^}]*)\\}`, 'm'));
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
    expect(ruleBody('.helpin-brand-attribution')).toContain('display: flex');
    expect(ruleBody('.helpin-brand-attribution')).toContain('align-items: center');
    expect(ruleBody('.helpin-brand-attribution')).toContain('justify-content: center');
    expect(ruleBody('.helpin-brand-attribution')).toContain('text-decoration: none');
    expect(ruleBody('.helpin-brand-attribution-brand')).toContain('display: inline-flex');
    expect(ruleBody('.helpin-brand-attribution-brand')).toContain('align-items: center');
    expect(ruleBody('.helpin-brand-attribution-brand')).toContain('gap: 2px');
    expect(ruleBody('.helpin-brand-attribution-name')).toContain('background-size: 0 1px');
    expect(ruleBody('.helpin-brand-attribution-icon')).not.toContain('transform: scale(1.25)');
    expect(ruleBody('.helpin-brand-attribution-icon')).toContain('color: var(--helpin-fg-secondary)');
    expect(ruleBody('.helpin-brand-attribution:hover .helpin-brand-attribution-name')).toContain('background-size: 100% 1px');
  });

  it('slides help drilldown screens from right to left', () => {
    expect(ruleBody('.helpin-help-drilldown-view')).toContain('animation: helpin-drilldown-enter 0.38s');
    expect(css).toContain('@keyframes helpin-drilldown-enter');
    expect(css).toContain('translateX(28px)');
    expect(css).toContain('translateX(0)');
  });

  it('animates destination views and reveals loaded rows', () => {
    expect(ruleBody('.helpin-view-enter')).toContain('animation: helpin-view-enter 0.24s');
    expect(css).toContain('@keyframes helpin-view-enter');
    expect(css).toContain('.helpin-stagger-list > .helpin-help-link,');
    expect(css).toContain('.helpin-stagger-list > .helpin-conversation-item {');
    expect(css).toContain('animation: helpin-list-item-enter 0.26s');
    expect(css).toContain('@keyframes helpin-list-item-enter');
  });

  it('reveals the message thread without adding per-message stagger', () => {
    expect(css).toContain('.helpin-message-list--smooth-enter {\n  animation: helpin-message-list-in');
    expect(ruleBody('.helpin-message-row')).toContain('animation: helpin-bubble-in');
    expect(ruleBody('.helpin-message-row')).not.toContain('animation-delay');
    expect(css).toContain('@keyframes helpin-message-list-in');
  });

  it('honors reduced motion for widget entry animations', () => {
    expect(css).toContain('@media (prefers-reduced-motion: reduce)');
    expect(css).toContain('.helpin-view-enter,');
    expect(css).toContain('.helpin-help-drilldown-view,');
    expect(css).toContain('.helpin-message-list--smooth-enter,');
    expect(css).toContain('.helpin-help-link-skeleton-icon,');
    expect(css).toContain('.helpin-help-link-skeleton-line');
  });

  it('uses neutral elevated surfaces and visible shimmer contrast in dark mode', () => {
    expect(ruleBody('.helpin-theme-dark')).toContain('--helpin-bg: #121419');
    expect(ruleBody('.helpin-theme-dark')).toContain('--helpin-card-bg: #1b1f26');
    expect(ruleBody('.helpin-chat-window.helpin-theme-dark')).toContain('border: 1px solid rgba(255, 255, 255, 0.1)');
    expect(ruleBody('.helpin-theme-dark .helpin-bottom-nav-item--active')).not.toContain('border-radius');
    expect(css).toContain('.helpin-bottom-nav-icon {\n  display: inline-flex;');
    expect(css).toContain('  border-radius: 9px;\n  transition: background 0.16s ease');
    expect(ruleBody('.helpin-bottom-nav-item--active .helpin-bottom-nav-icon')).toContain('background: var(--helpin-nav-active-color)');
    expect(ruleBody('.helpin-bottom-nav-item--active .helpin-bottom-nav-icon')).toContain('color: var(--helpin-nav-active-foreground)');
    expect(ruleBody('.helpin-theme-dark .helpin-ai-thinking-line')).toContain('rgba(255, 255, 255, 0.24)');
  });

  it('keeps transparent conversation logos visible without cropping them', () => {
    expect(ruleBody('.helpin-conversation-logo--brand')).toContain('padding: 5px');
    expect(ruleBody('.helpin-conversation-logo--brand')).toContain('overflow: hidden');
    expect(ruleBody('.helpin-conversation-brand-logo')).toContain('object-fit: contain');
    expect(ruleBody('.helpin-message-avatar--brand')).toContain('padding: 4px');
    expect(ruleBody('.helpin-message-brand-logo')).toContain('object-fit: contain');
  });

  it('uses a layered, brand-aware surface system in light mode', () => {
    expect(ruleBody('.helpin-theme-light')).toContain('--helpin-bg: #f7f8fb');
    expect(ruleBody('.helpin-theme-light')).toContain('--helpin-card-bg: #ffffff');
    expect(ruleBody('.helpin-theme-light')).toContain('--helpin-fg: #172033');
    expect(ruleBody('.helpin-chat-window.helpin-theme-light')).toContain('border: 1px solid rgba(23, 32, 51, 0.09)');
    expect(css).toContain('.helpin-theme-light .helpin-home-action:hover,');
    expect(css).toContain('background: color-mix(in srgb, var(--helpin-primary) 3.5%, var(--helpin-card-bg))');
    expect(ruleBody('.helpin-theme-light .helpin-help-search:focus-within')).toContain('var(--helpin-primary)');
    expect(ruleBody('.helpin-theme-light .helpin-compose-bar')).toContain('background: #ffffff');
    expect(ruleBody('.helpin-theme-light .helpin-bottom-nav')).toContain('border: 1px solid rgba(23, 32, 51, 0.08)');
  });

  it('keeps projected and quoted email content width-safe', () => {
    expect(ruleBody('.helpin-email-message-content')).toContain('min-width: 0');
    expect(ruleBody('.helpin-email-message-content')).toContain('max-width: 100%');
    expect(ruleBody('.helpin-email-quoted-content')).toContain('overflow-wrap: anywhere');
    expect(ruleBody('.helpin-message-content a')).toContain('overflow-wrap: anywhere');
    expect(ruleBody('.helpin-message-content code')).toContain('word-break: break-word');
    expect(ruleBody('.helpin-message-content pre')).toContain('overflow-x: auto');
  });
});
