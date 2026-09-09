import { useLayoutEffect, useRef, useState } from 'react';
import { useCommandState } from 'cmdk';

/** Hide optional local search only when the complete dropdown list fits. */
export function useDropdownSearch(enabled: boolean) {
  const wrapperRef = useRef<HTMLDivElement>(null);
  const [hidden, setHidden] = useState(false);
  const search = useCommandState((state) => state.search);

  useLayoutEffect(() => {
    const wrapper = wrapperRef.current;
    const command = wrapper?.closest('[cmdk-root]');
    const list = command?.querySelector<HTMLElement>('[cmdk-list]');
    if (!enabled || !wrapper || !list || !command?.closest('[data-slot="popover-content"], [data-dropdown-content]')) {
      setHidden(false);
      return;
    }

    let frame: number | undefined;
    const measure = () => {
      frame = undefined;
      // Filtering must never remove the field the user is typing in.
      if (search) {
        setHidden(false);
        return;
      }
      if (list.clientHeight === 0) return;
      const maxHeight = Number.parseFloat(getComputedStyle(list).maxHeight);
      // Include the space released by hiding search in constrained popovers.
      // This keeps visibility stable when the content is near the limit.
      const capacity = Math.min(
        Number.isFinite(maxHeight) ? maxHeight : Infinity,
        list.clientHeight + wrapper.offsetHeight,
      );
      const fits = list.scrollHeight <= capacity + 1;
      // Move focus before applying `hidden`, which otherwise blurs the input.
      if (fits && wrapper.contains(document.activeElement)) list.focus({ preventScroll: true });
      setHidden(fits);
    };
    const schedule = () => {
      if (frame === undefined) frame = requestAnimationFrame(measure);
    };
    const resize = new ResizeObserver(schedule);
    resize.observe(list);
    resize.observe(command);
    const content = list.querySelector('[cmdk-list-sizer]');
    if (content) resize.observe(content);
    const mutations = new MutationObserver(schedule);
    mutations.observe(list, { childList: true, subtree: true, characterData: true });
    measure();
    return () => {
      if (frame !== undefined) cancelAnimationFrame(frame);
      resize.disconnect();
      mutations.disconnect();
    };
  }, [enabled, search]);

  useLayoutEffect(() => {
    if (!hidden) return;
    const wrapper = wrapperRef.current;
    const list = wrapper?.closest('[cmdk-root]')?.querySelector<HTMLElement>('[cmdk-list]');
    if (!wrapper || !list) return;
    const previousTabIndex = list.tabIndex;
    list.tabIndex = 0;
    // cmdk handles Arrow/Enter keys from its list as well as its input.
    if (wrapper.contains(document.activeElement)) list.focus({ preventScroll: true });
    return () => { list.tabIndex = previousTabIndex; };
  }, [hidden]);

  return { wrapperRef, hidden };
}
