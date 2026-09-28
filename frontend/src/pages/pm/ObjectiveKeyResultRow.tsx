import { QuietTextAction } from '@/components/design-system/quiet';
import { QuickTooltip } from '@/components/ui/quick-tooltip';
import { PencilEdit02Icon } from '@/lib/icons';
import type { KeyResult } from '@/lib/pmTypes';
import { KeyResultHistory } from './KeyResultHistory';
import { keyResultHealth } from './keyResultHealth';

const numberFormat = new Intl.NumberFormat(undefined, { maximumFractionDigits: 20 });
const tones = { neutral: 'text-quiet-text-tertiary', positive: 'text-quiet-positive', warning: 'text-amber-700 dark:text-amber-400', negative: 'text-destructive' };

export function ObjectiveKeyResultRow({ kr, workspaceId, memberMap, onEdit, onLog, readOnly, startDate, deadline }: {
  kr: KeyResult;
  workspaceId: string;
  memberMap: Map<string, string>;
  onEdit: () => void;
  onLog: () => void;
  readOnly?: boolean;
  startDate?: string;
  deadline?: string;
}) {
  const isCompletion = kr.result_type === 'boolean';
  const suffix = kr.result_type === 'percent' ? '%' : '';
  const value = (number: number) => `${numberFormat.format(number)}${suffix}`;
  const progress = kr.progress > 0 && kr.progress < 1 ? '<1' : kr.progress < 100 ? Math.min(99, Math.round(kr.progress)) : 100;
  const health = keyResultHealth(kr.progress, startDate, deadline, new Date(), isCompletion);
  return <li className="group grid min-w-0 gap-x-5 gap-y-3 border-b border-quiet-divider-light py-4 hover:bg-quiet-row-hover lg:grid-cols-[minmax(0,1fr)_minmax(12rem,17rem)]">
    <div className="min-w-0">
      <p className="break-words text-sm font-semibold leading-6 text-quiet-text-primary">{kr.name}</p>
      <QuickTooltip label={health.explanation}>
        <span tabIndex={0} className={`mt-1 inline-flex flex-wrap items-baseline gap-2 text-xs leading-5 focus-visible:outline-2 focus-visible:outline-quiet-text-secondary ${tones[health.tone]}`}>
          <span className="font-semibold tabular-nums">{progress}% achieved</span>
          {health.label && <><span aria-hidden="true">·</span><span>{health.label}</span></>}
        </span>
      </QuickTooltip>
      <KeyResultHistory kr={kr} workspaceId={workspaceId} memberMap={memberMap} />
    </div>
    <div className="relative min-w-0 self-start">
    {isCompletion ? <dl className="self-start text-sm lg:text-right"><dt className="text-xs text-quiet-text-tertiary">Current</dt><dd className="mt-1 font-medium">{kr.progress >= 100 ? 'Complete' : 'Incomplete'}</dd></dl> :
      <dl className="grid grid-cols-3 gap-4 self-start tabular-nums lg:text-right">
        <div><dt className="text-xs text-quiet-text-tertiary">At start</dt><dd className="mt-1 break-words text-base text-quiet-text-tertiary">{value(kr.initial_value)}</dd></div>
        <div><dt className="text-xs text-quiet-text-tertiary">Current</dt><dd className="mt-1 break-words text-base font-medium text-quiet-text-primary">{value(kr.current_value)}</dd></div>
        <div><dt className="text-xs text-quiet-text-tertiary">Target</dt><dd className="mt-1 break-words text-base font-semibold text-quiet-text-primary">{value(kr.target_value)}</dd></div>
      </dl>}
    {!readOnly && <div className="mt-2 flex items-center gap-3 transition-opacity lg:absolute lg:right-0 lg:top-full lg:pointer-events-none lg:opacity-0 lg:group-hover:pointer-events-auto lg:group-hover:opacity-100 lg:group-focus-within:pointer-events-auto lg:group-focus-within:opacity-100 [@media(hover:none)]:pointer-events-auto [@media(hover:none)]:opacity-100">
      <QuietTextAction aria-label={`Log result for ${kr.name}`} onClick={onLog}>Log result</QuietTextAction>
      <QuickTooltip label="Edit key result"><QuietTextAction aria-label={`Edit key result ${kr.name}`} onClick={onEdit}><PencilEdit02Icon className="size-3.5" /></QuietTextAction></QuickTooltip>
    </div>}
    </div>
  </li>;
}
