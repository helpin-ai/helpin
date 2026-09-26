export type DateSeparatorPosition = { top: number; label: string };

export function getScrollDateIndicator({ scrollTop, previousScrollTop, separators }: {
  scrollTop: number;
  previousScrollTop: number;
  separators: DateSeparatorPosition[];
}): string | null {
  if (scrollTop >= previousScrollTop) return null;

  // The real separator takes over while crossing the floating label's position.
  if (separators.some(({ top }) => Math.abs(top - scrollTop) <= 24)) return null;

  let currentDate: string | null = null;
  for (const separator of separators) {
    if (separator.top > scrollTop) break;
    currentDate = separator.label;
  }
  return currentDate;
}
