import { useEffect, useMemo, useState, type ReactNode } from 'react';
import { ArchiveIcon, ArrowDown01Icon, DragDropVerticalIcon, InboxIcon, PencilEdit01Icon, PlusSignIcon, Settings02Icon } from '@/lib/icons';
import {
  DndContext,
  closestCenter,
  KeyboardSensor,
  PointerSensor,
  useSensor,
  useSensors,
  type DragEndEvent,
} from '@dnd-kit/core';
import {
  arrayMove,
  SortableContext,
  sortableKeyboardCoordinates,
  useSortable,
  verticalListSortingStrategy,
} from '@dnd-kit/sortable';
import { CSS } from '@dnd-kit/utilities';
import { useQueryClient } from '@tanstack/react-query';
import { toast } from 'sonner';
import { TeamInboxDialog } from '@/components/support/TeamInboxDialog';
import { UserAvatar } from '@/components/pm/UserAvatar';
import { useConfirm } from '@/components/ui/confirm-dialog';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Checkbox } from '@/components/ui/checkbox';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Switch } from '@/components/ui/switch';
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/tooltip';
import { ICON_MAP } from '@/components/ui/icon-picker';
import { cn } from '@/lib/utils';
import { queryKeys } from '@/lib/queryKeys';

import {
  useArchiveMailbox,
  useChatSettings,
  useMailboxMembers,
  useReorderMailboxes,
  useSupportRoutingUsage,
  useSupportMailboxes,
  useSupportTriageRules,
  useUpdateChatSettings,
} from '@/hooks/queries/useSupport';
import type {
  SupportInboxSettings,
  SupportMailbox,
  SupportMailboxMember,
  SupportTriageRule,
} from '@/lib/pmTypes';

type RoutingSettingsDraft = Pick<
  SupportInboxSettings,
  | 'triage_enabled'
  | 'triage_auto_move_enabled'
  | 'triage_confidence_threshold'
  | 'triage_widget_enabled'
  | 'triage_email_enabled'
  | 'triage_internal_enabled'
  | 'triage_fallback_behavior'
  | 'triage_rerun_on_meaning_change'
  | 'triage_daily_budget'
  | 'triage_skip_spam_conversations'
  | 'triage_deduplicate_first_message'
>;

const DEFAULT_ROUTING_SETTINGS: RoutingSettingsDraft = {
  triage_enabled: false,
  triage_auto_move_enabled: true,
  triage_confidence_threshold: 0.8,
  triage_widget_enabled: true,
  triage_email_enabled: true,
  triage_internal_enabled: false,
  triage_fallback_behavior: 'shared',
  triage_rerun_on_meaning_change: false,
  triage_daily_budget: 250,
  triage_skip_spam_conversations: true,
  triage_deduplicate_first_message: true,
};

function buildRoutingDraft(settings?: SupportInboxSettings | null): RoutingSettingsDraft {
  if (!settings) return DEFAULT_ROUTING_SETTINGS;
  return {
    triage_enabled: settings.triage_enabled,
    triage_auto_move_enabled: settings.triage_auto_move_enabled,
    triage_confidence_threshold: settings.triage_confidence_threshold,
    triage_widget_enabled: settings.triage_widget_enabled,
    triage_email_enabled: settings.triage_email_enabled,
    triage_internal_enabled: settings.triage_internal_enabled,
    triage_fallback_behavior: settings.triage_fallback_behavior,
    triage_rerun_on_meaning_change: settings.triage_rerun_on_meaning_change,
    triage_daily_budget: settings.triage_daily_budget,
    triage_skip_spam_conversations: settings.triage_skip_spam_conversations,
    triage_deduplicate_first_message: settings.triage_deduplicate_first_message,
  };
}

function serializeRoutingDraft(draft: RoutingSettingsDraft) {
  return JSON.stringify(draft);
}

