import { useCallback, useMemo, useRef, useState } from 'react';
import {
  addMonths,
  addQuarters,
  differenceInCalendarDays,
  eachMonthOfInterval,
  endOfMonth,
  endOfQuarter,
  format,
  startOfDay,
  startOfMonth,
  startOfQuarter,
  subMonths,
  subQuarters,
} from 'date-fns';

import { EpicColorSwatch } from '@/components/pm/EpicColorSwatch';
import { RoadmapEpicBar } from '@/components/pm/RoadmapEpicBar';
import { buildRoadmapQuarterSegments, getRoadmapEpicRange, getScheduledRoadmapEpics } from '@/components/pm/roadmapUtils';
import { Target01Icon, UserGroupIcon } from '@/lib/icons';
import type { Objective, RoadmapEpic } from '@/lib/pmTypes';

type GroupBy = 'objective' | 'team' | 'epic';
type Zoom = 'month' | 'quarter';

interface TimelineGroup {
  id: string;
  name: string;
  epics: RoadmapEpic[];
}

interface RoadmapTimelineProps {
  epics: RoadmapEpic[];
  objectives: Objective[];
  groupBy: GroupBy;
  zoom: Zoom;
  slug: string;
  teamNameMap: Map<string, string>;
  memberNameMap: Map<string, string>;
}

function buildGroups(
  epics: RoadmapEpic[],
  groupBy: GroupBy,
  objectives: Objective[],
  teamNameMap: Map<string, string>,
): TimelineGroup[] {
  const scheduled = getScheduledRoadmapEpics(epics);

  if (groupBy === 'epic') {
    return scheduled.map((epic) => ({ id: epic.epic.id, name: epic.epic.name, epics: [epic] }));
  }

  if (groupBy === 'objective') {
    const grouped = new Map<string, RoadmapEpic[]>();
    const unassigned: RoadmapEpic[] = [];
    scheduled.forEach((epic) => {
      if (epic.objectives.length === 0) {
        unassigned.push(epic);
        return;
      }
      epic.objectives.forEach((objective) => {
        const entries = grouped.get(objective.id) ?? [];
        entries.push(epic);
        grouped.set(objective.id, entries);
      });
    });

    const groups = objectives.flatMap((objective) => {
      const entries = grouped.get(objective.id);
      return entries?.length ? [{ id: objective.id, name: objective.name, epics: entries }] : [];
    });
    if (unassigned.length > 0) groups.push({ id: '__none__', name: 'No objective', epics: unassigned });
    return groups;
  }

  const grouped = new Map<string, RoadmapEpic[]>();
  const unassigned: RoadmapEpic[] = [];
  scheduled.forEach((epic) => {
    const teamId = epic.epic.team_id;
    if (!teamId) {
      unassigned.push(epic);
      return;
    }
    const entries = grouped.get(teamId) ?? [];
    entries.push(epic);
    grouped.set(teamId, entries);
  });

  const groups = Array.from(grouped.entries())
    .map(([id, entries]) => ({ id, name: teamNameMap.get(id) ?? 'Unknown team', epics: entries }))
    .sort((left, right) => left.name.localeCompare(right.name));
  if (unassigned.length > 0) groups.push({ id: '__none__', name: 'No team', epics: unassigned });
  return groups;
}

