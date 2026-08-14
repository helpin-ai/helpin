import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';
import { describe, expect, it } from 'vitest';

describe('CompaniesTable scrolling', () => {
  it('constrains the virtualized table so its overflow container can scroll', () => {
    const tableSource = readFileSync(resolve(__dirname, './CompaniesTable.tsx'), 'utf8');
    const styleSource = readFileSync(resolve(__dirname, '../../lib/tableStyles.ts'), 'utf8');

    expect(tableSource).toContain('className="flex h-full min-h-0 flex-1 flex-col gap-2"');
    expect(tableSource).toContain('<div ref={parentRef} className={TABLE_CONTAINER}>');
    expect(styleSource).toContain("TABLE_CONTAINER = 'min-h-0 flex-1 overflow-auto");
  });
});
