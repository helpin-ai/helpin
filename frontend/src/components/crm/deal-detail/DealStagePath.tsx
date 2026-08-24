import { useEffect, useMemo, useRef, useState, type CSSProperties } from 'react';
import { Tick01Icon } from '@/lib/icons';
import { cn } from '@/lib/utils';
import type { CRMPipelineStage } from '@/lib/crmTypes';

interface DealStagePathProps {
  stages: CRMPipelineStage[];
  value: string;
  onChange: (stageId: string) => void;
  disabled?: boolean;
}

function activeStageClass(stage: CRMPipelineStage) {
  if (stage.stage_type === 'won') return 'bg-emerald-600 text-white';
  if (stage.stage_type === 'lost') return 'bg-destructive text-destructive-foreground';
  return 'bg-foreground text-background';
}

export function DealStagePath({ stages, value, onChange, disabled }: DealStagePathProps) {
  const activeIndex = stages.findIndex((stage) => stage.id === value);
  const pathRef = useRef<HTMLDivElement>(null);
  const [columnCount, setColumnCount] = useState(1);
  const stageColumnWidth = useMemo(() => {
    const longestStageName = stages.reduce((longest, stage) => Math.max(longest, stage.name.length), 0);
    return Math.max(132, Math.min(205, longestStageName * 6 + 40));
  }, [stages]);

  useEffect(() => {
    const path = pathRef.current;
    if (!path) return;
    const updateColumnCount = (width: number) => {
      const gap = 20;
      const nextCount = Math.max(1, Math.min(stages.length, Math.floor((width + gap) / (stageColumnWidth + gap))));
      setColumnCount((current) => current === nextCount ? current : nextCount);
    };
    updateColumnCount(path.getBoundingClientRect().width);
    if (typeof ResizeObserver === 'undefined') return;
    const observer = new ResizeObserver(([entry]) => updateColumnCount(entry.contentRect.width));
    observer.observe(path);
    return () => observer.disconnect();
  }, [stageColumnWidth, stages.length]);

  const stageRows = useMemo(() => {
    const rows: CRMPipelineStage[][] = [];
    for (let index = 0; index < stages.length; index += columnCount) {
      rows.push(stages.slice(index, index + columnCount));
    }
    return rows;
  }, [columnCount, stages]);

  const rowStyle = {
    '--stage-columns': columnCount,
    gridTemplateColumns: `repeat(${columnCount}, minmax(0, 1fr))`,
    maxWidth: columnCount * stageColumnWidth + (columnCount - 1) * 20,
  } as CSSProperties;

  return (
    <section aria-label="Pipeline stage" className="border-b border-border/60 px-4 py-4 sm:px-6 lg:px-10">
      <div className="mb-3 flex items-center justify-between gap-3">
        <h2 className="text-[11px] font-semibold uppercase tracking-[0.1em] text-muted-foreground">
          Pipeline stage
        </h2>
        <span className="truncate text-xs text-muted-foreground">
          {stages[activeIndex]?.name ?? 'No stage selected'}
        </span>
      </div>
      <div ref={pathRef} className="w-full pb-1" role="radiogroup" aria-label="Deal stage">
        <div className="flex flex-col gap-6">
          {stageRows.map((row, rowIndex) => {
            const hasNextRow = rowIndex < stageRows.length - 1;
            return (
              <div key={row[0]?.id ?? rowIndex} className="relative mx-auto grid min-h-9 w-full items-center gap-x-5" style={rowStyle}>
                {row.map((stage, indexInRow) => {
                  const stageIndex = rowIndex * columnCount + indexInRow;
                  const active = stage.id === value;
                  const complete = activeIndex >= 0 && stageIndex < activeIndex;
                  return (
                    <div key={stage.id} className="relative flex min-w-0 justify-center">
                      {indexInRow > 0 ? (
                        <span aria-hidden="true" className="absolute right-1/2 top-1/2 h-px w-[calc(100%+1.25rem)] -translate-y-1/2 bg-border" />
                      ) : null}
                      <button
                        type="button"
                        role="radio"
                        aria-checked={active}
                        disabled={disabled}
                        className={cn(
                          'relative z-10 inline-flex min-h-9 max-w-full items-center justify-center gap-1.5 whitespace-nowrap rounded-full bg-background px-3 py-1.5 text-center text-xs font-medium leading-4 transition-[background-color,color,opacity] focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 disabled:cursor-not-allowed disabled:opacity-60',
                          active
                            ? activeStageClass(stage)
                            : complete
                              ? 'bg-muted text-foreground hover:bg-muted/80'
                              : 'text-muted-foreground hover:bg-muted/60 hover:text-foreground',
                        )}
                        onClick={() => onChange(stage.id)}
                      >
                        {complete ? <Tick01Icon className="h-3 w-3 shrink-0" /> : (
                          <span className={cn('h-1.5 w-1.5 shrink-0 rounded-full bg-current opacity-45', active && 'opacity-90')} />
                        )}
                        <span className="min-w-0 truncate">{stage.name}</span>
                      </button>
                      {hasNextRow && indexInRow === row.length - 1 && columnCount > 1 ? (
                        <span aria-hidden="true" className="absolute left-1/2 right-0 top-1/2 h-px -translate-y-1/2 bg-border" />
                      ) : null}
                    </div>
                  );
                })}
                {hasNextRow && columnCount > 1 ? (
                  <>
                    <span aria-hidden="true" className="absolute -bottom-[12px] right-0 top-1/2 w-px bg-border" />
                    <span
                      aria-hidden="true"
                      className="absolute -bottom-[12px] right-0 h-px bg-border"
                      style={{ left: `calc((100% - ${(columnCount - 1) * 20}px) / ${columnCount} / 2)` }}
                    />
                    <span
                      aria-hidden="true"
                      className="absolute -bottom-[42px] top-[calc(100%+12px)] w-px bg-border"
                      style={{ left: `calc((100% - ${(columnCount - 1) * 20}px) / ${columnCount} / 2)` }}
                    />
                  </>
                ) : null}
                {hasNextRow && columnCount === 1 ? (
                  <span aria-hidden="true" className="absolute -bottom-[42px] left-1/2 top-1/2 w-px -translate-x-1/2 bg-border" />
                ) : null}
              </div>
            );
          })}
        </div>
      </div>
    </section>
  );
}