function RoutingCardHeader({
  title,
  description,
  icon,
  status,
  action,
  chevron,
  asButton = false,
  onClick,
}: {
  title: string;
  description: string;
  icon: ReactNode;
  status?: ReactNode;
  action?: ReactNode;
  chevron?: ReactNode;
  asButton?: boolean;
  onClick?: () => void;
}) {
  const content = (
    <>
      <span className="flex h-9 w-9 shrink-0 items-center justify-center rounded-lg bg-muted text-muted-foreground">
        {icon}
      </span>
      <span className="min-w-0 flex-1">
        <span className="flex flex-wrap items-center gap-2">
          <span className="text-sm font-medium text-foreground">{title}</span>
          {status}
        </span>
        <span className="mt-0.5 block text-xs text-muted-foreground">{description}</span>
      </span>
      {chevron}
    </>
  );

  return (
    <div className="flex items-center border-b">
      {asButton ? (
        <button
          type="button"
          onClick={onClick}
          className="flex min-w-0 flex-1 items-center gap-4 px-4 py-4 text-left transition-colors hover:bg-muted/40"
        >
          {content}
        </button>
      ) : (
        <div className="flex min-w-0 flex-1 items-center gap-4 px-4 py-4">
          {content}
        </div>
      )}
      {action ? <div className="shrink-0 pr-4">{action}</div> : null}
    </div>
  );
}

function RoutingSection({
  id,
  title,
  description,
  icon,
  status,
  action,
  expanded,
  onToggle,
  children,
}: {
  id: string;
  title: string;
  description: string;
  icon: ReactNode;
  status?: ReactNode;
  action?: ReactNode;
  expanded: boolean;
  onToggle: (id: string) => void;
  children: ReactNode;
}) {
  return (
    <section className={cn('overflow-hidden rounded-lg border bg-card transition-colors', expanded ? 'border-primary/20' : 'border-border/60')}>
      <RoutingCardHeader
        title={title}
        description={description}
        icon={icon}
        status={status}
        action={action}
        asButton
        onClick={() => onToggle(id)}
        chevron={(
          <ArrowDown01Icon className={cn(
            'h-4 w-4 shrink-0 text-muted-foreground transition-transform duration-200',
            expanded && 'rotate-180',
          )} />
        )}
      />
      <div className="accordion-animate" data-open={expanded}>
        <div>{children}</div>
      </div>
    </section>
  );
}

function RoutingSettingRow({
  title,
  description,
  children,
  className,
}: {
  title: string;
  description?: string;
  children: ReactNode;
  className?: string;
}) {
  return (
    <div className={cn('grid gap-4 border-b px-6 py-5 last:border-b-0 md:grid-cols-[minmax(180px,0.42fr)_1fr]', className)}>
      <div className="min-w-0">
        <Label className="text-sm font-medium">{title}</Label>
        {description ? <p className="mt-1 text-sm text-muted-foreground">{description}</p> : null}
      </div>
      <div className="min-w-0 md:justify-self-stretch">{children}</div>
    </div>
  );
}

