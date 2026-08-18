import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';
import { describe, expect, it } from 'vitest';

describe('CompaniesTable layout', () => {
  it('constrains the virtualized table so its overflow container can scroll', () => {
    const tableSource = readFileSync(resolve(__dirname, './CompaniesTable.tsx'), 'utf8');
    const styleSource = readFileSync(resolve(__dirname, '../../lib/tableStyles.ts'), 'utf8');

    expect(tableSource).toContain('className="flex h-full min-h-0 flex-1 flex-col gap-2"');
    expect(tableSource).toContain('<div ref={parentRef} className={TABLE_CONTAINER}>');
    expect(styleSource).toContain("TABLE_CONTAINER = 'min-h-0 flex-1 overflow-auto");
  });

  it('does not render the internal company ID as a table column', () => {
    const tableSource = readFileSync(resolve(__dirname, './CompaniesTable.tsx'), 'utf8');

    expect(tableSource).not.toContain("columnHelper.accessor('display_id'");
    expect(tableSource).not.toContain("id: 'displayId'");
  });
});
