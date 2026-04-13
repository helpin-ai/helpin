/* eslint-disable react-refresh/only-export-components */
import {
  ArrowReloadHorizontalIcon,
  PriorityIcon as PMPriorityIcon,
  SeverityIcon as PMSeverityIcon,
  StateTypeIcon as PMStateTypeIcon,
  TaskTypeIcon as PMTaskTypeIcon,
} from '@/lib/pmIcons';
import type { ObjectiveState, Priority, Severity, SprintStatus, StateType, TaskType } from './pmTypes';

// ── Priority icons & colors ────────────────────────────────────────

export const PRIORITY_CONFIG: Record<
  Priority,
  { color: string; label: string }
> = {
  urgent: { color: 'text-red-500', label: 'Urgent' },
  high: { color: 'text-orange-500', label: 'High' },
  medium: { color: 'text-amber-500', label: 'Medium' },
  low: { color: 'text-sky-500', label: 'Low' },
  none: { color: 'text-zinc-400', label: 'None' },
};

export const PRIORITY_BORDER_COLOR: Record<Priority, string> = {
  urgent: 'border-red-400 dark:border-red-600',
  high: 'border-orange-400 dark:border-orange-600',
  medium: 'border-amber-400 dark:border-amber-600',
  low: 'border-sky-400 dark:border-sky-600',
  none: 'border-border',
};

export function PriorityIcon({
  priority,
  className = 'h-4 w-4',
}: {
  priority: Priority;
  className?: string;
}) {
  return <PMPriorityIcon priority={priority} className={className} />;
}

// ── Severity icons & colors ─────────────────────────────────────────

export const SEVERITY_CONFIG: Record<
  Severity,
  { color: string; label: string }
> = {
  critical: { color: 'text-red-600', label: 'Critical' },
  major: { color: 'text-orange-500', label: 'Major' },
  minor: { color: 'text-amber-500', label: 'Minor' },
  none: { color: 'text-zinc-400', label: 'None' },
};

export function SeverityIcon({
  severity,
  className = 'h-4 w-4',
}: {
  severity: Severity;
  className?: string;
}) {
  return <PMSeverityIcon severity={severity} className={className} />;
}

// ── Workflow state icons & colors ──────────────────────────────────

export const STATE_TYPE_ICON_CONFIG: Record<
  StateType,
  { color: string }
> = {
  backlog: { color: 'text-zinc-400' },
  unstarted: { color: 'text-zinc-400' },
  started: { color: 'text-amber-500' },
  done: { color: 'text-green-500' },
};

export function StateTypeIcon({
  stateType,
  className = 'h-4 w-4',
}: {
  stateType: StateType;
  className?: string;
}) {
  return <PMStateTypeIcon stateType={stateType} className={className} />;
}

// ── Sprint icon ───────────────────────────────────────────────────

export function SprintIcon({
  className = 'h-4 w-4 text-muted-foreground',
}: {
  className?: string;
}) {
  return <ArrowReloadHorizontalIcon className={className} />;
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

// ── Task type icons & colors ──────────────────────────────────────

export const TASK_TYPE_CONFIG: Record<
  TaskType,
  { color: string; label: string }
> = {
  feature: { color: 'text-amber-500', label: 'Feature' },
  bug: { color: 'text-red-500', label: 'Bug' },
  chore: { color: 'text-indigo-500', label: 'Chore' },
};

/** @deprecated Use TASK_TYPE_CONFIG */
export const STORY_TYPE_CONFIG = TASK_TYPE_CONFIG;

export function TaskTypeIcon({
  taskType,
  className = 'h-4 w-4',
}: {
  taskType: TaskType;
  className?: string;
}) {
  return <PMTaskTypeIcon taskType={taskType} className={className} />;
}

/** @deprecated Use TaskTypeIcon */
export function StoryTypeIcon({
  storyType,
  className = 'h-4 w-4',
}: {
  storyType: TaskType;
  className?: string;
}) {
  return <TaskTypeIcon taskType={storyType} className={className} />;
}
