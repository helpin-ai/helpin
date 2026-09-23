// @vitest-environment jsdom
import { act } from 'react';
import { createRoot, type Root } from 'react-dom/client';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { AgentLiveStatus } from '../AgentLiveStatus';

vi.mock('lottie-web', () => ({
  default: {
    loadAnimation: () => ({ destroy: vi.fn(), goToAndStop: vi.fn() }),
  },
}));

(globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true;

describe('AgentLiveStatus', () => {
  let container: HTMLDivElement;
  let root: Root;

  beforeEach(() => {
    container = document.createElement('div');
    document.body.appendChild(container);
    root = createRoot(container);
  });

  afterEach(() => {
    act(() => root.unmount());
    container.remove();
  });

  it('replaces working with an offline notice and restores it on reconnect', () => {
    act(() => root.render(<AgentLiveStatus progress={{ label: 'Working…', tone: 'working' }} />));
    const online = vi.spyOn(navigator, 'onLine', 'get');
    try {
      online.mockReturnValue(false);
      act(() => window.dispatchEvent(new Event('offline')));
      expect(container.textContent).toContain('Offline — live updates paused');
      expect(container.querySelector('[data-agent-work-loader]')).toBeNull();
      online.mockReturnValue(true);
      act(() => window.dispatchEvent(new Event('online')));
      expect(container.textContent).toContain('Working');
    } finally { online.mockRestore(); }
  });

  it('uses worked wording and stops ticking after completion', () => {
    const setIntervalSpy = vi.spyOn(window, 'setInterval');
    const clearIntervalSpy = vi.spyOn(window, 'clearInterval');
    act(() => {
      root.render(
        <AgentLiveStatus
          progress={{
            label: 'Worked',
            startedAt: new Date(Date.now() - 9_000).toISOString(),
            tone: 'waiting',
            completed: true,
          }}
        />,
      );
    });

    expect(container.textContent).toContain('Worked for 9s');
    expect(setIntervalSpy).not.toHaveBeenCalled();
    expect(clearIntervalSpy).not.toHaveBeenCalled();

    setIntervalSpy.mockRestore();
    clearIntervalSpy.mockRestore();
  });

  it('renders the selected loader animation beside the shimmering task label', () => {
    act(() => {
      root.render(
        <AgentLiveStatus progress={{ label: 'Searching…', tone: 'working', startedAt: new Date().toISOString() }} />,
      );
    });
    expect(container.querySelector('.agent-streaming-text')).not.toBeNull();
    const loader = container.querySelector('[data-agent-work-loader]');
    expect(loader?.classList.contains('h-7')).toBe(true);
    expect(loader?.classList.contains('w-7')).toBe(true);
  });

  it('uses a dot before elapsed time for every in-progress phase', () => {
    act(() => {
      root.render(
        <AgentLiveStatus progress={{ label: 'Working…', tone: 'working', startedAt: new Date(Date.now() - 9_000).toISOString() }} />,
      );
    });
    expect(container.textContent).toContain('Working · 9s');
    expect(container.textContent).not.toContain('Working for');

    act(() => {
      root.render(
        <AgentLiveStatus progress={{ label: 'Finalizing…', tone: 'working', startedAt: new Date(Date.now() - 10_000).toISOString() }} />,
      );
    });
    expect(container.textContent).toMatch(/Finalizing · (9|10)s/);
    expect(container.textContent).not.toContain('Finalizing for');
  });

  it('does not start or display an elapsed timer while the agent is starting', () => {
    const setIntervalSpy = vi.spyOn(window, 'setInterval');
    act(() => {
      root.render(
        <AgentLiveStatus progress={{ label: 'Starting…', tone: 'working', startedAt: new Date(Date.now() - 9_000).toISOString() }} />,
      );
    });
    expect(container.textContent).toContain('Starting…');
    expect(container.textContent).not.toContain('for 9s');
    expect(setIntervalSpy).not.toHaveBeenCalled();
  });

  it('pauses the work timer while waiting for approval', () => {
    const setIntervalSpy = vi.spyOn(window, 'setInterval');
    act(() => {
      root.render(
        <AgentLiveStatus
          progress={{
            label: 'Waiting for approval',
            tone: 'waiting',
            startedAt: new Date(Date.now() - 9_000).toISOString(),
          }}
        />,
      );
    });

    expect(container.textContent).toBe('Waiting for approval');
    expect(container.querySelector('.agent-paused-dot-pulse')).not.toBeNull();
    expect(setIntervalSpy).not.toHaveBeenCalled();
    setIntervalSpy.mockRestore();
  });
});
