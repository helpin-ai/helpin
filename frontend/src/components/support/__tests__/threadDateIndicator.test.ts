import { describe, expect, it } from 'vitest';
import { getScrollDateIndicator } from '../threadDateIndicator';

const separators = [
  { top: 100, label: 'Monday' },
  { top: 500, label: 'Tuesday' },
  { top: 900, label: 'Wednesday' },
];

describe('getScrollDateIndicator', () => {
  it('shows the current date only while scrolling upward', () => {
    expect(getScrollDateIndicator({ scrollTop: 700, previousScrollTop: 760, separators })).toBe('Tuesday');
    expect(getScrollDateIndicator({ scrollTop: 700, previousScrollTop: 640, separators })).toBeNull();
  });

  it('yields to the real separator as it reaches the floating label', () => {
    expect(getScrollDateIndicator({ scrollTop: 530, previousScrollTop: 560, separators })).toBe('Tuesday');
    expect(getScrollDateIndicator({ scrollTop: 515, previousScrollTop: 545, separators })).toBeNull();
    expect(getScrollDateIndicator({ scrollTop: 485, previousScrollTop: 515, separators })).toBeNull();
    expect(getScrollDateIndicator({ scrollTop: 450, previousScrollTop: 480, separators })).toBe('Monday');
  });

  it('hides before the oldest loaded day separator', () => {
    expect(getScrollDateIndicator({ scrollTop: 80, previousScrollTop: 120, separators })).toBeNull();
  });
});
