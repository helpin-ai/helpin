import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';
import { describe, expect, it } from 'vitest';

describe('TaskDetailPanel view tabs layout', () => {
  it('keeps the view tabs in the main column instead of reserving an empty rail header', () => {
    const source = readFileSync(resolve(__dirname, '../../TaskDetailPanel.tsx'), 'utf8');
    const gridStart = source.indexOf('grid-cols-[1fr_300px] overflow-hidden');
    const leftColumnStart = source.indexOf('flex min-h-0 flex-col', gridStart);
    const tabsStart = source.indexOf('aria-label="Task detail views"', leftColumnStart);
    const rightRailStart = source.indexOf('Right column (sidebar)', tabsStart);

    expect(gridStart).toBeGreaterThan(-1);
    expect(leftColumnStart).toBeGreaterThan(gridStart);
    expect(tabsStart).toBeGreaterThan(leftColumnStart);
    expect(rightRailStart).toBeGreaterThan(tabsStart);
    expect(source).not.toContain('bg-muted/10" aria-hidden');
  });
});
