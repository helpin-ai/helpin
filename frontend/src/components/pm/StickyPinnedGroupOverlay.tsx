import { startTransition, useEffect, useRef, useState, type ReactNode, type RefObject } from 'react';

interface VirtualizerLike {
  getVirtualItems: () => Array<{ index: number; start: number }>;
}

interface StickyPinnedGroupOverlayProps<TItem> {
  parentRef: RefObject<HTMLDivElement | null>;
  items: readonly TItem[];
  virtualizer: VirtualizerLike;
  isPinnedItem: (item: TItem, index: number) => boolean;
  renderHeader: (item: TItem, index: number) => ReactNode;
  /** CSS top offset for the sticky wrapper. Defaults to `0px`. */
  top?: string;
  /** z-index for the sticky wrapper. Defaults to 5. */
  zIndex?: number;
}

/**
 * Renders a sticky overlay anchored below the table header that shows the
 * currently "pinned" item (typically a group label row) while the user
 * scrolls vertically through that group's content.
 *
 * Used by TaskListView and the Epics page to keep the active group label
 * visible as long as one of its rows is on screen.
 */
export function StickyPinnedGroupOverlay<TItem>({
  parentRef,
  items,
  virtualizer,
  isPinnedItem,
  renderHeader,
  top = '0px',
  zIndex = 5,
}: StickyPinnedGroupOverlayProps<TItem>) {
  const pinnedRef = useRef<number | null>(null);
  const rafRef = useRef<number | null>(null);
  const [pinnedIdx, setPinnedIdx] = useState<number | null>(null);

  useEffect(() => {
    return () => {
      if (rafRef.current !== null) cancelAnimationFrame(rafRef.current);
    };
  }, []);

  useEffect(() => {
    const scrollEl = parentRef.current;
    if (!scrollEl) return;

    const sync = () => {
      const scrollTop = scrollEl.scrollTop;
      const virtualItems = virtualizer.getVirtualItems();
      let nextIdx: number | null = null;

      if (scrollTop > 10) {
        for (const vItem of virtualItems) {
          if (vItem.start > scrollTop) break;
          const candidate = items[vItem.index];
          if (candidate !== undefined && isPinnedItem(candidate, vItem.index)) {
            nextIdx = vItem.index;
          }
        }
        if (nextIdx === null && virtualItems.length > 0) {
          for (let i = virtualItems[0].index - 1; i >= 0; i -= 1) {
            const candidate = items[i];
            if (candidate !== undefined && isPinnedItem(candidate, i)) {
              nextIdx = i;
              break;
            }
          }
        }
      }

      if (nextIdx !== pinnedRef.current) {
        pinnedRef.current = nextIdx;
        startTransition(() => {
          setPinnedIdx((current) => (current === nextIdx ? current : nextIdx));
        });
      }
    };

    const onScroll = () => {
      if (rafRef.current !== null) return;
      rafRef.current = requestAnimationFrame(() => {
        rafRef.current = null;
        sync();
      });
    };

    sync();
    scrollEl.addEventListener('scroll', onScroll, { passive: true });

    return () => {
      scrollEl.removeEventListener('scroll', onScroll);
      if (rafRef.current !== null) {
        cancelAnimationFrame(rafRef.current);
        rafRef.current = null;
      }
    };
  }, [parentRef, items, virtualizer, isPinnedItem]);

  const pinnedItem = pinnedIdx !== null ? items[pinnedIdx] : undefined;
  if (pinnedItem === undefined || pinnedIdx === null) return null;

  return (
    <div className="sticky" style={{ top, height: 0, overflow: 'visible', zIndex }}>
      <div data-table-overlay="" className="border-b border-border/60 bg-background">
        {renderHeader(pinnedItem, pinnedIdx)}
      </div>
    </div>
  );
}
