import {
  Ban,
  CircleAlert,
  CircleCheck,
  CircleDashed,
  CircleDot,
  Minus,
  SignalHigh,
  SignalLow,
  SignalMedium,
} from 'lucide-react';
import type { Priority, StateType } from './pmTypes';

// ── Priority icons & colors ────────────────────────────────────────

export const PRIORITY_CONFIG: Record<
  Priority,
  { icon: React.ElementType; color: string; label: string }
> = {
  urgent: { icon: CircleAlert, color: 'text-red-500', label: 'Urgent' },
  high: { icon: SignalHigh, color: 'text-orange-500', label: 'High' },
  medium: { icon: SignalMedium, color: 'text-amber-500', label: 'Medium' },
  low: { icon: SignalLow, color: 'text-sky-500', label: 'Low' },
  none: { icon: Ban, color: 'text-zinc-400', label: 'None' },
};

export function PriorityIcon({
  priority,
  className = 'h-4 w-4',
}: {
  priority: Priority;
  className?: string;
}) {
  const config = PRIORITY_CONFIG[priority];
  const Icon = config.icon;
  return <Icon className={`${className} ${config.color}`} />;
}

// ── Workflow state icons & colors ──────────────────────────────────

export const STATE_TYPE_ICON_CONFIG: Record<
  StateType,
  { icon: React.ElementType; color: string }
> = {
  backlog: { icon: CircleDashed, color: 'text-zinc-400' },
  unstarted: { icon: Minus, color: 'text-zinc-400' },
  started: { icon: CircleDot, color: 'text-amber-500' },
  done: { icon: CircleCheck, color: 'text-green-500' },
};

export function StateTypeIcon({
  stateType,
  className = 'h-4 w-4',
}: {
  stateType: StateType;
  className?: string;
}) {
  const config = STATE_TYPE_ICON_CONFIG[stateType];
  const Icon = config.icon;
  return <Icon className={`${className} ${config.color}`} />;
}
