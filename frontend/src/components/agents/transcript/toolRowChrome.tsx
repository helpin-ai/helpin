import type { ReactNode } from 'react';
import { Cancel01Icon, Loading01Icon, Tick01Icon } from '@/lib/icons';
import type { CodingSessionLiveToolCall } from '@/lib/pmTypes';

export interface ToolRowChrome {
  icon: ReactNode;
  /** Tint applied to the status glyph. */
  className: string;
}

/**
 * Status-led chrome for a one-line tool row — running spinner, success tick, or
 * failure cross. The action verb itself lives in the row label (via
 * `describeToolCall`), so the icon only conveys run state.
 */
export function toolStatusChrome(status: CodingSessionLiveToolCall['status']): ToolRowChrome {
  if (status === 'running') {
    return { icon: <Loading01Icon className="h-3 w-3 animate-spin" />, className: 'text-orange-500' };
  }
  if (status === 'failed') {
    return { icon: <Cancel01Icon className="h-3 w-3" />, className: 'text-destructive' };
  }
  return { icon: <Tick01Icon className="h-3 w-3" />, className: 'text-emerald-600 dark:text-emerald-400' };
}

/** Compact duration label (e.g. `420ms`, `3s`) or undefined when not measured. */
export function formatToolDuration(ms?: number): string | undefined {
  if (!ms || ms <= 0) return undefined;
  return ms >= 1000 ? `${Math.round(ms / 1000)}s` : `${ms}ms`;
}
