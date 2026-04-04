import {
  ArrowUpDoubleIcon,
  Bug01Icon,
  CancelCircleIcon,
  CheckmarkCircle02Icon,
  DashedLineCircleIcon,
  MinusSignIcon,
  OctagonIcon,
  RecordIcon,
  Shield02Icon,
  SignalFull01Icon,
  SignalLow01Icon,
  SignalMedium01Icon,
  SparklesIcon,
  ArrowReloadHorizontalIcon,
  Alert01Icon,
  Wrench01Icon,
} from '@/lib/icons';
import type { ObjectiveState, Priority, Severity, SprintStatus, StateType, TaskType } from './pmTypes';

// ── Priority icons & colors ────────────────────────────────────────

export const PRIORITY_CONFIG: Record<
  Priority,
  { icon: React.ElementType; color: string; label: string; bold?: boolean }
> = {
  urgent: { icon: ArrowUpDoubleIcon, color: 'text-red-500', label: 'Urgent', bold: true },
  high: { icon: SignalFull01Icon, color: 'text-orange-500', label: 'High', bold: true },
  medium: { icon: SignalMedium01Icon, color: 'text-amber-500', label: 'Medium', bold: true },
  low: { icon: SignalLow01Icon, color: 'text-sky-500', label: 'Low', bold: true },
  none: { icon: CancelCircleIcon, color: 'text-zinc-400', label: 'None' },
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
  const config = PRIORITY_CONFIG[priority];
  const Icon = config.icon;
  return <Icon className={`${className} ${config.color}`} {...(config.bold ? { strokeWidth: 3 } : {})} />;
}

// ── Severity icons & colors ─────────────────────────────────────────

export const SEVERITY_CONFIG: Record<
  Severity,
  { icon: React.ElementType; color: string; label: string }
> = {
  critical: { icon: OctagonIcon, color: 'text-red-600', label: 'Critical' },
  major: { icon: Shield02Icon, color: 'text-orange-500', label: 'Major' },
  minor: { icon: Alert01Icon, color: 'text-amber-500', label: 'Minor' },
  none: { icon: CancelCircleIcon, color: 'text-zinc-400', label: 'None' },
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
  backlog: { icon: DashedLineCircleIcon, color: 'text-zinc-400' },
  unstarted: { icon: MinusSignIcon, color: 'text-zinc-400' },
  started: { icon: RecordIcon, color: 'text-amber-500' },
  done: { icon: CheckmarkCircle02Icon, color: 'text-green-500' },
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
  { icon: React.ElementType; color: string; label: string }
> = {
  feature: { icon: SparklesIcon, color: 'text-amber-500', label: 'Feature' },
  bug: { icon: Bug01Icon, color: 'text-red-500', label: 'Bug' },
  chore: { icon: Wrench01Icon, color: 'text-indigo-500', label: 'Chore' },
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
  const config = TASK_TYPE_CONFIG[taskType];
  const Icon = config.icon;
  return <Icon className={`${className} ${config.color}`} />;
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
