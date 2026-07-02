import { useEffect, useMemo, useState, type ReactNode } from 'react';
import { ArrowRight02Icon, Tick01Icon, ArrowLeft01Icon, HelpCircleIcon, PlusSignIcon, Cancel01Icon } from '@/lib/icons';
import { toast } from 'sonner';
import { UserAvatar } from '@/components/pm/UserAvatar';
import { Button } from '@/components/ui/button';
import { ChipInput } from '@/components/ui/chip-input';
import { Command, CommandEmpty, CommandGroup, CommandInput, CommandItem, CommandList } from '@/components/ui/command';
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog';
import { IconPicker } from '@/components/ui/icon-picker';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Popover, PopoverContent, PopoverTrigger } from '@/components/ui/popover';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { Switch } from '@/components/ui/switch';
import { Textarea } from '@/components/ui/textarea';
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/tooltip';
import { useWorkspaceModuleAccess } from '@/hooks/queries/useSettings';
import { useAssignableMembers } from '@/hooks/queries/useWorkspaces';
import {
  useCreateMailbox,
  useCreateSupportTriageRule,
  useChatSettings,
  useDeleteSupportTriageRule,
  useMailboxMembers,
  useSupportTriageRules,
  useUpdateMailbox,
  useUpdateSupportTriageRule,
} from '@/hooks/queries/useSupport';
import { useWorkspaceTeams } from '@/hooks/useWorkspaceTeams';
import { useAuthStore } from '@/stores/authStore';
import { useWorkspaceStore } from '@/stores/workspaceStore';
import type {
  CreateSupportMailboxRequest,
  CreateSupportTriageRuleRequest,
  SupportMailbox,
  SupportTriageRule,
  UpdateSupportMailboxRequest,
} from '@/lib/pmTypes';
import {
  buildSupportMemberOptions,
  buildSupportTeamOptions,
  filterMembersOutsideLinkedTeam,
  filterSupportAccessibleMembers,
  splitMailboxMembersBySelection,
} from './teamInboxDialogMembers';
import { getTeamInboxDialogSteps } from './teamInboxDialogFlow';

type MailboxFormState = {
  name: string;
  handle: string;
  icon: string;
  description: string;
  routingPrompt: string;
  triageEligible: boolean;
  linkedTeamId: string;
  assignmentMode: 'manual' | 'round_robin';
  workspaceMemberIds: string[];
  replyTimeOverride: boolean;
  replyTimePreset: string;
  replyTimeCustomMinutes: number | null;
};

type ManualRoutingConditionDraft = {
  id: string;
  type: 'message_contains' | 'sender_email_contains';
  values: string[];
  inputValue: string;
};

type ManualRoutingRuleDraft = {
  persistedId?: string;
  active: boolean;
  priority: number;
  conditionLogic: 'any' | 'all';
  conditions: ManualRoutingConditionDraft[];
};

const DEFAULT_FORM: MailboxFormState = {
  name: '',
  handle: '',
  icon: 'inbox',
  description: '',
  routingPrompt: '',
  triageEligible: true,
  linkedTeamId: 'none',
  assignmentMode: 'manual',
  workspaceMemberIds: [],
  replyTimeOverride: false,
  replyTimePreset: 'few_minutes',
  replyTimeCustomMinutes: null,
};

const EMPTY_MEMBERS: never[] = [];
const DIALOG_STEPS = getTeamInboxDialogSteps();

function createRoutingCondition(type: ManualRoutingConditionDraft['type'] = 'message_contains'): ManualRoutingConditionDraft {
  return {
    id: `condition-${Date.now()}-${Math.random().toString(36).slice(2)}`,
    type,
    values: [],
    inputValue: '',
  };
}

function createEmptyRoutingRuleDraft(priority: number): ManualRoutingRuleDraft {
  return {
    active: true,
    priority,
    conditionLogic: 'any',
    conditions: [createRoutingCondition()],
  };
}

function buildRoutingRuleDraft(rules: SupportTriageRule[]): ManualRoutingRuleDraft | null {
  if (rules.length === 0) return null;
  const sortedRules = [...rules].sort((a, b) => a.priority - b.priority);
  const primaryRule = sortedRules[0];
  const conditions = sortedRules.flatMap((rule) => [
    ...(rule.conditions?.phrase_contains ?? []).map((value) => ({
      id: `condition-${rule.id}-phrase-${value}`,
      type: 'message_contains' as const,
      values: [value],
      inputValue: '',
    })),
    ...(rule.conditions?.email_domain_equals ?? []).map((value) => ({
      id: `condition-${rule.id}-domain-${value}`,
      type: 'sender_email_contains' as const,
      values: [value],
      inputValue: '',
    })),
    ...(rule.conditions?.sender_email_contains ?? []).map((value) => ({
      id: `condition-${rule.id}-email-${value}`,
      type: 'sender_email_contains' as const,
      values: [value],
      inputValue: '',
    })),
  ]);

  return {
    persistedId: primaryRule.id,
    active: sortedRules.some((rule) => rule.active),
    priority: primaryRule.priority,
    conditionLogic: primaryRule.conditions?.condition_logic === 'any' ? 'any' : 'all',
    conditions: conditions.length > 0 ? conditions : [createRoutingCondition()],
  };
}

function normalizeHandle(value: string) {
  return value.trim().toLowerCase().replace(/\s+/g, '-');
}