export function RoadmapTimeline({
  epics,
  objectives,
  groupBy,
  zoom,
  slug,
  teamNameMap,
  memberNameMap,
}: RoadmapTimelineProps) {
  const scheduled = useMemo(() => getScheduledRoadmapEpics(epics), [epics]);
  const { rangeStart, rangeEnd, months } = useMemo(() => {
    if (scheduled.length === 0) {
      const now = new Date();
      const start = zoom === 'quarter'
        ? startOfQuarter(subQuarters(now, 1))
        : startOfMonth(subMonths(now, 1));
      const end = zoom === 'quarter'
        ? endOfQuarter(addQuarters(now, 3))
        : endOfMonth(addMonths(now, 4));
      return { rangeStart: start, rangeEnd: end, months: eachMonthOfInterval({ start, end }) };
    }

    const ranges = scheduled
      .map(getRoadmapEpicRange)
      .filter((range): range is NonNullable<ReturnType<typeof getRoadmapEpicRange>> => range !== null);
    const earliest = new Date(Math.min(...ranges.map((range) => range.start.getTime())));
    const latest = new Date(Math.max(...ranges.map((range) => range.target.getTime())));
    const start = zoom === 'quarter'
      ? startOfQuarter(subQuarters(earliest, 1))
      : startOfMonth(subMonths(earliest, 1));
    const end = zoom === 'quarter'
      ? endOfQuarter(addQuarters(latest, 1))
      : endOfMonth(addMonths(latest, 1));
    return { rangeStart: start, rangeEnd: end, months: eachMonthOfInterval({ start, end }) };
  }, [scheduled, zoom]);

  const totalDays = Math.max(differenceInCalendarDays(rangeEnd, rangeStart) + 1, 1);
  const groups = useMemo(
    () => buildGroups(epics, groupBy, objectives, teamNameMap),
    [epics, groupBy, objectives, teamNameMap],
  );
  const todayPct = useMemo(() => {
    const percent = (differenceInCalendarDays(startOfDay(new Date()), rangeStart) / totalDays) * 100;
    return percent >= 0 && percent <= 100 ? percent : null;
  }, [rangeStart, totalDays]);
  const monthGridLines = useMemo(() => {
    return months.map((month) => {
      const elapsedDays = differenceInCalendarDays(endOfMonth(month), rangeStart) + 1;
      return {
        position: (elapsedDays / totalDays) * 100,
        quarterBoundary: month.getMonth() % 3 === 2,
      };
    });
  }, [months, rangeStart, totalDays]);
  const quarterSegments = useMemo(() => buildRoadmapQuarterSegments(months), [months]);

  const monthWidth = zoom === 'month' ? 140 : 72;
  const timelineWidth = months.length * monthWidth;
  const isEpicGrouping = groupBy === 'epic';
  const GroupIcon = groupBy === 'objective' ? Target01Icon : UserGroupIcon;
  const [labelWidth, setLabelWidth] = useState(220);
  const dragRef = useRef<{ startX: number; startWidth: number } | null>(null);

  const onResizeStart = useCallback((event: React.PointerEvent<HTMLButtonElement>) => {
    event.preventDefault();
    dragRef.current = { startX: event.clientX, startWidth: labelWidth };
    event.currentTarget.setPointerCapture(event.pointerId);
  }, [labelWidth]);
  const onResizeMove = useCallback((event: React.PointerEvent<HTMLButtonElement>) => {
    if (!dragRef.current) return;
    const next = dragRef.current.startWidth + event.clientX - dragRef.current.startX;
    setLabelWidth(Math.max(176, Math.min(420, next)));
  }, []);
  const onResizeEnd = useCallback((event: React.PointerEvent<HTMLButtonElement>) => {
    dragRef.current = null;
    if (event.currentTarget.hasPointerCapture(event.pointerId)) {
      event.currentTarget.releasePointerCapture(event.pointerId);
    }
  }, []);

  const labelStyle = { width: labelWidth, minWidth: labelWidth };
  // Keep the page surface visible while preventing scrolled content from
  // painting underneath the transparent, sticky label column.
  const timelineClipStyle = { clipPath: 'inset(0 0 0 var(--roadmap-scroll-left, 0px))' };
  const getBarPosition = (epic: RoadmapEpic) => {
    const range = getRoadmapEpicRange(epic);
    if (!range) return { left: 0, width: 0 };
    const left = (differenceInCalendarDays(range.start, rangeStart) / totalDays) * 100;
    const duration = differenceInCalendarDays(range.target, range.start) + 1;
    return { left, width: Math.max((duration / totalDays) * 100, 1) };
  };

  if (groups.length === 0) {
    return (
      <div className="border-y border-quiet-divider-strong px-4 py-8 text-left sm:px-6">
        <p className="text-[14px] font-medium text-quiet-text-primary">Nothing is scheduled yet</p>
        <p className="mt-1 max-w-[620px] text-sm leading-[1.6] text-quiet-text-tertiary">
          Add both a start and target date in the planning queue above. The epic will move here automatically.
        </p>
      </div>
    );
  }

  return (
    <div
      className="overflow-x-auto border-y border-quiet-divider-strong [scrollbar-gutter:stable]"
      onScroll={(event) => {
        event.currentTarget.style.setProperty('--roadmap-scroll-left', `${event.currentTarget.scrollLeft}px`);
      }}
    >
      <div className="relative" style={{ minWidth: `${labelWidth + timelineWidth}px` }}>
        <div className="sticky top-0 z-20 flex border-b border-quiet-divider-strong">
          <div className="sticky left-0 z-30 flex shrink-0 items-center border-r border-quiet-divider-strong px-3 py-2.5" style={labelStyle}>
            <span className="text-[11.5px] font-semibold uppercase tracking-[0.06em] text-quiet-muted">
              {groupBy === 'objective' ? 'Objective' : groupBy === 'team' ? 'Team' : 'Epic'}
            </span>
            <button
              type="button"
              aria-label="Resize roadmap label column"
              className="absolute inset-y-0 right-[-4px] z-10 w-2 cursor-col-resize touch-none bg-transparent focus-visible:outline-2 focus-visible:outline-quiet-text-primary"
              onPointerDown={onResizeStart}
              onPointerMove={onResizeMove}
              onPointerUp={onResizeEnd}
              onPointerCancel={onResizeEnd}
            >
              <span className="absolute inset-y-0 left-1/2 w-px bg-transparent transition-colors hover:bg-quiet-text-primary" />
            </button>
          </div>
          <div className="flex min-w-0 flex-1 flex-col" style={timelineClipStyle}>
            {zoom === 'quarter' ? (
              <div className="flex border-b border-quiet-divider-light">
                {quarterSegments.map((segment) => {
                  const days = segment.months.reduce(
                    (total, month) => total + differenceInCalendarDays(endOfMonth(month), startOfMonth(month)) + 1,
                    0,
                  );
                  return (
                    <div
                      key={segment.key}
                      className="border-r border-quiet-divider-strong px-2 py-1.5 text-center"
                      style={{ width: `${(days / totalDays) * 100}%` }}
                    >
                      <span className="text-[12.5px] font-semibold text-quiet-text-primary">{segment.label}</span>
                    </div>
                  );
                })}
              </div>
            ) : null}
            <div className="flex flex-1">
              {months.map((month) => {
                const monthDays = differenceInCalendarDays(endOfMonth(month), startOfMonth(month)) + 1;
                const width = (monthDays / totalDays) * 100;
                const now = new Date();
                const current = month.getMonth() === now.getMonth() && month.getFullYear() === now.getFullYear();
                return (
                  <div
                    key={month.toISOString()}
                    className={`border-r px-2 text-center ${zoom === 'quarter' ? 'border-quiet-divider-light py-1.5' : 'border-quiet-divider-light py-2.5'}`}
                    style={{ width: `${width}%` }}
                  >
                    <span className={`text-[11.5px] font-medium ${current ? 'text-quiet-text-primary' : 'text-quiet-muted'}`}>
                      {zoom === 'quarter' ? format(month, 'MMM') : format(month, 'MMM yyyy')}
                    </span>
                  </div>
                );
              })}
            </div>
          </div>
        </div>

        {groups.map((group, groupIndex) => (
          <div key={group.id} className={groupIndex < groups.length - 1 ? 'border-b border-quiet-divider-light' : undefined}>
            {!isEpicGrouping ? (
              <div className="flex border-b border-quiet-divider-strong">
                <div className="sticky left-0 z-10 flex shrink-0 items-center gap-2 border-r border-quiet-divider-strong px-3 py-1.5" style={labelStyle}>
                  <GroupIcon className="h-[14px] w-[14px] shrink-0 text-quiet-muted" />
                  <span className="min-w-0 truncate text-[12.5px] font-semibold text-quiet-text-primary">{group.name}</span>
                  <span className="shrink-0 text-[11.5px] tabular-nums text-quiet-muted">
                    {group.epics.length} {group.epics.length === 1 ? 'epic' : 'epics'}
                  </span>
                </div>
                <div className="flex-1" />
              </div>
            ) : null}

            {group.epics.map((epic) => {
              const position = getBarPosition(epic);
              return (
                <div key={`${group.id}-${epic.epic.id}`} className="flex border-b border-quiet-divider-light last:border-b-0">
                  <div className={`sticky left-0 z-10 flex min-w-0 shrink-0 items-center gap-2 border-r border-quiet-divider-strong py-2 ${isEpicGrouping ? 'px-3' : 'pl-7 pr-3'}`} style={labelStyle}>
                    <EpicColorSwatch color={epic.epic.color} />
                    <span className="truncate text-sm font-medium text-quiet-text-tertiary">
                      {isEpicGrouping ? group.name : epic.epic.name}
                    </span>
                  </div>
                  <div className="relative h-10 flex-1" style={timelineClipStyle}>
                    {monthGridLines.map((line, index) => (
                      <span
                        key={index}
                        aria-hidden="true"
                        className={`absolute inset-y-0 border-r ${zoom === 'quarter' && line.quarterBoundary ? 'border-quiet-divider-strong' : 'border-quiet-divider-light'}`}
                        style={{ left: `${line.position}%` }}
                      />
                    ))}
                    <RoadmapEpicBar
                      epic={epic}
                      left={position.left}
                      width={position.width}
                      slug={slug}
                      memberNameMap={memberNameMap}
                    />
                  </div>
                </div>
              );
            })}
          </div>
        ))}

        {todayPct !== null ? (
          <div className="pointer-events-none absolute inset-y-0 z-10" style={{ ...timelineClipStyle, left: labelWidth, right: 0 }}>
            <span className="absolute inset-y-0 w-px bg-quiet-accent" style={{ left: `${todayPct}%` }}>
              <span className="absolute left-1 top-1 whitespace-nowrap bg-background px-1 text-[10px] font-semibold uppercase tracking-[0.03em] text-quiet-accent">
                Today
              </span>
            </span>
          </div>
        ) : null}
      </div>
    </div>
  );
}
