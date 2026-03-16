import { useMemo } from 'react';
import {
  parseISO,
  startOfMonth,
  endOfMonth,
  addMonths,
  subMonths,
  differenceInCalendarDays,
  eachMonthOfInterval,
  format,
  startOfDay,
} from 'date-fns';
import { RoadmapEpicBar } from '@/components/pm/RoadmapEpicBar';
import type { RoadmapEpic, Objective } from '@/lib/pmTypes';

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

function getScheduledEpics(epics: RoadmapEpic[]) {
  return epics.filter((e) => e.epic.planned_start_date && e.epic.deadline);
}

function buildGroups(
  epics: RoadmapEpic[],
  groupBy: GroupBy,
  objectives: Objective[],
  teamNameMap: Map<string, string>,
): TimelineGroup[] {
  const scheduled = getScheduledEpics(epics);

  if (groupBy === 'epic') {
    return scheduled.map((epic) => ({
      id: epic.epic.id,
      name: epic.epic.name,
      epics: [epic],
    }));
  }

  if (groupBy === 'objective') {
    const objMap = new Map<string, RoadmapEpic[]>();
    const noObj: RoadmapEpic[] = [];

    for (const e of scheduled) {
      if (e.objectives.length === 0) {
        noObj.push(e);
      } else {
        for (const o of e.objectives) {
          const arr = objMap.get(o.id) ?? [];
          arr.push(e);
          objMap.set(o.id, arr);
        }
      }
    }

    const groups: TimelineGroup[] = [];
    for (const obj of objectives) {
      const items = objMap.get(obj.id);
      if (items && items.length > 0) {
        groups.push({ id: obj.id, name: obj.name, epics: items });
      }
    }
    if (noObj.length > 0) {
      groups.push({ id: '__none__', name: 'No objective', epics: noObj });
    }
    return groups;
  }

  // Group by team
  const teamMap = new Map<string, RoadmapEpic[]>();
  const noTeam: RoadmapEpic[] = [];

  for (const e of scheduled) {
    const tid = e.epic.team_id;
    if (!tid) {
      noTeam.push(e);
    } else {
      const arr = teamMap.get(tid) ?? [];
      arr.push(e);
      teamMap.set(tid, arr);
    }
  }

  const groups: TimelineGroup[] = [];
  for (const [tid, items] of teamMap) {
    groups.push({ id: tid, name: teamNameMap.get(tid) ?? 'Unknown team', epics: items });
  }
  groups.sort((a, b) => a.name.localeCompare(b.name));
  if (noTeam.length > 0) {
    groups.push({ id: '__none__', name: 'No team', epics: noTeam });
  }
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
  const scheduled = useMemo(() => getScheduledEpics(epics), [epics]);

  const { rangeStart, rangeEnd, months } = useMemo(() => {
    if (scheduled.length === 0) {
      const now = new Date();
      const s = startOfMonth(subMonths(now, 1));
      const e = endOfMonth(addMonths(now, 4));
      return { rangeStart: s, rangeEnd: e, months: eachMonthOfInterval({ start: s, end: e }) };
    }

    let minDate = new Date('2099-01-01');
    let maxDate = new Date('2000-01-01');

    for (const e of scheduled) {
      const s = parseISO(e.epic.planned_start_date!);
      const d = parseISO(e.epic.deadline!);
      if (s < minDate) minDate = s;
      if (d > maxDate) maxDate = d;
    }

    const s = startOfMonth(subMonths(minDate, 1));
    const e = endOfMonth(addMonths(maxDate, 1));
    return { rangeStart: s, rangeEnd: e, months: eachMonthOfInterval({ start: s, end: e }) };
  }, [scheduled]);

  const totalDays = differenceInCalendarDays(rangeEnd, rangeStart) || 1;

  const groups = useMemo(
    () => buildGroups(epics, groupBy, objectives, teamNameMap),
    [epics, groupBy, objectives, teamNameMap],
  );

  const todayPct = useMemo(() => {
    const today = startOfDay(new Date());
    const pct = (differenceInCalendarDays(today, rangeStart) / totalDays) * 100;
    return pct >= 0 && pct <= 100 ? pct : null;
  }, [rangeStart, totalDays]);

  // Pre-compute month grid line positions (cumulative % at the end of each month)
  const monthGridLines = useMemo(() => {
    let cumDays = 0;
    return months.map((month) => {
      cumDays += differenceInCalendarDays(endOfMonth(month), startOfMonth(month)) + 1;
      return (cumDays / totalDays) * 100;
    });
  }, [months, totalDays]);

  const monthWidthPx = zoom === 'month' ? 140 : 90;
  const totalWidthPx = months.length * monthWidthPx;
  const isEpicGrouping = groupBy === 'epic';

  function getBarPosition(epic: RoadmapEpic) {
    const s = parseISO(epic.epic.planned_start_date!);
    const d = parseISO(epic.epic.deadline!);
    const left = (differenceInCalendarDays(s, rangeStart) / totalDays) * 100;
    const width = (differenceInCalendarDays(d, s) / totalDays) * 100;
    return { left, width: Math.max(width, 1) };
  }

  if (groups.length === 0) {
    return (
      <div className="flex items-center justify-center py-16 text-sm text-muted-foreground">
        No scheduled epics to display. Add start and target dates to place epics on the timeline.
      </div>
    );
  }

  return (
    <div className="overflow-x-auto rounded-lg border border-border">
      <div className="relative" style={{ minWidth: `${totalWidthPx}px` }}>
        {/* Month header */}
        <div className="flex sticky top-0 z-20 bg-background border-b border-border">
          <div className="w-52 shrink-0 sticky left-0 z-30 bg-background border-r border-border px-3 py-2">
            <span className="text-[11px] font-medium uppercase tracking-wide text-muted-foreground">
              {groupBy === 'objective' ? 'Objective' : groupBy === 'team' ? 'Team' : 'Epic'}
            </span>
          </div>
          <div className="flex flex-1">
            {months.map((month) => {
              const monthDays = differenceInCalendarDays(
                endOfMonth(month),
                startOfMonth(month),
              ) + 1;
              const widthPct = (monthDays / totalDays) * 100;
              const now = new Date();
              const isCurrentMonth = month.getMonth() === now.getMonth() && month.getFullYear() === now.getFullYear();
              return (
                <div
                  key={month.toISOString()}
                  className={`border-r border-border/40 px-2 py-2 text-center ${isCurrentMonth ? 'bg-primary/5' : ''}`}
                  style={{ width: `${widthPct}%` }}
                >
                  <span className="text-[11px] text-muted-foreground font-medium">
                    {zoom === 'quarter'
                      ? format(month, month.getMonth() === 0 ? 'MMM yyyy' : 'MMM')
                      : format(month, 'MMM yyyy')}
                  </span>
                </div>
              );
            })}
          </div>
        </div>

        {/* Groups */}
        {groups.map((group) => (
          <div key={group.id}>
            {!isEpicGrouping && (
              <div className="flex border-b border-border bg-muted/30">
                <div className="w-52 shrink-0 sticky left-0 z-10 bg-muted/30 border-r border-border px-3 py-1.5 flex items-center gap-1.5">
                  <span className="text-[13px] font-medium truncate">{group.name}</span>
                  <span className="text-[11px] text-muted-foreground/60 tabular-nums shrink-0">
                    {group.epics.length}
                  </span>
                </div>
                <div className="flex-1" />
              </div>
            )}

            {/* Epic rows */}
            {group.epics.map((epic) => {
              const { left, width } = getBarPosition(epic);
              return (
                <div key={`${group.id}-${epic.epic.id}`} className="flex border-b border-border/40">
                  <div className="w-52 shrink-0 sticky left-0 z-10 bg-background border-r border-border px-3 py-1.5 flex items-center min-w-0">
                    <span
                      className={`truncate ${isEpicGrouping ? 'text-[13px] font-medium text-foreground' : 'text-[12px] text-muted-foreground'}`}
                    >
                      {isEpicGrouping ? group.name : epic.epic.name}
                    </span>
                  </div>
                  <div className="flex-1 relative h-9">
                    {/* Month grid lines */}
                    {monthGridLines.map((linePct, i) => (
                      <div
                        key={i}
                        className="absolute top-0 bottom-0 border-r border-border/20"
                        style={{ left: `${linePct}%` }}
                      />
                    ))}
                    <RoadmapEpicBar
                      epic={epic}
                      left={left}
                      width={width}
                      slug={slug}
                      memberNameMap={memberNameMap}
                    />
                  </div>
                </div>
              );
            })}
          </div>
        ))}

        {/* Today line */}
        {todayPct !== null && (
          <div
            className="absolute top-0 bottom-0 w-px bg-red-500/70 z-10 pointer-events-none"
            style={{ left: `${todayPct}%` }}
          >
            <div className="absolute top-0 left-1/2 -translate-x-1/2 flex flex-col items-center">
              <span className="text-[9px] font-semibold text-red-500 bg-background/90 px-1 rounded leading-tight">
                Today
              </span>
              <div className="w-1.5 h-1.5 rounded-full bg-red-500 mt-0.5" />
            </div>
          </div>
        )}
      </div>
    </div>
  );
}
