import { describe, expect, it } from 'vitest';

import {
  buildSearchCommandValue,
  buildTaskCommandValue,
  normalizeCommandSearchText,
} from '@/components/search/searchCommandPalette';

describe('normalizeCommandSearchText', () => {
  it('treats punctuation as word separators for cmdk matching', () => {
    expect(normalizeCommandSearchText('Billing | Invoices / API-v2')).toBe('billing invoices api v2');
  });
});

describe('buildSearchCommandValue', () => {
  it('includes normalized text so punctuation does not hide returned results', () => {
    expect(
      buildSearchCommandValue('document', {
        id: 'doc-1',
        name: 'Billing | Invoices',
      }),
    ).toBe('document doc-1 Billing | Invoices billing invoices');
  });
});

describe('buildTaskCommandValue', () => {
  it('includes task key, numeric display id, and task name for cmdk matching', () => {
    expect(
      buildTaskCommandValue({
        id: 'task-123',
        task_key: 'HLP-123',
        display_id: 123,
        name: 'Fix command palette search',
      }),
    ).toBe('task task-123 HLP-123 123 Fix command palette search fix command palette search');
  });

  it('falls back cleanly when the task key is missing', () => {
    expect(
      buildTaskCommandValue({
        id: 'task-42',
        display_id: 42,
        name: 'Unkeyed task',
      }),
    ).toBe('task task-42 42 Unkeyed task unkeyed task');
  });
});
