// @vitest-environment jsdom
import { act, useRef } from 'react';
import { createRoot } from 'react-dom/client';
import { afterEach, expect, it, vi } from 'vitest';
import { useTableSurface } from '../useTableSurface';

function Preview() {
  const ref = useRef<HTMLDivElement>(null);
  return (
    <div ref={useTableSurface(ref)} data-testid="viewport">
      <div className="shared-table-header" data-testid="header" />
      <div className="shared-table-row" data-testid="row">
        <div className="shared-table-cell shared-table-pinned-left" data-testid="pinned" />
        <div className="shared-table-cell" data-testid="scrolling" />
        <div className="shared-table-cell shared-table-pinned-right" data-testid="actions" />
      </div>
    </div>
  );
}

afterEach(() => { vi.restoreAllMocks(); vi.unstubAllGlobals(); });

it('clips scrolling content behind pinned cells and headers, settles, and resets after scrolling back', async () => {
  const frames = new Map<number, FrameRequestCallback>();
  let nextFrame = 0;
  vi.stubGlobal('requestAnimationFrame', (callback: FrameRequestCallback) => {
    frames.set(++nextFrame, callback);
    return nextFrame;
  });
  vi.stubGlobal('cancelAnimationFrame', (id: number) => frames.delete(id));
  const flushFrame = async () => {
    const callbacks = [...frames.values()];
    frames.clear();
    await act(async () => { callbacks.forEach(callback => callback(0)); });
  };
  vi.stubGlobal('IS_REACT_ACT_ENVIRONMENT', true);
  const container = document.createElement('div');
  document.body.appendChild(container);
  const root = createRoot(container);
  await act(async () => root.render(<Preview />));
  const view = {
    getByTestId: (id: string) => container.querySelector<HTMLElement>(`[data-testid="${id}"]`)!,
  };
  const geometry = (id: string, left: number, top: number, width: number, height: number) => {
    vi.spyOn(view.getByTestId(id), 'getBoundingClientRect').mockReturnValue({
      left, top, width, height, right: left + width, bottom: top + height, x: left, y: top, toJSON: () => ({}),
    });
  };
  geometry('viewport', 0, 0, 600, 300);
  geometry('header', 0, 0, 600, 30);
  geometry('row', 0, 20, 800, 36);
  geometry('pinned', 0, 20, 200, 36);
  geometry('scrolling', 100, 20, 500, 36);
  geometry('actions', 550, 20, 50, 36);
  await flushFrame();
  expect(view.getByTestId('row').style.clipPath).toBe('inset(10px 0px 0px)');
  expect(view.getByTestId('scrolling').style.clipPath).toBe('inset(0px 50px 0px 100px)');
  expect(view.getByTestId('pinned').style.clipPath).toBe('');
  expect(view.getByTestId('actions').style.clipPath).toBe('');
  await flushFrame();
  expect(frames.size).toBe(0);

  geometry('row', 0, 30, 800, 36);
  geometry('scrolling', 200, 30, 300, 36);
  view.getByTestId('viewport').dispatchEvent(new Event('scroll'));
  await flushFrame();
  expect(view.getByTestId('row').style.clipPath).toBe('');
  expect(view.getByTestId('scrolling').style.clipPath).toBe('');
  await act(async () => root.unmount());
  container.remove();
  expect(frames.size).toBe(0);
});
