import type React from 'react';
import { formatDistanceToNow, parseISO } from 'date-fns';
import { AgentAvatar } from '@/components/agents/AgentAvatar';
import { UserAvatar } from '@/components/pm/UserAvatar';
import {
  ArchiveIcon,
  ArrowLeftRightIcon,
  AttachmentIcon,
  BotIcon,
  Calendar03Icon,
  DashboardSpeed01Icon,
  HashtagIcon,
  HexagonIcon,
  Layers01Icon,
  LayoutGridIcon,
  Message01Icon,
  PencilEdit01Icon,
  PlayIcon,
  Shield02Icon,
  Tag01Icon,
  UserGroupIcon,
  UserIcon,
} from '@/lib/icons';
import {
  PRIORITY_CONFIG,
  PriorityIcon,
  SEVERITY_CONFIG,
  SeverityIcon,
  StateTypeIcon,
  TASK_TYPE_CONFIG,
  TaskTypeIcon,
} from '@/lib/pmConstants';
import type { ActivityLogEntry, Priority, Severity, TaskType, WorkflowState } from '@/lib/pmTypes';

const ACTIVITY_ICON_MAP: Record<string, { icon: React.ElementType; color: string }> = {
  workflow_state_id: { icon: HashtagIcon, color: 'text-blue-500' },
  epic_state_id: { icon: HashtagIcon, color: 'text-blue-500' },
  owner_member_id: { icon: UserIcon, color: 'text-violet-500' },
  owner_member_ids: { icon: UserIcon, color: 'text-violet-500' },
  team_id: { icon: UserGroupIcon, color: 'text-teal-500' },
  priority: { icon: DashboardSpeed01Icon, color: 'text-orange-500' },
  sprint_id: { icon: HexagonIcon, color: 'text-green-500' },
  epic_id: { icon: Layers01Icon, color: 'text-purple-500' },
  estimate: { icon: LayoutGridIcon, color: 'text-amber-500' },
  deadline: { icon: Calendar03Icon, color: 'text-red-500' },
  health: { icon: DashboardSpeed01Icon, color: 'text-green-500' },
  task_type: { icon: Tag01Icon, color: 'text-indigo-500' },
  labels: { icon: Tag01Icon, color: 'text-pink-500' },
  label: { icon: Tag01Icon, color: 'text-pink-500' },
  severity: { icon: Shield02Icon, color: 'text-red-500' },
  name: { icon: PencilEdit01Icon, color: 'text-muted-foreground' },
  description: { icon: PencilEdit01Icon, color: 'text-muted-foreground' },
  agent_run: { icon: BotIcon, color: 'text-indigo-500' },
};

const AGENT_RUN_ACTION_LABELS: Record<string, string> = {
  started: 'started',
  completed: 'completed',
  failed: 'failed',
  cancelled: 'cancelled',
  paused: 'paused',
  resumed: 'resumed',
};

const ACTION_LABELS: Record<string, string> = {
  comment_added: 'added a comment',
  comment_updated: 'edited a comment',
  comment_deleted: 'deleted a comment',
  attachment_added: 'attached a file',
  attachment_removed: 'removed an attachment',
  task_created: 'created this task',
  label_added: 'added a label',
  label_removed: 'removed a label',
  health_updated: 'updated health',
};

const FIELD_LABELS: Record<string, string> = {
  workflow_state_id: 'state',
  epic_state_id: 'state',
  owner_member_id: 'owner',
  owner_member_ids: 'owners',
  requester_member_id: 'requester',
  team_id: 'team',
  sprint_id: 'sprint',
  epic_id: 'epic',
  planned_start_date: 'start date',
  deadline: 'due date',
  planning_repository_id: 'code repo',
  task_type: 'type',
  health_comment: 'health comment',
};

function formatRelativeTime(iso: string) {
  try {
    return formatDistanceToNow(parseISO(iso), { addSuffix: true });
  } catch {
    return iso;
  }
}

function formatFieldLabel(fieldName: string): string {
  return FIELD_LABELS[fieldName] ?? fieldName.replace(/_id$/, '').replace(/_/g, ' ');
}

export function formatActivityAction(action: string | undefined, entityLabel: string, fieldName?: string): string {
  if (!action) return '';
  if (action === 'created') return `created this ${entityLabel}`;
  if (action === 'archived') return `archived this ${entityLabel}`;
  if (action === 'updated') return fieldName ? `updated ${formatFieldLabel(fieldName)}` : `updated this ${entityLabel}`;
  if (action === 'auto-started by automation') return `started this ${entityLabel}`;
  if (action === 'auto-completed by automation') return `completed this ${entityLabel}`;
  return ACTION_LABELS[action] ?? action.replace(/_/g, ' ');
}

