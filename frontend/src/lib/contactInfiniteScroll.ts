export interface ContactInfiniteScrollParams {
  hasNextPage?: boolean;
  isFetchingNextPage?: boolean;
  loadedCount: number;
  lastVisibleIndex: number | null;
  preloadThreshold?: number;
}

export function shouldFetchNextContactPage({
  hasNextPage,
  isFetchingNextPage,
  loadedCount,
  lastVisibleIndex,
  preloadThreshold = 8,
}: ContactInfiniteScrollParams): boolean {
  if (!hasNextPage || isFetchingNextPage || loadedCount === 0 || lastVisibleIndex == null) {
    return false;
  }

  return lastVisibleIndex >= loadedCount - 1 - preloadThreshold;
}