function ConfidenceThresholdControl({
  value,
  disabled,
  onChange,
}: {
  value: number;
  disabled: boolean;
  onChange: (value: number) => void;
}) {
  const percent = Math.round(value * 100);
  const fillPercent = Math.max(0, Math.min(100, ((value - 0.5) / 0.5) * 100));

  return (
    <div className="ml-auto w-full max-w-md space-y-3">
      <div className="relative w-full pt-6">
        <span
          className="absolute top-0 -translate-x-1/2 rounded-md border bg-background px-1.5 py-0.5 text-[11px] font-medium tabular-nums text-foreground shadow-sm"
          style={{ left: `clamp(1.25rem, ${fillPercent}%, calc(100% - 1.25rem))` }}
        >
          {percent}%
        </span>
        <div className="relative h-2 w-full">
          <div className="absolute inset-y-0 left-0 right-0 rounded-full bg-border" />
          <div
            className="absolute inset-y-0 left-0 rounded-full bg-foreground"
            style={{ width: `${fillPercent}%` }}
          />
          <input
            type="range"
            min="0.5"
            max="1"
            step="0.01"
            value={value}
            disabled={disabled}
            onChange={(event) => onChange(Number(event.target.value))}
            className="absolute inset-0 h-full w-full cursor-pointer appearance-none bg-transparent accent-foreground disabled:cursor-not-allowed disabled:opacity-50 [&::-webkit-slider-thumb]:h-4 [&::-webkit-slider-thumb]:w-4 [&::-webkit-slider-thumb]:appearance-none [&::-webkit-slider-thumb]:rounded-full [&::-webkit-slider-thumb]:border-2 [&::-webkit-slider-thumb]:border-background [&::-webkit-slider-thumb]:bg-foreground [&::-webkit-slider-thumb]:shadow [&::-moz-range-thumb]:h-4 [&::-moz-range-thumb]:w-4 [&::-moz-range-thumb]:appearance-none [&::-moz-range-thumb]:rounded-full [&::-moz-range-thumb]:border-2 [&::-moz-range-thumb]:border-background [&::-moz-range-thumb]:bg-foreground"
          />
        </div>
      </div>
    </div>
  );
}

function getRuleConditionCount(rule: SupportTriageRule) {
  return (
    (rule.conditions.phrase_contains ?? []).length +
    (rule.conditions.email_domain_equals ?? []).length +
    (rule.conditions.sender_email_contains ?? []).length
  );
}

function getMailboxRuleSummary(mailboxRules: SupportTriageRule[]) {
  const activeRules = mailboxRules.filter((rule) => rule.active);
  const conditionCount = activeRules.reduce((count, rule) => count + getRuleConditionCount(rule), 0);

  if (conditionCount === 0) return 'No rule';
  if (conditionCount === 1) return '1 condition';
  return `${conditionCount} conditions`;
}

function getRuleConditionLines(rule: SupportTriageRule) {
  const lines: string[] = [];
  const textValues = rule.conditions.phrase_contains ?? [];
  const emailValues = [
    ...(rule.conditions.sender_email_contains ?? []),
    ...(rule.conditions.email_domain_equals ?? []),
  ];

  if (textValues.length > 0) {
    lines.push(`Text contains: ${textValues.join(' OR ')}`);
  }
  if (emailValues.length > 0) {
    lines.push(`Email ID contains: ${emailValues.join(' OR ')}`);
  }

  return lines;
}

function ManualRuleBadge({ mailboxRules }: { mailboxRules: SupportTriageRule[] }) {
  const activeRules = mailboxRules.filter((rule) => rule.active);
  const ruleSummary = getMailboxRuleSummary(mailboxRules);

  const badge = (
    <Badge variant={ruleSummary === 'No rule' ? 'destructive' : 'secondary'} className="h-5 px-2 text-[11px]">
      {ruleSummary}
    </Badge>
  );

  if (activeRules.length === 0) return badge;

  return (
    <Tooltip>
      <TooltipTrigger asChild>
        <span className="inline-flex cursor-default">{badge}</span>
      </TooltipTrigger>
      <TooltipContent side="bottom" className="max-w-xs p-2">
        <div className="space-y-2">
          {activeRules.map((rule) => {
            const lines = getRuleConditionLines(rule);
            return (
              <div key={rule.id} className="space-y-1">
                {lines.length > 0 ? (
                  lines.map((line) => (
                    <p key={line} className="text-xs">
                      {line}
                    </p>
                  ))
                ) : (
                  <p className="text-xs">No conditions</p>
                )}
              </div>
            );
          })}
        </div>
      </TooltipContent>
    </Tooltip>
  );
}

function getAIRoutingDescription(mailbox: SupportMailbox) {
  return mailbox.routing_prompt?.trim() || mailbox.description?.trim() || '';
}

