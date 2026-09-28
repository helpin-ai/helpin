export type DateSeparatorPosition = { top: number; label: string };

export function getScrollDateIndicator({ scrollTop, previousScrollTop, floatingTop = 0, separators }: {
  scrollTop: number;
  previousScrollTop: number;
  floatingTop?: number;
  separators: DateSeparatorPosition[];
}): string | null {
  if (scrollTop >= previousScrollTop) return null;

  const handoffTop = scrollTop + floatingTop;
  // Yield only when the two labels actually meet, not as the inline label nears the viewport edge.
  if (separators.some(({ top }) => Math.abs(top - handoffTop) <= 2)) return null;

  let currentDate: string | null = null;
  for (const separator of separators) {
    if (separator.top > handoffTop) break;
    currentDate = separator.label;
  }
  return currentDate;
}
