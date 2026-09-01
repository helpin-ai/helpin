export function findLastMatchingIndex<T>(
  items: readonly T[],
  predicate: (item: T, index: number) => boolean,
): number {
  for (let index = items.length - 1; index >= 0; index -= 1) {
    if (predicate(items[index], index)) return index;
  }
  return -1;
}