function AIFallbackBadge({ mailbox }: { mailbox: SupportMailbox }) {
  const description = getAIRoutingDescription(mailbox);

  const badge = (
    <Badge
      variant={mailbox.triage_eligible && description ? 'secondary' : mailbox.triage_eligible ? 'outline' : 'destructive'}
      className={cn(
        'h-5 px-2 text-[11px]',
        mailbox.triage_eligible && description && 'bg-emerald-100 text-emerald-700 hover:bg-emerald-100 dark:bg-emerald-900/30 dark:text-emerald-300',
      )}
    >
      {mailbox.triage_eligible ? (description ? 'On' : 'Needs text') : 'Off'}
    </Badge>
  );

  if (!description) return badge;

  return (
    <Tooltip>
      <TooltipTrigger asChild>
        <span className="inline-flex cursor-default">{badge}</span>
      </TooltipTrigger>
      <TooltipContent side="bottom" className="max-w-xs p-2">
        <p className="text-xs">{description}</p>
      </TooltipContent>
    </Tooltip>
  );
}

function InboxMembersCell({ mailbox }: { mailbox: SupportMailbox }) {
  const { data: members = [] } = useMailboxMembers(mailbox.workspace_id, mailbox.id);
  const memberCount = members.length || mailbox.member_count || 0;

  if (memberCount === 0) {
    return <span className="text-xs text-muted-foreground/50">-</span>;
  }

  const visibleMembers = members.slice(0, 4);
  const avatarStack = (
    <div className="flex items-center gap-1.5">
      <div className="flex -space-x-1.5">
        {visibleMembers.map((member) => (
          <InboxMemberAvatar key={member.workspace_member_id} member={member} />
        ))}
      </div>
      {memberCount > 4 ? (
        <span className="text-xs text-muted-foreground">+{memberCount - 4}</span>
      ) : null}
      {members.length === 0 ? (
        <span className="text-xs text-muted-foreground">{memberCount}</span>
      ) : null}
    </div>
  );

  if (members.length === 0) return avatarStack;

  return (
    <Tooltip>
      <TooltipTrigger asChild>
        <div className="inline-flex cursor-default">{avatarStack}</div>
      </TooltipTrigger>
      <TooltipContent side="bottom" className="p-2">
        <div className="space-y-1.5">
          {members.map((member) => (
            <div key={member.workspace_member_id} className="flex items-center gap-2">
              <InboxMemberAvatar member={member} className="h-5 w-5" />
              <span className="text-xs">{member.display_name || member.email}</span>
            </div>
          ))}
        </div>
      </TooltipContent>
    </Tooltip>
  );
}

function InboxMemberAvatar({ member, className }: { member: SupportMailboxMember; className?: string }) {
  const name = member.display_name || member.email;

  return (
    <UserAvatar
      name={name}
      avatarUrl={member.avatar_url}
      avatarStyle={member.avatar_style}
      avatarSeed={member.avatar_seed}
      avatarBackgroundMode={member.avatar_background_mode}
      avatarBackgroundColor={member.avatar_background_color}
      className={cn('h-5 w-5 ring-1 ring-background', className)}
    />
  );
}

