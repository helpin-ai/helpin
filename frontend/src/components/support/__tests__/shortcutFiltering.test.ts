import { describe, expect, it } from 'vitest';
import { filterShortcuts } from '../shortcutFiltering';
import type { SupportCannedResponse } from '@/lib/pmTypes';

function shortcut(overrides: Pick<SupportCannedResponse, 'id' | 'short_code' | 'content'>): SupportCannedResponse {
  return {
    workspace_id: 'workspace-1',
    tag: 'General',
    created_by_id: null,
    created_at: '2026-05-01T00:00:00Z',
    updated_at: '2026-05-01T00:00:00Z',
    ...overrides,
  };
}

describe('filterShortcuts', () => {
  it('filters manual search by shortcut code and saved message content', () => {
    const shortcuts = [
      shortcut({ id: '1', short_code: '!refund', content: '<p>We can issue a refund.</p>' }),
      shortcut({ id: '2', short_code: '!invoice', content: '<p>I attached the receipt.</p>' }),
      shortcut({ id: '3', short_code: '!hello', content: '<p>Hello there.</p>' }),
    ];

    expect(filterShortcuts(shortcuts, 'receipt').map((item) => item.short_code)).toEqual(['!invoice']);
    expect(filterShortcuts(shortcuts, '!ref').map((item) => item.short_code)).toEqual(['!refund']);
  });

  it('caps results at eight for compact shortcut panels', () => {
    const shortcuts = Array.from({ length: 10 }, (_, index) =>
      shortcut({ id: String(index), short_code: `!reply${index}`, content: '<p>Common reply</p>' }),
    );

    expect(filterShortcuts(shortcuts, '').map((item) => item.short_code)).toHaveLength(8);
  });
});
