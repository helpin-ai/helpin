// @vitest-environment jsdom
import { renderToStaticMarkup } from 'react-dom/server';
import { describe, expect, it } from 'vitest';
import { AskAgentWorkAnimation } from '../AskAgentWorkAnimation';

describe('AskAgentWorkAnimation', () => {
  it('renders a square snake at the requested size without a runtime animation player', () => {
    const markup = renderToStaticMarkup(<AskAgentWorkAnimation className="h-7 w-7" />);
    const element = document.createElement('div');
    element.innerHTML = markup;

    const loader = element.querySelector('[data-agent-work-loader]');
    expect(loader?.classList.contains('h-7')).toBe(true);
    expect(loader?.classList.contains('w-7')).toBe(true);
    expect(loader?.getAttribute('aria-hidden')).toBe('true');
    expect(loader?.querySelectorAll('.agent-square-snake-piece')).toHaveLength(4);
  });
});
