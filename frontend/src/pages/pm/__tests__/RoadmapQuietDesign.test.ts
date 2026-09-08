import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';
import { describe, expect, it } from 'vitest';

const roadmapSource = readFileSync(resolve(__dirname, '../Roadmap.tsx'), 'utf8');
const timelineSource = readFileSync(resolve(__dirname, '../../../components/pm/RoadmapTimeline.tsx'), 'utf8');
const barSource = readFileSync(resolve(__dirname, '../../../components/pm/RoadmapEpicBar.tsx'), 'utf8');
const epicsSource = readFileSync(resolve(__dirname, '../Epics.tsx'), 'utf8');

describe('Roadmap Quiet Hairline composition', () => {
  it('uses the shared page shell, search, sections, and empty states', () => {
    expect(roadmapSource).toContain('<QuietPageHeader');
    expect(roadmapSource).toContain('variant="shell"');
    expect(roadmapSource).toContain('<QuietPageViewport className="min-h-0 flex-1 p-0 md:p-0" contentClassName="max-w-none">');
    expect(roadmapSource).toContain('<QuietSearchInput');
    expect(roadmapSource).toContain('<QuietSectionHeader');
    expect(roadmapSource).toContain('<QuietEmptyState');
    expect(roadmapSource).not.toContain('max-w-[1600px]');
  });

  it('places the full-width timeline before the actionable planning queue', () => {
    expect(roadmapSource.indexOf('<RoadmapTimeline')).toBeLessThan(roadmapSource.indexOf('<PlanningQueue'));
    expect(roadmapSource).toContain('<InlineEpicDateControl');
    expect(roadmapSource).toContain('<InlineEpicObjectivesControl');
    expect(roadmapSource).toContain('Add start');
    expect(roadmapSource).toContain('Add target');
    expect(roadmapSource).toContain('mx-auto w-full max-w-7xl px-4 pb-8 pt-7');
  });

  it('uses the same shared filter trigger and applied-filter row as Tasks', () => {
    expect(roadmapSource).toContain('<PMFilterTrigger');
    expect(roadmapSource).toContain('<PMFilterBar');
    expect(roadmapSource).toContain("type RoadmapFilterKey = 'objective' | 'team' | 'health' | 'completed'");
    expect(roadmapSource).not.toContain('<DropdownMenuCheckboxItem');
    expect(roadmapSource).not.toContain('function ActiveFilter');
  });

  it('shares Epic planning editors instead of duplicating them', () => {
    expect(epicsSource).toContain("from '@/components/pm/InlineEpicPlanningFields'");
    expect(epicsSource).toContain('<InlineEpicDateControl');
    expect(epicsSource).toContain('<InlineEpicObjectivesControl');
  });

  it('uses hairline timeline structure and restrained duration bars', () => {
    expect(timelineSource).toContain('border-y border-quiet-divider-strong');
    expect(timelineSource).toContain('bg-quiet-accent');
    expect(timelineSource).toContain("const GroupIcon = groupBy === 'objective' ? Target01Icon : UserGroupIcon");
    expect(timelineSource).toContain('buildRoadmapQuarterSegments(months)');
    expect(timelineSource).toContain('startOfQuarter(subQuarters');
    expect(timelineSource).toContain('endOfQuarter(addQuarters');
    expect(timelineSource).toContain("segment.label");
    expect(timelineSource).toContain("zoom === 'month' ? 140 : 72");
    expect(timelineSource).toContain("isEpicGrouping ? 'px-3' : 'pl-7 pr-3'");
    expect(timelineSource).toContain('text-[12.5px] font-semibold text-quiet-text-primary');
    expect(timelineSource).toContain('text-sm font-medium text-quiet-text-tertiary');
    expect(timelineSource).toContain('<EpicColorSwatch');
    expect(timelineSource).not.toContain('rounded-lg border');
    expect(timelineSource).not.toContain('bg-muted/30');
    expect(barSource).toContain('backgroundColor: resolveEpicColor(entity.color)');
    expect(barSource).toContain('color: getEpicBadgeTextColor(entity.color)');
    expect(roadmapSource).toContain('<EpicColorSwatch color={epic.color}');
    expect(barSource).toContain('h-0.5');
    expect(barSource).toContain('min-w-[260px]');
    expect(barSource).toContain('shadow-lg');
  });
});