function buildFormState(mailbox?: SupportMailbox | null): MailboxFormState {
  if (!mailbox) return DEFAULT_FORM;
  const hasOverride = Boolean(mailbox.reply_time_preset);
  return {
    name: mailbox.name,
    handle: mailbox.handle,
    icon: mailbox.icon || 'inbox',
    description: mailbox.description ?? '',
    routingPrompt: mailbox.routing_prompt ?? '',
    triageEligible: mailbox.triage_eligible,
    linkedTeamId: mailbox.linked_team_id ?? 'none',
    assignmentMode: mailbox.assignment_mode,
    workspaceMemberIds: [],
    replyTimeOverride: hasOverride,
    replyTimePreset: mailbox.reply_time_preset ?? 'few_minutes',
    replyTimeCustomMinutes: mailbox.reply_time_custom_minutes ?? null,
  };
}

function FieldLabel({ htmlFor, children, tip }: { htmlFor?: string; children: ReactNode; tip: string }) {
  return (
    <div className="flex items-center gap-1.5">
      <Label htmlFor={htmlFor}>{children}</Label>
      <Tooltip>
        <TooltipTrigger asChild>
          <button
            type="button"
            aria-label="More info"
            className="inline-flex cursor-help items-center justify-center rounded-full text-muted-foreground/60 outline-none transition-colors hover:text-foreground focus-visible:ring-2 focus-visible:ring-ring"
          >
            <HelpCircleIcon className="h-3.5 w-3.5" />
          </button>
        </TooltipTrigger>
        <TooltipContent side="top" className="max-w-64 text-xs leading-relaxed">
          {tip}
        </TooltipContent>
      </Tooltip>
    </div>
  );
}

function TeamSelectItem({
  value,
  children,
  disabledReason,
}: {
  value: string;
  children: ReactNode;
  disabledReason?: string | null;
}) {
  return (
    <SelectItem value={value} disabled={Boolean(disabledReason)}>
      <span className="min-w-0 truncate">{children}</span>
      {disabledReason && <span className="ml-auto shrink-0 text-xs text-muted-foreground">No Support access</span>}
    </SelectItem>
  );
}

