import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';
import { describe, expect, it } from 'vitest';

describe('Contacts toolbar layout', () => {
  it('keeps search and filters on the left and table controls on the right', () => {
    const pageSource = readFileSync(resolve(__dirname, '../../pages/crm/Contacts.tsx'), 'utf8');
    const tableSource = readFileSync(resolve(__dirname, './ContactsTable.tsx'), 'utf8');

    expect(pageSource.indexOf('<QuietSearchInput')).toBeLessThan(pageSource.indexOf('<ContactsFilterBar'));
    expect(pageSource).toContain('context={!isLoading ? totalCount : undefined}');
    expect(pageSource).toContain('max-w-full flex-wrap items-center justify-end');
    expect(pageSource).toContain('hidden h-7 text-xs sm:inline-flex');
    expect(pageSource).toContain('<span ref={setTableToolbarContainer} className="contents" />');
    expect(pageSource).toContain('<ContactsActiveFilterBar');
    expect(pageSource).toContain('toolbarContainer={tableToolbarContainer}');
    expect(tableSource).toContain('createPortal(tableToolbar, toolbarContainer)');
    expect(tableSource).toContain('min-w-[150px] max-w-[190px]');
    expect(tableSource).not.toContain('totalCount?: number');
  });

  it('uses the Tasks display-columns icon across CRM list controls', () => {
    const taskMenuSource = readFileSync(resolve(__dirname, '../pm/ListDisplayMenu.tsx'), 'utf8');
    const sharedMenuSource = readFileSync(resolve(__dirname, './ColumnVisibilityPopover.tsx'), 'utf8');
    const dealMenuSource = readFileSync(resolve(__dirname, './DealDisplayMenu.tsx'), 'utf8');
    const companiesPageSource = readFileSync(resolve(__dirname, '../../pages/crm/Companies.tsx'), 'utf8');

    expect(taskMenuSource).toContain('ColumnsThreeCogIcon');
    expect(sharedMenuSource).toContain('ColumnsThreeCogIcon');
    expect(sharedMenuSource).not.toContain('ViewIcon');
    expect(dealMenuSource).toContain('ColumnsThreeCogIcon');
    expect(dealMenuSource).not.toContain('Settings02Icon');
    expect(companiesPageSource).toContain('toolbarContainer={tableToolbarContainer}');
  });
});
