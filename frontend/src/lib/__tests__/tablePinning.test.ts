import { readFileSync } from 'node:fs';
import { dirname, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';
import { describe, expect, it } from 'vitest';
import {
  TABLE_GROUP_ROW_INNER,
  TABLE_HEADER_CELL_ACTIONS,
  TABLE_PINNED_HEADER_LEFT,
  TABLE_PINNED_HEADER_LEFT_NAME,
  TABLE_PINNED_HEADER_RIGHT,
  TABLE_PINNED_LEFT,
  TABLE_PINNED_LEFT_NAME,
  TABLE_PINNED_RIGHT,
} from '@/lib/tableStyles';

const __dirname = dirname(fileURLToPath(import.meta.url));

describe('table column pinning', () => {
  it('tags every horizontally pinned token with a hook class the stylesheet can unpin', () => {
    for (const token of [TABLE_PINNED_LEFT, TABLE_PINNED_LEFT_NAME, TABLE_PINNED_HEADER_LEFT, TABLE_PINNED_HEADER_LEFT_NAME]) {
      expect(token).toContain('shared-table-pinned-left');
    }
    for (const token of [TABLE_PINNED_RIGHT, TABLE_PINNED_HEADER_RIGHT, TABLE_HEADER_CELL_ACTIONS]) {
      expect(token).toContain('shared-table-pinned-right');
    }
    expect(TABLE_GROUP_ROW_INNER).toContain('shared-table-group-inner');
  });

  it('drops pinning below md so narrow viewports can scroll the whole row', () => {
    const css = readFileSync(resolve(__dirname, '../../index.css'), 'utf8');
    const rule = css.slice(css.indexOf('@media (max-width: 767px)'));
    expect(rule).toContain('.shared-table-pinned-left');
    expect(rule).toContain('.shared-table-pinned-right');
    expect(rule).toContain('.shared-table-group-inner');
    expect(rule).toContain('position: static !important');
  });
});
