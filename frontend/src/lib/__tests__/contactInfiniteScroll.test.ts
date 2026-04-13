import { describe, expect, it } from 'vitest';
import { shouldFetchNextContactPage } from '../contactInfiniteScroll';

describe('shouldFetchNextContactPage', () => {
  it('returns true when the last visible row is near the end of loaded contacts', () => {
    expect(shouldFetchNextContactPage({
      hasNextPage: true,
      isFetchingNextPage: false,
      loadedCount: 50,
      lastVisibleIndex: 45,
    })).toBe(true);
  });

  it('returns false when more scrolling is still needed', () => {
    expect(shouldFetchNextContactPage({
      hasNextPage: true,
      isFetchingNextPage: false,
      loadedCount: 50,
      lastVisibleIndex: 30,
    })).toBe(false);
  });

  it('returns false while a next page is already loading', () => {
    expect(shouldFetchNextContactPage({
      hasNextPage: true,
      isFetchingNextPage: true,
      loadedCount: 50,
      lastVisibleIndex: 49,
    })).toBe(false);
  });

  it('returns false when there is no next page', () => {
    expect(shouldFetchNextContactPage({
      hasNextPage: false,
      isFetchingNextPage: false,
      loadedCount: 50,
      lastVisibleIndex: 49,
    })).toBe(false);
  });
});