export function getActivityActorLabel(action: string | undefined, actor?: ActivityLogEntry['actor']): string {
  if (actor) return actor.full_name || actor.email;
  if (action?.includes('by automation')) return 'Automation';
  return 'System';
}

function getActivityIcon(action?: string, fieldName?: string): { icon: React.ElementType; color: string } {
  if (fieldName && ACTIVITY_ICON_MAP[fieldName]) return ACTIVITY_ICON_MAP[fieldName];
  if (action?.includes('comment')) return { icon: Message01Icon, color: 'text-blue-500' };
  if (action?.includes('attachment') || action?.includes('file')) return { icon: AttachmentIcon, color: 'text-muted-foreground' };
  if (action?.includes('label')) return { icon: Tag01Icon, color: 'text-pink-500' };
  if (action?.includes('moved') || action?.includes('state')) return { icon: HashtagIcon, color: 'text-blue-500' };
  if (action?.includes('priority')) return { icon: DashboardSpeed01Icon, color: 'text-orange-500' };
  if (action?.includes('owner') || action?.includes('assigned') || action?.includes('requester')) return { icon: UserIcon, color: 'text-violet-500' };
  if (action?.includes('team')) return { icon: UserGroupIcon, color: 'text-teal-500' };
  if (action?.includes('sprint')) return { icon: HexagonIcon, color: 'text-green-500' };
  if (action?.includes('epic')) return { icon: Layers01Icon, color: 'text-purple-500' };
  if (action?.includes('blocked')) return { icon: Shield02Icon, color: 'text-red-500' };
  if (action?.includes('archived')) return { icon: ArchiveIcon, color: 'text-amber-500' };
  if (action?.includes('created')) return { icon: PlayIcon, color: 'text-green-500' };
  if (action?.includes('deadline') || action?.includes('due date')) return { icon: Calendar03Icon, color: 'text-red-500' };
  if (action?.includes('estimate')) return { icon: LayoutGridIcon, color: 'text-amber-500' };
  if (action?.includes('type')) return { icon: Tag01Icon, color: 'text-indigo-500' };
  if (action?.includes('severity')) return { icon: Shield02Icon, color: 'text-red-500' };
  return { icon: ArrowLeftRightIcon, color: 'text-muted-foreground' };
}

