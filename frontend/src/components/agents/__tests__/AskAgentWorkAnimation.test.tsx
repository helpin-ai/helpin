// @vitest-environment jsdom
import { renderToStaticMarkup } from 'react-dom/server';
import { describe, expect, it } from 'vitest';
import { AskAgentWorkAnimation } from '../AskAgentWorkAnimation';

describe('AskAgentWorkAnimation', () => {
  it('renders the neutral CSS loader at the requested size without child elements', () => {
    const markup = renderToStaticMarkup(<AskAgentWorkAnimation className="h-7 w-7" />);
    const element = document.createElement('div');
    element.innerHTML = markup;

    const loader = element.querySelector('[data-agent-work-loader]');
    expect(loader?.classList.contains('h-7')).toBe(true);
    expect(loader?.classList.contains('w-7')).toBe(true);
    expect(loader?.getAttribute('aria-hidden')).toBe('true');
    expect(loader?.classList.contains('agent-work-orbit')).toBe(true);
    expect(loader?.classList.contains('text-quiet-text-secondary')).toBe(true);
    expect(loader?.children).toHaveLength(0);
  });
});
