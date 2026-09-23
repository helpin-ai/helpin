// @vitest-environment jsdom
import { act, type ReactNode } from 'react';
import { createRoot, type Root } from 'react-dom/client';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { GitProviderButton, GitProviderIcon } from '../GitProviderIcon';
import { gitProviderLabels, gitProviderOf } from '../gitProvider';

(globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true;

let root: Root | null = null;
let container: HTMLDivElement | null = null;

function render(node: ReactNode) {
  container = document.createElement('div');
  document.body.appendChild(container);
  root = createRoot(container);
  act(() => root?.render(node));
  return container;
}

afterEach(() => {
  act(() => root?.unmount());
  container?.remove();
  root = null;
  container = null;
});

describe('GitProviderIcon', () => {
  it('draws the GitHub mark in brand ink that inverts in dark mode', () => {
    const view = render(<GitProviderIcon provider="github" />);
    const svg = view.querySelector('svg[data-git-provider-icon="github"]') as SVGElement;
    expect(svg).not.toBeNull();
    expect(svg.getAttribute('aria-hidden')).toBe('true');
    expect(svg.getAttribute('fill')).toBe('currentColor');
    expect(svg.getAttribute('class')).toContain('text-[#181717]');
    expect(svg.getAttribute('class')).toContain('dark:text-white');
    expect(svg.querySelectorAll('path')).toHaveLength(1);
  });

  it('draws the GitLab tanuki in its fixed brand colors', () => {
    const view = render(<GitProviderIcon provider="gitlab" />);
    const svg = view.querySelector('svg[data-git-provider-icon="gitlab"]') as SVGElement;
    expect(svg.getAttribute('viewBox')).toBe('0 0 25 24');
    const fills = Array.from(svg.querySelectorAll('path')).map((path) => path.getAttribute('fill'));
    expect(new Set(fills)).toEqual(new Set(['#E24329', '#FC6D26', '#FCA326']));
    expect(svg.getAttribute('class')).not.toContain('dark:');
  });

  it('maps stored provider values and labels', () => {
    expect(gitProviderOf('gitlab')).toBe('gitlab');
    expect(gitProviderOf('github')).toBe('github');
    expect(gitProviderOf(undefined)).toBe('github');
    expect(gitProviderLabels).toEqual({ github: 'GitHub', gitlab: 'GitLab' });
  });
});

describe('GitProviderButton', () => {
  it('renders a quiet outlined action with the mark and an accessible label', () => {
    const onClick = vi.fn();
    const view = render(
      <>
        <GitProviderButton provider="github" onClick={onClick}>Connect GitHub</GitProviderButton>
        <GitProviderButton provider="gitlab">Connect GitLab</GitProviderButton>
      </>,
    );
    const [github, gitlab] = Array.from(view.querySelectorAll('button'));
    expect(github.textContent).toBe('Connect GitHub');
    expect(github.getAttribute('type')).toBe('button');
    expect(github.dataset.variant).toBe('outline');
    expect(github.className).toContain('rounded-[7px]');
    expect(github.className).toContain('bg-quiet-surface');
    expect(github.className).not.toContain('bg-primary');
    expect(github.querySelector('svg[data-git-provider-icon="github"]')).not.toBeNull();
    expect(gitlab.textContent).toBe('Connect GitLab');
    expect(gitlab.querySelector('svg[data-git-provider-icon="gitlab"]')).not.toBeNull();

    act(() => github.click());
    expect(onClick).toHaveBeenCalledTimes(1);
  });
});
