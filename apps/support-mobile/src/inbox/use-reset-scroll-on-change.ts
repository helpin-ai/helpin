import { useLayoutEffect, type RefObject } from 'react'

/** Keep a newly selected inbox view from inheriting the previous view's scroll offset. */
export function useResetScrollOnChange(
  scrollRef: RefObject<HTMLElement | null>,
  resetKey: string,
) {
  useLayoutEffect(() => {
    const element = scrollRef.current
    if (element && element.scrollTop !== 0) {
      element.scrollTop = 0
    }
  }, [resetKey, scrollRef])
}