function SortableInboxRoutingRow({
  mailbox,
  mailboxRules,
  onEdit,
  onArchive,
  isArchiving,
}: {
  mailbox: SupportMailbox;
  mailboxRules: SupportTriageRule[];
  onEdit: (mailbox: SupportMailbox) => void;
  onArchive: (mailbox: SupportMailbox) => void;
  isArchiving: boolean;
}) {
  const { attributes, listeners, setNodeRef, transform, transition, isDragging } = useSortable({
    id: mailbox.id,
  });
  const MailboxIcon = ICON_MAP[mailbox.icon] ?? ICON_MAP.inbox;

  return (
    <div
      ref={setNodeRef}
      style={{ transform: CSS.Transform.toString(transform), transition }}
      className={cn(
        'grid items-center gap-3 border-b px-6 py-3 last:border-b-0 lg:grid-cols-[minmax(240px,1fr)_120px_120px_120px_120px_80px]',
        isDragging && 'opacity-50',
      )}
    >
      <div className="flex min-w-0 items-center gap-3">
        <button
          type="button"
          className="flex h-5 w-5 shrink-0 cursor-grab items-center justify-center text-muted-foreground/50 hover:text-muted-foreground active:cursor-grabbing"
          aria-label={`Reorder ${mailbox.name}`}
          {...attributes}
          {...listeners}
        >
          <DragDropVerticalIcon className="h-4 w-4" />
        </button>
        {MailboxIcon ? <MailboxIcon className="h-4 w-4 shrink-0 text-muted-foreground" /> : null}
        <div className="min-w-0">
          <p className="truncate text-sm font-medium text-foreground">{mailbox.name}</p>
          <p className="mt-1 truncate text-xs text-muted-foreground">
            #{mailbox.handle}
            {mailbox.linked_team_name ? ` · ${mailbox.linked_team_name}` : ''}
          </p>
        </div>
      </div>
      <div>
        <InboxMembersCell mailbox={mailbox} />
      </div>
      <div>
        <Badge variant="secondary" className="h-5 px-2 text-[11px]">
          {mailbox.assignment_mode === 'round_robin' ? 'Round robin' : 'Manual'}
        </Badge>
      </div>
      <div>
        <ManualRuleBadge mailboxRules={mailboxRules} />
      </div>
      <div>
        <AIFallbackBadge mailbox={mailbox} />
      </div>
      <div className="flex items-center justify-start gap-1 lg:justify-end">
        <Button variant="ghost" size="icon" className="h-8 w-8" onClick={() => onEdit(mailbox)} aria-label={`Edit ${mailbox.name}`}>
          <PencilEdit01Icon className="h-3.5 w-3.5" />
        </Button>
        <Button
          variant="ghost"
          size="icon"
          className="h-8 w-8"
          disabled={!mailbox.active || isArchiving}
          onClick={() => onArchive(mailbox)}
          aria-label={`Archive ${mailbox.name}`}
        >
          <ArchiveIcon className="h-3.5 w-3.5" />
        </Button>
      </div>
    </div>
  );
}

/* ── Main component ──────────────────────────────────────────────────── */

