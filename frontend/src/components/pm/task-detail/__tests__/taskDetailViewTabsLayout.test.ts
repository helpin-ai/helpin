import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';
import { describe, expect, it } from 'vitest';

describe('TaskDetailPanel view tabs layout', () => {
  it('keeps the view tabs in the main column instead of reserving an empty rail header', () => {
    const source = readFileSync(resolve(__dirname, '../../TaskDetailPanel.tsx'), 'utf8');
    const gridStart = source.indexOf('grid-cols-1 overflow-x-hidden overflow-y-auto lg:grid-cols-[minmax(0,1fr)_300px] lg:overflow-hidden');
    const leftColumnStart = source.indexOf('flex min-w-0 flex-col lg:min-h-0', gridStart);
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
    expect(panelSource).toContain('grid-cols-1 overflow-x-hidden overflow-y-auto lg:grid-cols-[minmax(0,1fr)_300px] lg:overflow-hidden');
    expect(panelSource).toContain('data-[side=right]:w-screen data-[side=right]:!max-w-none');
    expect(panelSource).toContain('lg:data-[side=right]:w-[80vw] lg:data-[side=right]:!max-w-[1200px]');
    expect(updatesSource).toContain('min-w-0 space-y-5');
    expect(updatesSource).toContain('min-w-0 divide-y divide-border/50 overflow-hidden');
  });

  it('stacks the metadata rail below task content on mobile', () => {
    const source = readFileSync(resolve(__dirname, '../../TaskDetailPanel.tsx'), 'utf8');
    const leftColumnStart = source.indexOf('Left column (main content)');
    const rightRailStart = source.indexOf('Right column (sidebar)');

    expect(leftColumnStart).toBeGreaterThan(-1);
    expect(rightRailStart).toBeGreaterThan(leftColumnStart);
    expect(source).toContain('border-t border-border/60 px-4 py-5 pb-16 sm:px-6');
    expect(source).toContain('lg:border-l lg:border-t-0');
    expect(source).toContain('px-4 sm:px-6 lg:px-10');
  });
});