export function TeamInboxDialog({
  workspaceId,
  open,
  onOpenChange,
  mailbox,
}: {
  workspaceId: string;
  open: boolean;
  onOpenChange: (open: boolean) => void;
  mailbox?: SupportMailbox | null;
}) {
  const [step, setStep] = useState<1 | 2 | 3>(1);
  const [form, setForm] = useState<MailboxFormState>(DEFAULT_FORM);
  const [memberPickerOpen, setMemberPickerOpen] = useState(false);
  const [manualRuleDraft, setManualRuleDraft] = useState<ManualRoutingRuleDraft | null>(null);
  const [deletedManualRuleIds, setDeletedManualRuleIds] = useState<string[]>([]);

  const currentUserId = useAuthStore((state) => state.user?.id);
  const workspaceSlug = useWorkspaceStore((state) => state.currentWorkspace?.slug);
  const { data: members = [] } = useAssignableMembers(workspaceId);
  const { teams, userMemberships } = useWorkspaceTeams(workspaceId);
  const { data: moduleAccess } = useWorkspaceModuleAccess(workspaceId);
  const { data: chatSettings } = useChatSettings(workspaceId);
  const { data: triageRules = [] } = useSupportTriageRules(workspaceId);
  const { data: mailboxMembersData = EMPTY_MEMBERS } = useMailboxMembers(workspaceId, mailbox?.id);
  const mailboxMembers = Array.isArray(mailboxMembersData) ? mailboxMembersData : EMPTY_MEMBERS;
  const createMailbox = useCreateMailbox(workspaceId);
  const updateMailbox = useUpdateMailbox(workspaceId);
  const createTriageRule = useCreateSupportTriageRule(workspaceId);
  const updateTriageRule = useUpdateSupportTriageRule(workspaceId);
  const deleteTriageRule = useDeleteSupportTriageRule(workspaceId);
  const supportAccessHref = workspaceSlug ? `/w/${workspaceSlug}/settings/access` : '/workspaces';
  const supportRoutingHref = workspaceSlug ? `/w/${workspaceSlug}/settings/inboxes-routing?tab=routing` : '/workspaces';
  const isRoutingOff = chatSettings ? !chatSettings.settings.triage_enabled : false;

  useEffect(() => {
    if (!open) {
      setForm(DEFAULT_FORM);
      setStep(1);
      setMemberPickerOpen(false);
      setManualRuleDraft(null);
      setDeletedManualRuleIds([]);
      return;
    }
    setForm(buildFormState(mailbox));
    setStep(1);
    setMemberPickerOpen(false);
    setDeletedManualRuleIds([]);
  }, [open, mailbox]);

  const mailboxRoutingRules = useMemo(
    () => triageRules.filter((rule) => mailbox?.id && rule.target_mailbox_id === mailbox.id),
    [mailbox?.id, triageRules],
  );

  useEffect(() => {
    if (!open) return;
    if (!mailbox) {
      setManualRuleDraft(null);
      return;
    }
    const draft = buildRoutingRuleDraft(mailboxRoutingRules);
    const sortedRules = [...mailboxRoutingRules].sort((a, b) => a.priority - b.priority);
    setManualRuleDraft(draft);
    setDeletedManualRuleIds(sortedRules.slice(1).map((rule) => rule.id));
  }, [mailbox, mailboxRoutingRules, open]);

  useEffect(() => {
    if (!mailbox) return;
    setForm((current) => ({
      ...current,
      workspaceMemberIds: mailboxMembers.map((member) => member.workspace_member_id),
    }));
  }, [mailbox, mailboxMembers]);

  const activeMembers = useMemo(
    () => members.filter((member) => member.status === 'active'),
    [members],
  );

  const supportGrants = useMemo(
    () => moduleAccess?.grants.filter((grant) => grant.module === 'support') ?? [],
    [moduleAccess?.grants],
  );

  const supportTeamOptions = useMemo(
    () => buildSupportTeamOptions(teams, supportGrants),
    [supportGrants, teams],
  );

  const supportAccessibleMembers = useMemo(
    () => filterSupportAccessibleMembers(activeMembers, supportGrants, userMemberships),
    [activeMembers, supportGrants, userMemberships],
  );

  const currentWorkspaceMember = useMemo(
    () => activeMembers.find((member) => member.user_id === currentUserId),
    [activeMembers, currentUserId],
  );

  const supportAccessibleMemberIDs = useMemo(
    () => new Set(supportAccessibleMembers.map((member) => member.id)),
    [supportAccessibleMembers],
  );

  const additionalMembers = useMemo(
    () =>
      filterMembersOutsideLinkedTeam(activeMembers, form.linkedTeamId, userMemberships),
    [activeMembers, form.linkedTeamId, userMemberships],
  );

  const additionalMemberOptions = useMemo(
    () => buildSupportMemberOptions(additionalMembers, supportGrants, userMemberships),
    [additionalMembers, supportGrants, userMemberships],
  );

  const additionalMemberIDs = useMemo(
    () => new Set(additionalMembers.map((member) => member.id)),
    [additionalMembers],
  );

  useEffect(() => {
    if (!open || mailbox || !currentWorkspaceMember) return;
    if (!additionalMemberIDs.has(currentWorkspaceMember.id)) return;

    setForm((current) => {
      if (current.workspaceMemberIds.includes(currentWorkspaceMember.id)) return current;
      return {
        ...current,
        workspaceMemberIds: [...current.workspaceMemberIds, currentWorkspaceMember.id],
      };
    });
  }, [additionalMemberIDs, currentWorkspaceMember, mailbox, open]);

  const teamMemberCounts = useMemo(() => {
    const counts = new Map<string, number>();
    for (const membership of userMemberships) {
      counts.set(membership.team_id, (counts.get(membership.team_id) ?? 0) + 1);
    }
    return counts;
  }, [userMemberships]);

  const supportedWorkspaceMemberIds = useMemo(
    () => form.workspaceMemberIds.filter((id) => supportAccessibleMemberIDs.has(id)),
    [form.workspaceMemberIds, supportAccessibleMemberIDs],
  );

  const selectedAdditionalWorkspaceMemberIds = useMemo(
    () => form.workspaceMemberIds.filter((id) => additionalMemberIDs.has(id)),
    [additionalMemberIDs, form.workspaceMemberIds],
  );

  const supportedAdditionalWorkspaceMemberIds = useMemo(
    () => selectedAdditionalWorkspaceMemberIds.filter((id) => supportAccessibleMemberIDs.has(id)),
    [selectedAdditionalWorkspaceMemberIds, supportAccessibleMemberIDs],
  );

  const inaccessibleSelectedMemberCount = form.workspaceMemberIds.length - supportedWorkspaceMemberIds.length;
  const selectedAdditionalMemberCount = selectedAdditionalWorkspaceMemberIds.length;

  const selectedLinkedTeamHasSupportAccess =
    form.linkedTeamId === 'none' ||
    supportTeamOptions.some((option) => option.team.id === form.linkedTeamId && option.hasSupportAccess);

  const inaccessibleTeamCount = supportTeamOptions.filter((option) => !option.hasSupportAccess).length;
  const inaccessibleMemberCount = Math.max(activeMembers.length - supportAccessibleMembers.length, 0);
  const accessRestrictionParts = [
    inaccessibleTeamCount > 0 ? `${inaccessibleTeamCount} ${inaccessibleTeamCount === 1 ? 'team' : 'teams'}` : null,
    inaccessibleMemberCount > 0 ? `${inaccessibleMemberCount} ${inaccessibleMemberCount === 1 ? 'member' : 'members'}` : null,
  ].filter((part): part is string => Boolean(part));
  const accessRestrictionSummary =
    accessRestrictionParts.length > 0
      ? `${accessRestrictionParts.join(' and ')} cannot be selected because ${inaccessibleTeamCount + inaccessibleMemberCount === 1 ? 'it does' : 'they do'} not have access to the Support module.`
      : '';

  const availableAdditionalMembers = useMemo(
    () => splitMailboxMembersBySelection(additionalMemberOptions, selectedAdditionalWorkspaceMemberIds, (option) => option.member.id).available,
    [additionalMemberOptions, selectedAdditionalWorkspaceMemberIds],
  );

  const selectedMembers = useMemo(
    () => splitMailboxMembersBySelection(additionalMemberOptions, selectedAdditionalWorkspaceMemberIds, (option) => option.member.id).selected,
    [additionalMemberOptions, selectedAdditionalWorkspaceMemberIds],
  );

  const canProceedToStep2 = form.name.trim().length > 0 && form.handle.trim().length > 0;

  const toggleMember = (memberId: string, checked: boolean) => {
    setForm((current) => ({
      ...current,
      workspaceMemberIds: checked
        ? [...current.workspaceMemberIds, memberId]
        : current.workspaceMemberIds.filter((id) => id !== memberId),
    }));
  };

  const nextManualRulePriority = () => {
    const maxExisting = triageRules.reduce((max, rule) => Math.max(max, rule.priority), 0);
    return Math.max(maxExisting, manualRuleDraft?.priority ?? 0) + 1;
  };

  const addManualCondition = () => {
    setManualRuleDraft((current) => {
      if (!current) return createEmptyRoutingRuleDraft(nextManualRulePriority());
      const draft = current;
      return {
        ...draft,
        conditions: [...draft.conditions, createRoutingCondition()],
      };
    });
  };

  const updateManualCondition = (conditionId: string, patch: Partial<ManualRoutingConditionDraft>) => {
    setManualRuleDraft((current) => {
      if (!current) return current;
      return {
        ...current,
        conditions: current.conditions.map((condition) =>
          condition.id === conditionId ? { ...condition, ...patch } : condition,
        ),
      };
    });
  };

  const removeManualCondition = (conditionId: string) => {
    setManualRuleDraft((current) => {
      if (!current) return current;
      const nextConditions = current.conditions.filter((condition) => condition.id !== conditionId);
      if (nextConditions.length > 0) {
        return { ...current, conditions: nextConditions };
      }
      if (current.persistedId) {
        setDeletedManualRuleIds((deleted) =>
          deleted.includes(current.persistedId!) ? deleted : [...deleted, current.persistedId!],
        );
      }
      return null;
    });
  };

  const validateManualRules = () => {
    if (!manualRuleDraft) return true;
    const hasCondition = manualRuleDraft.conditions.some(
      (condition) =>
        condition.values.some((value) => value.trim()) || condition.inputValue.trim(),
    );
    if (!hasCondition) {
      toast.error('Add at least one manual match');
      return false;
    }
    return true;
  };

  const buildManualRulePayload = (
    rule: ManualRoutingRuleDraft,
    targetMailboxId: string,
  ): CreateSupportTriageRuleRequest => ({
    name: `Route to ${form.name.trim() || 'inbox'}`,
    priority: rule.priority,
    active: rule.active,
    channels: [],
    target_mailbox_id: targetMailboxId,
    conditions: {
      condition_logic: rule.conditionLogic,
      phrase_contains: rule.conditions
        .filter((condition) => condition.type === 'message_contains')
        .flatMap((condition) => [
          ...condition.values,
          ...(condition.inputValue.trim() ? [condition.inputValue] : []),
        ])
        .map((value) => value.trim())
        .filter(Boolean),
      email_domain_equals: [],
      sender_email_contains: rule.conditions
        .filter((condition) => condition.type === 'sender_email_contains')
        .flatMap((condition) => [
          ...condition.values,
          ...(condition.inputValue.trim() ? [condition.inputValue] : []),
        ])
        .map((value) => value.trim().toLowerCase())
        .filter(Boolean),
    },
  });

  const syncManualRules = async (targetMailboxId: string) => {
    for (const ruleId of deletedManualRuleIds) {
      await deleteTriageRule.mutateAsync(ruleId);
    }

    if (!manualRuleDraft) return;

    const payload = buildManualRulePayload(manualRuleDraft, targetMailboxId);
    const hasConditions =
      payload.conditions.phrase_contains.length > 0 ||
      payload.conditions.email_domain_equals.length > 0 ||
      (payload.conditions.sender_email_contains?.length ?? 0) > 0;
    if (!hasConditions) return;

    if (manualRuleDraft.persistedId) {
      await updateTriageRule.mutateAsync({ ruleId: manualRuleDraft.persistedId, payload });
    } else {
      await createTriageRule.mutateAsync(payload);
    }
  };

  const goToStep2 = () => {
    if (!form.name.trim()) {
      toast.error('Inbox name is required');
      return;
    }
    if (!form.handle.trim()) {
      toast.error('Inbox handle is required');
      return;
    }
    setStep(2);
  };

  const submit = async () => {
    if (!form.name.trim()) {
      toast.error('Inbox name is required');
      return;
    }
    if (!form.handle.trim()) {
      toast.error('Inbox handle is required');
      return;
    }
    if (!validateManualRules()) return;

    if (mailbox) {
      const payload: UpdateSupportMailboxRequest = {
        name: form.name.trim(),
        handle: normalizeHandle(form.handle),
        icon: form.icon,
        description: form.description.trim() || null,
        routing_prompt: form.routingPrompt.trim() || null,
        triage_eligible: form.triageEligible,
        linked_team_id: form.linkedTeamId === 'none' || !selectedLinkedTeamHasSupportAccess ? null : form.linkedTeamId,
        assignment_mode: form.assignmentMode,
        workspace_member_ids: supportedAdditionalWorkspaceMemberIds,
      };
      if (form.replyTimeOverride) {
        payload.reply_time_preset = form.replyTimePreset;
        payload.reply_time_custom_minutes = form.replyTimePreset === 'custom' ? form.replyTimeCustomMinutes : null;
        payload.clear_reply_time_custom_minutes = form.replyTimePreset !== 'custom';
      } else {
        payload.clear_reply_time_preset = true;
        payload.clear_reply_time_custom_minutes = true;
      }
      await updateMailbox.mutateAsync({ mailboxId: mailbox.id, payload });
      await syncManualRules(mailbox.id);
      toast.success('Team inbox updated');
    } else {
      const payload: CreateSupportMailboxRequest = {
        name: form.name.trim(),
        handle: normalizeHandle(form.handle),
        icon: form.icon,
        description: form.description.trim() || null,
        routing_prompt: form.routingPrompt.trim() || null,
        triage_eligible: form.triageEligible,
        linked_team_id: form.linkedTeamId === 'none' || !selectedLinkedTeamHasSupportAccess ? null : form.linkedTeamId,
        assignment_mode: form.assignmentMode,
        workspace_member_ids: supportedAdditionalWorkspaceMemberIds,
      };
      const createdMailbox = await createMailbox.mutateAsync(payload);
      await syncManualRules(createdMailbox.id);
      toast.success('Team inbox created');
    }

    onOpenChange(false);
  };

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="overflow-hidden p-0 sm:max-w-2xl">
        <div className="flex items-center gap-0 border-b px-6 pb-4 pt-6">
          <button
            type="button"
            onClick={() => setStep(1)}
            className="flex items-center gap-2 text-sm font-medium"
          >
            <span className={`flex h-6 w-6 items-center justify-center rounded-full text-xs font-semibold transition-colors ${step === 1 ? 'bg-primary text-primary-foreground' : 'bg-primary/10 text-primary'}`}>
              {step > 1 ? <Tick01Icon className="h-3.5 w-3.5" /> : '1'}
            </span>
            <span className={step === 1 ? 'text-foreground' : 'text-muted-foreground'}>{DIALOG_STEPS[0].label}</span>
          </button>
          <div className="mx-3 h-px w-8 bg-border" />
          <button
            type="button"
            onClick={() => {
              if (canProceedToStep2) setStep(2);
            }}
            className="flex items-center gap-2 text-sm font-medium"
          >
            <span className={`flex h-6 w-6 items-center justify-center rounded-full text-xs font-semibold transition-colors ${step === 2 ? 'bg-primary text-primary-foreground' : step > 2 ? 'bg-primary/10 text-primary' : 'bg-muted text-muted-foreground'}`}>
              {step > 2 ? <Tick01Icon className="h-3.5 w-3.5" /> : '2'}
            </span>
            <span className={step === 2 ? 'text-foreground' : 'text-muted-foreground'}>{DIALOG_STEPS[1].label}</span>
          </button>
          <div className="mx-3 h-px w-8 bg-border" />
          <button
            type="button"
            onClick={() => {
              if (canProceedToStep2) setStep(3);
            }}
            className="flex items-center gap-2 text-sm font-medium"
          >
            <span className={`flex h-6 w-6 items-center justify-center rounded-full text-xs font-semibold transition-colors ${step === 3 ? 'bg-primary text-primary-foreground' : 'bg-muted text-muted-foreground'}`}>
              3
            </span>
            <span className={step === 3 ? 'text-foreground' : 'text-muted-foreground'}>{DIALOG_STEPS[2].label}</span>
          </button>
        </div>

        {step === 1 && (
          <div className="px-6 pb-2">
            <DialogHeader className="mb-4">
              <DialogTitle>{mailbox ? 'Edit Team Inbox' : 'Create Team Inbox'}</DialogTitle>
              <DialogDescription>{DIALOG_STEPS[0].description}</DialogDescription>
            </DialogHeader>

            <div className="space-y-4">
              <div className="flex items-end gap-3">
                <div className="space-y-1.5">
                  <Label>Icon</Label>
                  <IconPicker value={form.icon} onChange={(icon) => setForm((current) => ({ ...current, icon }))} />
                </div>
                <div className="flex-1 space-y-1.5">
                  <Label htmlFor="team-inbox-name">Name</Label>
                  <Input
                    id="team-inbox-name"
                    value={form.name}
                    onChange={(event) => setForm((current) => {
                      const nextName = event.target.value;
                      return {
                        ...current,
                        name: nextName,
                        handle: mailbox ? current.handle : normalizeHandle(nextName),
                      };
                    })}
                    placeholder="Technical Support"
                  />
                </div>
              </div>

              <div className="space-y-1.5">
                <FieldLabel htmlFor="team-inbox-handle" tip="A unique, URL-safe identifier for this inbox. Used in routing rules and API references. Auto-generated from the name but you can customize it.">
                  Handle
                </FieldLabel>
                <div className="relative">
                  <span className="absolute left-3 top-1/2 -translate-y-1/2 text-sm text-muted-foreground">#</span>
                  <Input
                    id="team-inbox-handle"
                    value={form.handle}
                    onChange={(event) => setForm((current) => ({ ...current, handle: normalizeHandle(event.target.value) }))}
                    placeholder="technical-support"
                    className="pl-7"
                  />
                </div>
              </div>

              <div className="space-y-1.5">
                <FieldLabel htmlFor="team-inbox-description" tip="An internal note for your team explaining what this inbox is for. Not shown to customers.">
                  Description <span className="font-normal text-muted-foreground">(optional)</span>
                </FieldLabel>
                <Textarea
                  id="team-inbox-description"
                  value={form.description}
                  onChange={(event) => setForm((current) => ({ ...current, description: event.target.value }))}
                  placeholder="Specialist queue for escalations and technical debugging."
                  rows={3}
                />
              </div>

              <div className="rounded-xl border bg-muted/20 p-3">
                  <div className="flex items-start justify-between gap-4">
                    <div className="space-y-0.5">
                      <FieldLabel tip="Override the workspace reply-time expectation for conversations in this inbox. Leave off to use the workspace default; turn on to give this inbox its own SLA.">
                        Reply expectations
                      </FieldLabel>
                      <p className="text-xs text-muted-foreground">
                        {form.replyTimeOverride
                          ? 'Using a custom reply-time SLA for this inbox.'
                          : 'Using the workspace default reply-time SLA.'}
                      </p>
                    </div>
                    <Switch
                      checked={form.replyTimeOverride}
                      onCheckedChange={(checked) => setForm((c) => ({ ...c, replyTimeOverride: checked }))}
                    />
                  </div>

                  {form.replyTimeOverride && (
                    <div className="mt-3 flex items-center gap-2">
                      <Select
                        value={form.replyTimePreset}
                        onValueChange={(v) => setForm((c) => ({ ...c, replyTimePreset: v }))}
                      >
                        <SelectTrigger className={form.replyTimePreset === 'custom' ? 'w-44' : 'flex-1'}><SelectValue /></SelectTrigger>
                        <SelectContent>
                          <SelectItem value="few_minutes">Usually a few minutes</SelectItem>
                          <SelectItem value="few_hours">Usually a few hours</SelectItem>
                          <SelectItem value="same_day">Within a day</SelectItem>
                          <SelectItem value="custom">Custom…</SelectItem>
                        </SelectContent>
                      </Select>
                      {form.replyTimePreset === 'custom' && (
                        <div className="flex shrink-0 items-center gap-2">
                          <Input
                            type="number"
                            min={1}
                            max={10080}
                            value={form.replyTimeCustomMinutes ?? ''}
                            onChange={(e) => {
                              const v = e.target.value;
                              setForm((c) => ({
                                ...c,
                                replyTimeCustomMinutes: v === '' ? null : Number(v),
                              }));
                            }}
                            className="h-9 w-16 text-sm"
                            placeholder="30"
                          />
                          <span className="text-xs text-muted-foreground">minutes</span>
                        </div>
                      )}
                    </div>
                  )}
              </div>
            </div>
          </div>
        )}

        {step === 2 && (
          <div className="max-h-[calc(90svh-8rem)] overflow-y-auto px-6 pb-2 pr-5">
            <DialogHeader className="mb-4">
              <DialogTitle>Members & Assignment</DialogTitle>
              <DialogDescription>
                Choose who can access this inbox and how conversations are assigned.
              </DialogDescription>
            </DialogHeader>

            <div className="mb-4 space-y-3">
              {(accessRestrictionSummary || !selectedLinkedTeamHasSupportAccess || inaccessibleSelectedMemberCount > 0) && (
                <div className="rounded-lg border border-amber-200 bg-amber-50 px-3 py-2 text-xs leading-relaxed text-amber-900 dark:border-amber-900/50 dark:bg-amber-950/30 dark:text-amber-100">
                  {accessRestrictionSummary && <span className="font-medium">{accessRestrictionSummary}</span>}
                  {!selectedLinkedTeamHasSupportAccess && (
                    <span className="ml-1">
                      The selected team will be removed on save unless Support access is added first.
                    </span>
                  )}
                  {inaccessibleSelectedMemberCount > 0 && (
                    <span className="ml-1">
                      {inaccessibleSelectedMemberCount} selected {inaccessibleSelectedMemberCount === 1 ? 'member no longer has' : 'members no longer have'} Support access and will be removed on save.
                    </span>
                  )}
                  <a
                    href={supportAccessHref}
                    target="_blank"
                    rel="noreferrer"
                    className="mt-1 block font-medium underline underline-offset-2"
                  >
                    Open module access settings
                  </a>
                </div>
              )}

              <div className="space-y-1.5">
                <FieldLabel tip="Choose which team gets access to this inbox. Team members are included automatically, and you can add extra people below.">
                  Team Access
                </FieldLabel>
                <Select value={form.linkedTeamId} onValueChange={(value) => setForm((current) => ({ ...current, linkedTeamId: value }))}>
                  <SelectTrigger className="w-full">
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent className="max-h-72">
                    <SelectItem value="none">No linked team</SelectItem>
                    {supportTeamOptions.map(({ team, disabledReason }) => (
                      <TeamSelectItem key={team.id} value={team.id} disabledReason={disabledReason}>
                        {team.name} · {teamMemberCounts.get(team.id) ?? 0} members
                      </TeamSelectItem>
                    ))}
                  </SelectContent>
                </Select>
              </div>
            </div>

            <div className="space-y-2">
              <div className="flex items-center justify-between gap-3">
                <div>
                  <h3 className="text-sm font-medium">
                    Additional members outside team
                    {selectedAdditionalMemberCount > 0 && (
                      <span className="ml-1 text-muted-foreground">({selectedAdditionalMemberCount})</span>
                    )}
                  </h3>
                </div>
                <Popover open={memberPickerOpen} onOpenChange={setMemberPickerOpen}>
                  <PopoverTrigger asChild>
                    <Button
                      type="button"
                      variant="outline"
                      size="sm"
                      className="shrink-0 gap-2"
                      disabled={availableAdditionalMembers.length === 0}
                    >
                      <PlusSignIcon className="h-4 w-4" />
                      Add member
                    </Button>
                  </PopoverTrigger>
                  <PopoverContent align="end" className="w-80 p-0">
                    <Command>
                      <CommandInput placeholder="Search members..." />
                      <CommandList>
                        <CommandEmpty>No eligible members found.</CommandEmpty>
                        <CommandGroup>
                          {availableAdditionalMembers.map(({ member, disabledReason }) => {
                            const displayName = member.display_name || member.email || 'Unknown';
                            const item = (
                              <CommandItem
                                key={member.id}
                                value={`${displayName} ${member.email ?? ''}`}
                                disabled={Boolean(disabledReason)}
                                onSelect={() => {
                                  toggleMember(member.id, true);
                                }}
                                className="gap-3"
                              >
                                <UserAvatar
                                  name={displayName}
                                  avatarUrl={member.avatar_url}
                                  avatarStyle={member.avatar_style}
                                  avatarSeed={member.avatar_seed}
                                  avatarBackgroundMode={member.avatar_background_mode}
                                  avatarBackgroundColor={member.avatar_background_color}
                                  className="h-6 w-6 border-border/70"
                                  fallbackClassName="text-[10px]"
                                />
                                <div className="min-w-0 flex-1">
                                  <p className="truncate font-medium">{displayName}</p>
                                  {member.email && member.display_name && (
                                    <p className="truncate text-xs text-muted-foreground">{member.email}</p>
                                  )}
                                </div>
                              </CommandItem>
                            );
                            if (!disabledReason) return item;
                            return (
                              <Tooltip key={member.id}>
                                <TooltipTrigger asChild>
                                  <div>{item}</div>
                                </TooltipTrigger>
                                <TooltipContent side="left" className="max-w-56">
                                  {disabledReason}
                                </TooltipContent>
                              </Tooltip>
                            );
                          })}
                        </CommandGroup>
                      </CommandList>
                    </Command>
                  </PopoverContent>
                </Popover>
              </div>

              <div className="max-h-72 min-h-32 overflow-y-auto rounded-lg border">
                {selectedMembers.length === 0 ? (
                  <p className="px-3 py-8 text-center text-sm text-muted-foreground">
                    No additional members selected
                  </p>
                ) : (
                  <div className="divide-y divide-border/70">
                    {selectedMembers.map(({ member, disabledReason }) => {
                      const displayName = member.display_name || member.email || 'Unknown';
                      const isCurrentUser = member.id === currentWorkspaceMember?.id;
                      const row = (
                        <div key={member.id} className={`flex items-center gap-3 px-3 py-2.5 text-sm ${disabledReason ? 'opacity-60' : ''}`}>
                          <UserAvatar
                            name={displayName}
                            avatarUrl={member.avatar_url}
                            avatarStyle={member.avatar_style}
                            avatarSeed={member.avatar_seed}
                            avatarBackgroundMode={member.avatar_background_mode}
                            avatarBackgroundColor={member.avatar_background_color}
                            className="h-7 w-7 border-border/70"
                            fallbackClassName="text-[10px]"
                          />
                          <div className="min-w-0 flex-1">
                            <div className="flex min-w-0 items-center gap-2">
                              <p className="truncate font-medium">{displayName}</p>
                              {isCurrentUser && (
                                <span className="shrink-0 rounded bg-muted px-1.5 py-0.5 text-[10px] font-medium text-muted-foreground">
                                  You
                                </span>
                              )}
                            </div>
                            {member.email && member.display_name && (
                              <p className="truncate text-xs text-muted-foreground">{member.email}</p>
                            )}
                          </div>
                          {!isCurrentUser && (
                            <Button
                              type="button"
                              variant="ghost"
                              size="icon"
                              className="h-7 w-7 shrink-0 text-muted-foreground hover:text-foreground"
                              onClick={() => toggleMember(member.id, false)}
                              aria-label={`Remove ${displayName}`}
                            >
                              <Cancel01Icon className="h-4 w-4" />
                            </Button>
                          )}
                        </div>
                      );
                      if (!disabledReason) return row;
                      return (
                        <Tooltip key={member.id}>
                          <TooltipTrigger asChild>{row}</TooltipTrigger>
                          <TooltipContent side="left" className="max-w-56">
                            {disabledReason}
                          </TooltipContent>
                        </Tooltip>
                      );
                    })}
                  </div>
                )}
              </div>
            </div>

            <div className="mt-4 space-y-1.5">
              <FieldLabel tip="Choose whether conversations stay unassigned for agents to pick up, or are automatically distributed across inbox members.">
                Assignment
              </FieldLabel>
              <Select value={form.assignmentMode} onValueChange={(value: 'manual' | 'round_robin') => setForm((current) => ({ ...current, assignmentMode: value }))}>
                <SelectTrigger className="w-full">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="manual">Manual</SelectItem>
                  <SelectItem value="round_robin">Round robin</SelectItem>
                </SelectContent>
              </Select>
            </div>
          </div>
        )}

        {step === 3 && (
          <div className="max-h-[calc(90svh-8rem)] overflow-y-auto px-6 pb-2 pr-5">
            <DialogHeader className="mb-4">
              <DialogTitle>Routing</DialogTitle>
              <DialogDescription>
                Set how conversations reach this inbox using manual rules and AI routing.
              </DialogDescription>
            </DialogHeader>

            <div className="space-y-4">
              {isRoutingOff && (
                <div className="flex flex-col gap-2 rounded-lg border border-amber-200 bg-amber-50 px-3 py-2 text-xs text-amber-900 sm:flex-row sm:items-center sm:justify-between dark:border-amber-900/60 dark:bg-amber-950/30 dark:text-amber-200">
                  <span>
                    Automated routing is off. You can save rule-based conditions and AI routing prompts, but they will not run until Automated routing is enabled in settings.
                  </span>
                  <Button asChild type="button" variant="outline" size="xs" className="h-7 w-fit shrink-0 border-amber-300 bg-amber-50 text-amber-950 hover:bg-amber-100 dark:border-amber-800 dark:bg-amber-950/20 dark:text-amber-100 dark:hover:bg-amber-900/40">
                    <a href={supportRoutingHref} target="_blank" rel="noreferrer">
                      Open settings
                    </a>
                  </Button>
                </div>
              )}

              <div className="space-y-2">
                <div className="flex items-center justify-between gap-3">
                  <div>
                    <h3 className="text-sm font-medium">Rule-based routing</h3>
                  </div>
                  <Button type="button" variant="outline" size="sm" className="shrink-0 gap-2" onClick={addManualCondition}>
                    <PlusSignIcon className="h-4 w-4" />
                    Add condition
                  </Button>
                </div>
                <div className="space-y-2">
                  {!manualRuleDraft || manualRuleDraft.conditions.length === 0 ? (
                    <div className="rounded-lg border bg-muted/20 px-3 py-3 text-center text-sm text-muted-foreground">
                      No manual rules yet.
                    </div>
                  ) : (
                    <div className="space-y-2">
                      <div className="flex items-center gap-2">
                        <Select
                          value={manualRuleDraft.conditionLogic}
                          onValueChange={(value) =>
                            setManualRuleDraft((current) =>
                              current ? { ...current, conditionLogic: value as 'any' | 'all' } : current,
                            )
                          }
                        >
                          <SelectTrigger className="h-8 w-52">
                            <SelectValue />
                          </SelectTrigger>
                          <SelectContent>
                            <SelectItem value="any">Any condition matches</SelectItem>
                            <SelectItem value="all">All conditions match</SelectItem>
                          </SelectContent>
                        </Select>
                      </div>
                      {manualRuleDraft.conditions.map((condition, index) => (
                        <div key={condition.id} className="space-y-2">
                          {index > 0 && (
                            <div className="flex items-center gap-2 px-1">
                              <div className="h-px flex-1 bg-border" />
                              <span className="text-[10px] font-semibold text-muted-foreground">
                                {manualRuleDraft.conditionLogic === 'any' ? 'OR' : 'AND'}
                              </span>
                              <div className="h-px flex-1 bg-border" />
                            </div>
                          )}
                          <div className="grid gap-2 rounded-lg border p-2 sm:grid-cols-[140px_minmax(0,1fr)_32px]">
                            <Select
                              value={condition.type}
                              onValueChange={(value) =>
                                updateManualCondition(condition.id, {
                                  type: value as ManualRoutingConditionDraft['type'],
                                  values: [],
                                  inputValue: '',
                                })
                              }
                            >
                              <SelectTrigger className="h-9">
                                <SelectValue />
                              </SelectTrigger>
                            <SelectContent>
                              <SelectItem value="message_contains">Text contains</SelectItem>
                              <SelectItem value="sender_email_contains">Email ID contains</SelectItem>
                            </SelectContent>
                          </Select>
                          <ChipInput
                            value={condition.values}
                            onValueChange={(values) => updateManualCondition(condition.id, { values })}
                            inputValue={condition.inputValue}
                            onInputValueChange={(inputValue) => updateManualCondition(condition.id, { inputValue })}
                            placeholder={condition.type === 'sender_email_contains' ? '@example.com, billing@acme.com' : 'refund, cancel my plan, great work...'}
                            normalize={condition.type === 'sender_email_contains' ? (value) => value.toLowerCase() : undefined}
                            className="min-h-9 py-1.5"
                          />
                            <Button
                              type="button"
                              variant="ghost"
                              size="icon"
                              className="h-9 w-9 text-muted-foreground hover:text-foreground"
                              onClick={() => removeManualCondition(condition.id)}
                              aria-label="Remove match"
                            >
                              <Cancel01Icon className="h-4 w-4" />
                            </Button>
                          </div>
                        </div>
                      ))}
                    </div>
                  )}
                </div>
              </div>

              <div className="space-y-3 pt-2">
                <div className="flex items-start justify-between gap-4">
                  <div>
                    <h3 className="text-sm font-medium">AI routing</h3>
                    <p className="text-xs text-muted-foreground">
                      When manual rules do not match, AI uses this description to choose the best inbox.
                    </p>
                  </div>
                  <Switch
                    checked={form.triageEligible}
                    onCheckedChange={(checked) => setForm((current) => ({ ...current, triageEligible: checked }))}
                  />
                </div>

                {form.triageEligible && (
                  <div className="space-y-3">
                    <Textarea
                      id="team-inbox-routing-prompt"
                      value={form.routingPrompt}
                      onChange={(event) => setForm((current) => ({ ...current, routingPrompt: event.target.value }))}
                      placeholder="Describe what AI should route to this inbox. Example: billing questions, refund requests, failed payments, invoice issues, subscription changes, and customers asking why they were charged."
                      rows={6}
                    />
                  </div>
                )}
              </div>
            </div>
          </div>
        )}

        <DialogFooter className="border-t px-6 py-4">
          {step === 1 && (
            <>
              <Button variant="outline" onClick={() => onOpenChange(false)}>
                Cancel
              </Button>
              <Button onClick={goToStep2} disabled={!canProceedToStep2} className="gap-2">
                Continue
                <ArrowRight02Icon className="h-4 w-4" />
              </Button>
            </>
          )}
          {step === 2 && (
            <>
              <Button variant="outline" onClick={() => setStep(1)} className="mr-auto gap-2">
                <ArrowLeft01Icon className="h-4 w-4" />
                Back
              </Button>
              <Button onClick={() => setStep(3)} className="gap-2">
                Continue
                <ArrowRight02Icon className="h-4 w-4" />
              </Button>
            </>
          )}
          {step === 3 && (
            <>
              <Button variant="outline" onClick={() => setStep(2)} className="mr-auto gap-2">
                <ArrowLeft01Icon className="h-4 w-4" />
                Back
              </Button>
              <Button
                onClick={submit}
                disabled={
                  createMailbox.isPending ||
                  updateMailbox.isPending ||
                  createTriageRule.isPending ||
                  updateTriageRule.isPending ||
                  deleteTriageRule.isPending
                }
              >
                {mailbox ? 'Save Changes' : 'Create Inbox'}
              </Button>
            </>
          )}
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
