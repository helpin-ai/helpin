import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';
import { describe, expect, it } from 'vitest';

describe('TaskDetailPanel view tabs layout', () => {
  it('keeps the view tabs in the main column instead of reserving an empty rail header', () => {
    const source = readFileSync(resolve(__dirname, '../../TaskDetailPanel.tsx'), 'utf8');
    const gridStart = source.indexOf('grid-cols-[minmax(0,1fr)_300px] overflow-hidden');
    const leftColumnStart = source.indexOf('flex min-h-0 min-w-0 flex-col', gridStart);
    const tabsStart = source.indexOf('aria-label="Task detail views"', leftColumnStart);
    const rightRailStart = source.indexOf('Right column (sidebar)', tabsStart);

    expect(gridStart).toBeGreaterThan(-1);
    expect(leftColumnStart).toBeGreaterThan(gridStart);
    expect(tabsStart).toBeGreaterThan(leftColumnStart);
    expect(rightRailStart).toBeGreaterThan(tabsStart);
    expect(source).not.toContain('bg-muted/10" aria-hidden');
  });

  it('contains Updates content without allowing the right sheet to exceed the viewport', () => {
    const panelSource = readFileSync(resolve(__dirname, '../../TaskDetailPanel.tsx'), 'utf8');
    const updatesSource = readFileSync(resolve(__dirname, '../TaskUpdatesView.tsx'), 'utf8');

    expect(panelSource).toContain('flex h-full min-w-0 flex-col overflow-hidden');
    expect(panelSource).toContain('grid-cols-[minmax(0,1fr)_300px] overflow-hidden');
    expect(panelSource).toContain('overflow-x-hidden overflow-y-auto');
    expect(panelSource).toContain('max-w-[100vw] overflow-hidden p-0');
    expect(updatesSource).toContain('min-w-0 space-y-5');
    expect(updatesSource).toContain('min-w-0 divide-y divide-border/50 overflow-hidden');
  });
});