function ActivityTimelineEntry({
  entry,
  states = [],
  entityLabel,
}: {
  entry: ActivityLogEntry;
  states?: WorkflowState[];
  entityLabel: string;
}) {
  const { activity, actor } = entry;
  const iconConfig = getActivityIcon(activity.action, activity.field_name);
  const ActivityIconEl = iconConfig.icon;
  const label = formatActivityAction(activity.action, entityLabel, activity.field_name);
  const actorLabel = getActivityActorLabel(activity.action, actor);

  const stateMatch = activity.action?.match(/moved this (?:task|story|epic) to (.+)/);
  const targetStateName = stateMatch?.[1] ?? null;
  const matchedState = targetStateName ? states.find((s) => s.name === targetStateName) : null;
  const stateColor = matchedState?.color ?? null;

  const marker = actor ? (
    <span className="relative z-10">
      <UserAvatar
        name={actor.full_name || actor.email}
        avatarUrl={actor.avatar_url}
        className="h-5 w-5"
        fallbackClassName="text-[7px]"
      />
    </span>
  ) : (
    <span className="relative z-10 flex h-5 w-5 shrink-0 items-center justify-center rounded-full bg-muted ring-2 ring-background">
      <ActivityIconEl className={`h-3 w-3 ${iconConfig.color}`} />
    </span>
  );

  let richLabel: React.ReactNode = null;

  if (targetStateName) {
    richLabel = (
      <span className="inline-flex items-center gap-1 text-[11px] text-muted-foreground">
        moved to
        <span
          className="inline-flex items-center gap-1 rounded-full px-1.5 py-0.5 text-[10px] font-semibold"
          style={stateColor ? { backgroundColor: `${stateColor}18`, color: stateColor, border: `1px solid ${stateColor}30` } : undefined}
        >
          {matchedState && <StateTypeIcon stateType={matchedState.state_type} className="h-3 w-3" />}
          {targetStateName}
        </span>
      </span>
    );
  } else if (activity.field_name === 'agent_run') {
    const meta = activity.metadata as { agent_name?: string } | undefined;
    const rawAction = activity.new_value ?? 'started';
    const actionLabel = AGENT_RUN_ACTION_LABELS[rawAction] ?? rawAction;
    const agentName = meta?.agent_name ?? 'agent';

    richLabel = (
      <span className="inline-flex items-center gap-1 text-[11px] text-muted-foreground">
        {actionLabel} agent run
        <span className="inline-flex items-center gap-1 rounded-full border border-indigo-500/30 bg-indigo-500/10 px-1.5 py-0.5 text-[10px] font-medium text-indigo-700 dark:text-indigo-300">
          <AgentAvatar name={agentName} className="h-3 w-3 rounded-none border-0 bg-transparent shadow-none" genericBare />
          {agentName}
        </span>
      </span>
    );
  } else {
    const changeMatch = activity.action?.match(/changed (type|priority|severity) from (\S+) to (\S+)/);
    if (changeMatch) {
      const [, field, oldVal, newVal] = changeMatch;
      const renderBadge = (value: string) => {
        if (field === 'type') {
          const cfg = TASK_TYPE_CONFIG[value as TaskType];
          if (cfg) {
            return (
              <span className="inline-flex items-center gap-0.5 rounded-full border border-border/60 px-1.5 py-0.5 text-[10px] font-medium">
                <TaskTypeIcon taskType={value as TaskType} className="h-3 w-3" />
                {cfg.label}
              </span>
            );
          }
        }
        if (field === 'priority') {
          const cfg = PRIORITY_CONFIG[value as Priority];
          if (cfg) {
            return (
              <span className="inline-flex items-center gap-0.5 rounded-full border border-border/60 px-1.5 py-0.5 text-[10px] font-medium">
                <PriorityIcon priority={value as Priority} className="h-3 w-3" />
                {cfg.label}
              </span>
            );
          }
        }
        if (field === 'severity') {
          const cfg = SEVERITY_CONFIG[value as Severity];
          if (cfg) {
            return (
              <span className="inline-flex items-center gap-0.5 rounded-full border border-border/60 px-1.5 py-0.5 text-[10px] font-medium">
                <SeverityIcon severity={value as Severity} className="h-3 w-3" />
                {cfg.label}
              </span>
            );
          }
        }
        return <span className="rounded-full border border-border/60 px-1.5 py-0.5 text-[10px] font-medium">{value}</span>;
      };

      richLabel = (
        <span className="inline-flex items-center gap-1 text-[11px] text-muted-foreground flex-wrap">
          changed {field} from {renderBadge(oldVal)}
          <ArrowLeftRightIcon className="h-2.5 w-2.5 text-muted-foreground/50" />
          {renderBadge(newVal)}
        </span>
      );
    }
  }

  return (
    <div className="flex items-center gap-2.5">
      {marker}
      <div className="min-w-0 flex-1">
        <div className="flex items-center gap-1.5">
          <span className="text-xs font-medium shrink-0">{actorLabel}</span>
          {richLabel ?? <span className="text-[11px] text-muted-foreground truncate">{label}</span>}
          <span className="ml-auto shrink-0 text-[10px] text-muted-foreground/70">{formatRelativeTime(activity.created_at)}</span>
        </div>
      </div>
    </div>
  );
}

export function ActivityTimeline({
  activity,
  states = [],
  showAll,
  onShowAll,
  initialCount = 5,
  entityLabel = 'item',
}: {
  activity: ActivityLogEntry[];
  states?: WorkflowState[];
  showAll: boolean;
  onShowAll: () => void;
  initialCount?: number;
  entityLabel?: string;
}) {
  const visibleActivity = showAll ? activity : activity.slice(0, initialCount);

  return (
    <div className="relative mt-3">
      <div className="absolute left-[9px] top-3 bottom-3 w-px bg-border/60" />
      <div className="space-y-3">
        {!showAll && activity.length > initialCount && (
          <button
            type="button"
            className="relative z-10 ml-6 text-xs text-muted-foreground hover:text-foreground transition-colors cursor-pointer"
            onClick={onShowAll}
          >
            Show {activity.length - initialCount} older entries...
          </button>
        )}
        {visibleActivity.map((entry) => (
          <ActivityTimelineEntry key={`a-${entry.activity.id}`} entry={entry} states={states} entityLabel={entityLabel} />
        ))}
      </div>
    </div>
  );
}
