import {
  Ban,
  Bug,
  ChevronsUp,
  CircleCheck,
  CircleDashed,
  CircleDot,
  Minus,
  OctagonAlert,
  ShieldAlert,
  SignalHigh,
  SignalLow,
  SignalMedium,
  Sparkles,
  TriangleAlert,
  Wrench,
} from 'lucide-react';
import type { ObjectiveState, Priority, Severity, SprintStatus, StateType, StoryType } from './pmTypes';

// ── Priority icons & colors ────────────────────────────────────────

export const PRIORITY_CONFIG: Record<
  Priority,
  { icon: React.ElementType; color: string; label: string; bold?: boolean }
> = {
  urgent: { icon: ChevronsUp, color: 'text-red-500', label: 'Urgent', bold: true },
  high: { icon: SignalHigh, color: 'text-orange-500', label: 'High', bold: true },
  medium: { icon: SignalMedium, color: 'text-amber-500', label: 'Medium', bold: true },
  low: { icon: SignalLow, color: 'text-sky-500', label: 'Low', bold: true },
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
  return <Icon className={`${className} ${config.color}`} {...(config.bold ? { strokeWidth: 3 } : {})} />;
}

// ── Severity icons & colors ─────────────────────────────────────────

export const SEVERITY_CONFIG: Record<
  Severity,
  { icon: React.ElementType; color: string; label: string }
> = {
  critical: { icon: OctagonAlert, color: 'text-red-600', label: 'Critical' },
  major: { icon: ShieldAlert, color: 'text-orange-500', label: 'Major' },
  minor: { icon: TriangleAlert, color: 'text-amber-500', label: 'Minor' },
  none: { icon: Ban, color: 'text-zinc-400', label: 'None' },
};

export function SeverityIcon({
  severity,
  className = 'h-4 w-4',
}: {
  severity: Severity;
  className?: string;
}) {
  const config = SEVERITY_CONFIG[severity];
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

// ── Sprint status config ──────────────────────────────────────────

export const SPRINT_STATUS_CONFIG: Record<
  SprintStatus,
  { label: string; color: string; badge: string }
> = {
  unstarted: {
    label: 'Not Started',
    color: 'text-zinc-400',
    badge: 'bg-zinc-100 text-zinc-600 dark:bg-zinc-800/30 dark:text-zinc-400',
  },
  started: {
    label: 'In Progress',
    color: 'text-amber-500',
    badge: 'bg-amber-100 text-amber-700 dark:bg-amber-900/30 dark:text-amber-400',
  },
  done: {
    label: 'Done',
    color: 'text-green-500',
    badge: 'bg-green-100 text-green-700 dark:bg-green-900/30 dark:text-green-400',
  },
};

// ── Objective state config ─────────────────────────────────────────

export const OBJECTIVE_STATE_CONFIG: Record<
  ObjectiveState,
  { label: string; color: string; badge: string }
> = {
  not_started: {
    label: 'Not Started',
    color: 'text-zinc-400',
    badge: 'bg-zinc-100 text-zinc-600 dark:bg-zinc-800/30 dark:text-zinc-400',
  },
  active: {
    label: 'In Progress',
    color: 'text-amber-500',
    badge: 'bg-amber-100 text-amber-700 dark:bg-amber-900/30 dark:text-amber-400',
  },
  closed: {
    label: 'Done',
    color: 'text-green-500',
    badge: 'bg-green-100 text-green-700 dark:bg-green-900/30 dark:text-green-400',
  },
};

// ── Story type icons & colors ──────────────────────────────────────

export const STORY_TYPE_CONFIG: Record<
  StoryType,
  { icon: React.ElementType; color: string; label: string }
> = {
  feature: { icon: Sparkles, color: 'text-amber-500', label: 'Feature' },
  bug: { icon: Bug, color: 'text-red-500', label: 'Bug' },
  chore: { icon: Wrench, color: 'text-indigo-500', label: 'Chore' },
};

export function StoryTypeIcon({
  storyType,
  className = 'h-4 w-4',
}: {
  storyType: StoryType;
  className?: string;
}) {
  const config = STORY_TYPE_CONFIG[storyType];
  const Icon = config.icon;
  return <Icon className={`${className} ${config.color}`} />;
}
