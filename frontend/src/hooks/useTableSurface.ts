import { useCallback, type RefObject } from 'react';

/** Clip scrolling content behind transparent headers and pinned columns. */
export function useTableSurface(scrollRef: RefObject<HTMLDivElement | null>) {
  return useCallback((element: HTMLDivElement | null) => {
    scrollRef.current = element;
    if (!element) return;

    let frame: number | null = null;
    const sync = () => {
      frame = null;
      const viewport = element.getBoundingClientRect();
      let headerBottom = viewport.top;
      for (const header of element.querySelectorAll<HTMLElement>('.shared-table-header, [data-table-overlay]')) {
        const rect = header.getBoundingClientRect();
        if (rect.top <= viewport.top + 1) headerBottom = Math.max(headerBottom, rect.bottom);
      }

      // Read all geometry before writing styles to avoid repeated layouts.
      const updates: Array<[HTMLElement, string]> = [];
      for (const row of element.querySelectorAll<HTMLElement>('.shared-table-row')) {
        if (row.closest('[data-table-overlay]')) continue;
        const rect = row.getBoundingClientRect();
        const top = Math.max(0, Math.min(rect.height, headerBottom - rect.top));
        updates.push([row, top > 0 ? `inset(${top}px 0px 0px)` : '']);
      }

      const rows = new Map<Element, HTMLElement[]>();
      for (const cell of element.querySelectorAll<HTMLElement>('.shared-table-cell')) {
        if (!cell.parentElement) continue;
        const siblings = rows.get(cell.parentElement) ?? [];
        siblings.push(cell);
        rows.set(cell.parentElement, siblings);
      }
      for (const cells of rows.values()) {
        const geometry = cells.map((cell) => ({
          cell,
          rect: cell.getBoundingClientRect(),
          left: cell.classList.contains('shared-table-pinned-left'),
          right: cell.classList.contains('shared-table-pinned-right'),
        }));
        const left = Math.max(viewport.left, ...geometry.filter((cell) => cell.left).map(({ rect }) => rect.right));
        const right = Math.min(viewport.right, ...geometry.filter((cell) => cell.right).map(({ rect }) => rect.left));
        for (const entry of geometry) {
          if (entry.left || entry.right) continue;
          const leftInset = Math.max(0, Math.min(entry.rect.width, left - entry.rect.left));
          const rightInset = Math.max(0, Math.min(entry.rect.width, entry.rect.right - right));
          updates.push([entry.cell, leftInset || rightInset ? `inset(0px ${rightInset}px 0px ${leftInset}px)` : '']);
        }
      }
      for (const [node, clipPath] of updates) {
        if (node.style.clipPath !== clipPath) node.style.clipPath = clipPath;
      }
    };
    const schedule = () => {
      if (frame === null) frame = requestAnimationFrame(sync);
    };
    const mutations = new MutationObserver(schedule);
    mutations.observe(element, { childList: true, subtree: true, attributes: true, attributeFilter: ['style', 'class'] });
    const resize = typeof ResizeObserver === 'undefined' ? null : new ResizeObserver(schedule);
    resize?.observe(element);
    element.addEventListener('scroll', schedule, { passive: true });
    schedule();

    return () => {
      if (frame !== null) cancelAnimationFrame(frame);
      element.removeEventListener('scroll', schedule);
      mutations.disconnect();
      resize?.disconnect();
      scrollRef.current = null;
    };
  }, [scrollRef]);
}