export function ConversationRoutingTab({ workspaceId }: { workspaceId: string }) {
  const confirm = useConfirm();
  const queryClient = useQueryClient();
  const { data: installation, isLoading } = useChatSettings(workspaceId);
  const { data: routingUsage } = useSupportRoutingUsage(workspaceId);
  const { data: mailboxes = [] } = useSupportMailboxes(workspaceId);
  const { data: rules = [] } = useSupportTriageRules(workspaceId);
  const updateSettings = useUpdateChatSettings(workspaceId);
  const archiveMailbox = useArchiveMailbox(workspaceId);
  const reorderMailboxes = useReorderMailboxes(workspaceId);

  const activeMailboxes = useMemo(
    () => mailboxes.filter((mailbox) => mailbox.active),
    [mailboxes],
  );
  const rulesByMailbox = useMemo(() => {
    const grouped = new Map<string, SupportTriageRule[]>();
    for (const rule of rules) {
      const existing = grouped.get(rule.target_mailbox_id) ?? [];
      existing.push(rule);
      grouped.set(rule.target_mailbox_id, existing);
    }
    return grouped;
  }, [rules]);

  const sensors = useSensors(
    useSensor(PointerSensor, { activationConstraint: { distance: 5 } }),
    useSensor(KeyboardSensor, { coordinateGetter: sortableKeyboardCoordinates }),
  );

  const [draft, setDraft] = useState<RoutingSettingsDraft>(DEFAULT_ROUTING_SETTINGS);
  const [editingMailbox, setEditingMailbox] = useState<SupportMailbox | null>(null);
  const [mailboxDialogOpen, setMailboxDialogOpen] = useState(false);

  const openCreateMailbox = () => {
    setEditingMailbox(null);
    setMailboxDialogOpen(true);
  };
  const openEditMailbox = (mailbox: SupportMailbox) => {
    setEditingMailbox(mailbox);
    setMailboxDialogOpen(true);
  };

  // Accordion state
  const [expandedSections, setExpandedSections] = useState<Set<string>>(new Set());
  const toggleSection = (id: string) => {
    setExpandedSections((prev) => {
      const next = new Set(prev);
      if (next.has(id)) next.delete(id);
      else next.add(id);
      return next;
    });
  };
  const isExpanded = (id: string) => expandedSections.has(id);

  useEffect(() => {
    if (installation?.settings) {
      setDraft(buildRoutingDraft(installation.settings));
    }
  }, [installation?.settings]);

  const savedDraftKey = installation?.settings ? serializeRoutingDraft(buildRoutingDraft(installation.settings)) : serializeRoutingDraft(DEFAULT_ROUTING_SETTINGS);
  const isDirty = serializeRoutingDraft(draft) !== savedDraftKey;
  const routingControlsDisabled = !draft.triage_enabled || updateSettings.isPending;
  const handleSaveSettings = async () => {
    try {
      await updateSettings.mutateAsync(draft);
      toast.success('Conversation routing settings saved');
    } catch {
      // Mutation hook shows toast on failure.
    }
  };

  const handleArchiveMailbox = async (mailbox: SupportMailbox) => {
    const ok = await confirm({
      title: `Archive ${mailbox.name}?`,
      description: 'This inbox will be archived. You can restore it later.',
      confirmText: 'Archive',
      variant: 'destructive',
    });
    if (!ok) return;
    archiveMailbox.mutate(mailbox.id, {
      onSuccess: () => toast.success('Team inbox archived'),
    });
  };

  const handleDragEnd = (event: DragEndEvent) => {
    const { active, over } = event;
    if (!over || active.id === over.id) return;

    const oldIndex = activeMailboxes.findIndex((mailbox) => mailbox.id === active.id);
    const newIndex = activeMailboxes.findIndex((mailbox) => mailbox.id === over.id);
    if (oldIndex === -1 || newIndex === -1) return;

    const reordered = arrayMove(activeMailboxes, oldIndex, newIndex);
    const inactiveMailboxes = mailboxes.filter((mailbox) => !mailbox.active);
    queryClient.setQueryData(queryKeys.support.mailboxes(workspaceId), [...reordered, ...inactiveMailboxes]);
    reorderMailboxes.mutate(reordered.map((mailbox) => mailbox.id));
  };

  if (isLoading) {
    return (
      <div className="flex items-center justify-center py-16 text-sm text-muted-foreground">
        Loading routing settings...
      </div>
    );
  }

  return (
    <div className="space-y-3">
      {/* Floating save bar */}
      {isDirty && (
        <div className="fixed bottom-4 right-4 z-50 flex items-center gap-3 rounded-lg border bg-background/95 px-4 py-2.5 shadow-lg backdrop-blur">
          <Badge variant="secondary">Unsaved changes</Badge>
          <Button size="sm" onClick={handleSaveSettings} disabled={updateSettings.isPending}>
            {updateSettings.isPending ? 'Saving...' : 'Save'}
          </Button>
        </div>
      )}

      {routingUsage?.triage_enabled && routingUsage.exhausted ? (
        <div className="rounded-lg border border-amber-200 bg-amber-50 px-4 py-3 text-sm text-amber-900 dark:border-amber-900/60 dark:bg-amber-950/30 dark:text-amber-200">
          AI routing limit reached today. Manual rules still run; AI routing resumes after the daily reset.
        </div>
      ) : null}

      <section className="overflow-hidden rounded-lg border border-border/60 bg-card">
        <RoutingCardHeader
          title="Inboxes"
          description="Create destinations for teams, topics, or workflows that need their own queue."
          icon={<InboxIcon className="h-4 w-4" />}
          action={(
            <Button size="sm" onClick={openCreateMailbox} className="shrink-0">
              <PlusSignIcon className="mr-1.5 h-3.5 w-3.5" />
              Add inbox
            </Button>
          )}
        />

        <div className="hidden grid-cols-[minmax(240px,1fr)_120px_120px_120px_120px_80px] gap-3 border-b bg-muted/25 px-6 py-2 text-xs font-medium text-muted-foreground lg:grid">
          <span className="pl-[3.75rem]">Inbox</span>
          <span>Members</span>
          <span>Assignment</span>
          <span>Manual rule</span>
          <span>AI routing</span>
          <span className="text-right">Actions</span>
        </div>

        {activeMailboxes.length === 0 ? (
          <div className="px-6 py-8 text-center">
            <p className="text-sm font-medium text-foreground">No team inboxes yet</p>
            <p className="mt-1 text-sm text-muted-foreground">Create an inbox for teams like Billing, Support, or VIP customers.</p>
            <Button className="mt-4" variant="outline" size="sm" onClick={openCreateMailbox}>
              <PlusSignIcon className="mr-1.5 h-3.5 w-3.5" />
              Add inbox
            </Button>
          </div>
        ) : (
          <DndContext sensors={sensors} collisionDetection={closestCenter} onDragEnd={handleDragEnd}>
            <SortableContext items={activeMailboxes.map((mailbox) => mailbox.id)} strategy={verticalListSortingStrategy}>
              <div>
                {activeMailboxes.map((mailbox) => (
                  <SortableInboxRoutingRow
                    key={mailbox.id}
                    mailbox={mailbox}
                    mailboxRules={rulesByMailbox.get(mailbox.id) ?? []}
                    onEdit={openEditMailbox}
                    onArchive={handleArchiveMailbox}
                    isArchiving={archiveMailbox.isPending}
                  />
                ))}
              </div>
            </SortableContext>
          </DndContext>
        )}
      </section>

      <section className="overflow-hidden rounded-lg border border-border/60 bg-card">
        <RoutingCardHeader
          title="Automated routing"
          description="Automated routing checks manual rules first, then uses AI when no rule matches."
          icon={<Settings02Icon className="h-4 w-4" />}
          action={(
            <label className="flex shrink-0 items-center gap-2 text-sm font-medium">
              Enable routing
              <Switch
                checked={draft.triage_enabled}
                disabled={updateSettings.isPending}
                onCheckedChange={(checked) => setDraft((current) => ({ ...current, triage_enabled: checked }))}
              />
            </label>
          )}
        />
        <div className="divide-y">
          <div className={cn('divide-y', routingControlsDisabled && 'opacity-60')}>
            <RoutingSettingRow
              title="Channels"
              description="Conversation sources where routing should run."
              className="md:grid-cols-[minmax(220px,0.42fr)_auto]"
            >
              <div className="flex flex-wrap justify-start gap-x-6 gap-y-3 md:justify-end">
                <label className="flex items-center gap-2 text-sm">
                  <Checkbox
                    checked={draft.triage_widget_enabled}
                    disabled={routingControlsDisabled}
                    onCheckedChange={(checked) => setDraft((current) => ({ ...current, triage_widget_enabled: Boolean(checked) }))}
                  />
                  Widget
                </label>
                <label className="flex items-center gap-2 text-sm">
                  <Checkbox
                    checked={draft.triage_email_enabled}
                    disabled={routingControlsDisabled}
                    onCheckedChange={(checked) => setDraft((current) => ({ ...current, triage_email_enabled: Boolean(checked) }))}
                  />
                  Email
                </label>
                <label className="flex items-center gap-2 text-sm">
                  <Checkbox
                    checked={draft.triage_internal_enabled}
                    disabled={routingControlsDisabled}
                    onCheckedChange={(checked) => setDraft((current) => ({ ...current, triage_internal_enabled: Boolean(checked) }))}
                  />
                  Internal & API
                </label>
              </div>
            </RoutingSettingRow>

            <RoutingSettingRow
              title="Do not auto-move conversations"
              description="Show suggestions instead of moving matched conversations."
              className="md:grid-cols-[minmax(220px,0.42fr)_auto]"
            >
              <div className="flex justify-start md:justify-end">
                <Switch
                  checked={!draft.triage_auto_move_enabled}
                  disabled={routingControlsDisabled}
                  onCheckedChange={(checked) => setDraft((current) => ({ ...current, triage_auto_move_enabled: !checked }))}
                />
              </div>
            </RoutingSettingRow>
          </div>
        </div>
      </section>

      <RoutingSection
        id="advanced"
        title="Advanced Settings"
        description="AI thresholds, daily limits, and spam prevention."
        icon={<Settings02Icon className="h-4 w-4" />}
        expanded={isExpanded('advanced')}
        onToggle={toggleSection}
      >
        <div className={cn('divide-y', routingControlsDisabled && 'opacity-60')}>
          <RoutingSettingRow
            title="AI auto-move confidence threshold"
            description="Below this, conversations stay as suggestions."
          >
            <ConfidenceThresholdControl
              value={draft.triage_confidence_threshold}
              disabled={routingControlsDisabled}
              onChange={(value) => setDraft((current) => ({ ...current, triage_confidence_threshold: value }))}
            />
          </RoutingSettingRow>

          <RoutingSettingRow
            title="Re-run on new customer replies"
            description="Run routing again when customers send follow-up replies."
            className="md:grid-cols-[minmax(220px,0.42fr)_auto]"
          >
            <div className="flex justify-start md:justify-end">
              <Switch
                checked={draft.triage_rerun_on_meaning_change}
                disabled={routingControlsDisabled}
                onCheckedChange={(checked) => setDraft((c) => ({ ...c, triage_rerun_on_meaning_change: checked }))}
              />
            </div>
          </RoutingSettingRow>

          <RoutingSettingRow
            title="Daily AI routing attempts"
            description="Maximum non-cached AI routing evaluations per day. Use 0 for unlimited."
            className="md:grid-cols-[minmax(220px,0.42fr)_auto]"
          >
            <div className="flex justify-start md:justify-end">
              <Input
                id="triage-daily-budget"
                type="number"
                min={0}
                disabled={routingControlsDisabled}
                value={draft.triage_daily_budget}
                onChange={(event) => setDraft((c) => ({
                  ...c,
                  triage_daily_budget: Math.max(0, Number(event.target.value) || 0),
                }))}
                className="w-full sm:w-32"
              />
            </div>
          </RoutingSettingRow>

          <RoutingSettingRow
            title="Skip conversations already marked as spam"
            description="Do not run routing when a conversation is already in spam."
            className="md:grid-cols-[minmax(220px,0.42fr)_auto]"
          >
            <div className="flex justify-start md:justify-end">
              <Switch
                checked={draft.triage_skip_spam_conversations}
                disabled={routingControlsDisabled}
                onCheckedChange={(checked) => setDraft((c) => ({ ...c, triage_skip_spam_conversations: checked }))}
              />
            </div>
          </RoutingSettingRow>

        </div>
      </RoutingSection>

      <TeamInboxDialog
        workspaceId={workspaceId}
        open={mailboxDialogOpen}
        onOpenChange={(open) => {
          setMailboxDialogOpen(open);
          if (!open) setEditingMailbox(null);
        }}
        mailbox={editingMailbox}
      />
    </div>
  );
}
