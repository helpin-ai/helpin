import { useEffect, useMemo, useState, type ReactNode } from 'react';
import { ArrowRight02Icon, BotIcon, ArrowDown01Icon, InboxIcon, PencilEdit01Icon, PlusSignIcon, Settings02Icon, SparklesIcon, Delete01Icon, WorkflowSquare01Icon } from '@/lib/icons';
import { toast } from 'sonner';
import { TeamInboxDialog } from '@/components/support/TeamInboxDialog';
import { useConfirm } from '@/components/ui/confirm-dialog';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Checkbox } from '@/components/ui/checkbox';
import { ChipInput } from '@/components/ui/chip-input';
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { Switch } from '@/components/ui/switch';
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table';
import { cn } from '@/lib/utils';

import {
  useChatSettings,
  useCreateSupportTriageRule,
  useDeleteSupportTriageRule,
  useSupportMailboxes,
  useSupportTriageRules,
  useUpdateChatSettings,
  useUpdateSupportTriageRule,
} from '@/hooks/queries/useSupport';
import type {
  CreateSupportTriageRuleRequest,
  SupportInboxSettings,
  SupportMailbox,
  SupportTriageRule,
  SupportTriageRuleConditions,
  UpdateSupportTriageRuleRequest,
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
  triage_auto_move_enabled: false,
  triage_confidence_threshold: 0.9,
  triage_widget_enabled: true,
  triage_email_enabled: true,
  triage_internal_enabled: false,
  triage_fallback_behavior: 'shared',
  triage_rerun_on_meaning_change: false,
  triage_daily_budget: 250,
  triage_skip_spam_conversations: true,
  triage_deduplicate_first_message: true,
};

type RuleFormState = {
  name: string;
  priority: string;
  active: boolean;
  targetMailboxId: string;
  conditionLogic: 'any' | 'all';
  phraseChips: string[];
  domainChips: string[];
  phraseInput: string;
  domainInput: string;
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

function buildRuleFormState(rule: SupportTriageRule | null, mailboxes: SupportMailbox[]): RuleFormState {
  return {
    name: rule?.name ?? '',
    priority: String(rule?.priority ?? 0),
    active: rule?.active ?? true,
    targetMailboxId: rule?.target_mailbox_id ?? mailboxes[0]?.id ?? '',
    conditionLogic: rule?.conditions?.condition_logic === 'any' ? 'any' : 'all',
    phraseChips: [...(rule?.conditions?.phrase_contains ?? [])],
    domainChips: [
      ...(rule?.conditions?.sender_email_contains ?? []),
      ...(rule?.conditions?.email_domain_equals ?? []),
    ],
    phraseInput: '',
    domainInput: '',
  };
}

/* ── Rule Dialog ─────────────────────────────────────────────────────── */

function RuleDialog({
  open,
  onOpenChange,
  rule,
  mailboxes,
  isSaving,
  onSave,
}: {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  rule: SupportTriageRule | null;
  mailboxes: SupportMailbox[];
  isSaving: boolean;
  onSave: (payload: CreateSupportTriageRuleRequest | UpdateSupportTriageRuleRequest) => Promise<void>;
}) {
  const [form, setForm] = useState<RuleFormState>(() => buildRuleFormState(rule, mailboxes));

  useEffect(() => {
    if (!open) return;
    setForm(buildRuleFormState(rule, mailboxes));
  }, [open, rule, mailboxes]);

  const handleSubmit = async () => {
    if (!form.name.trim()) {
      toast.error('Rule name is required');
      return;
    }
    if (!form.targetMailboxId) {
      toast.error('Choose a target inbox');
      return;
    }

    const conditions: SupportTriageRuleConditions = {
      condition_logic: form.conditionLogic,
      phrase_contains: form.phraseChips,
      email_domain_equals: [],
      sender_email_contains: form.domainChips.map((v) => v.toLowerCase()),
    };
    if (
      conditions.phrase_contains.length === 0 &&
      conditions.email_domain_equals.length === 0 &&
      (conditions.sender_email_contains?.length ?? 0) === 0
    ) {
      toast.error('Add at least one condition');
      return;
    }

    const payload: CreateSupportTriageRuleRequest = {
      name: form.name.trim(),
      priority: Number(form.priority) || 0,
      active: form.active,
      target_mailbox_id: form.targetMailboxId,
      channels: [],
      conditions,
    };

    try {
      await onSave(rule ? payload : payload);
      onOpenChange(false);
    } catch {
      // Mutation hooks surface toast errors.
    }
  };

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-xl">
        <DialogHeader>
          <DialogTitle>{rule ? 'Edit Routing Rule' : 'New Routing Rule'}</DialogTitle>
          <DialogDescription>
            Rules run before AI triage. First active match wins.
          </DialogDescription>
        </DialogHeader>

        <div className="space-y-5">
          {/* ── Basics ─────────────────────────────────────── */}
          <div className="space-y-4">
            <p className="text-xs font-medium uppercase tracking-wider text-muted-foreground">Basics</p>
            <div className="grid gap-4 sm:grid-cols-2">
              <div className="space-y-2 sm:col-span-2">
                <Label htmlFor="triage-rule-name">Rule Name</Label>
                <Input
                  id="triage-rule-name"
                  value={form.name}
                  onChange={(event) => setForm((c) => ({ ...c, name: event.target.value }))}
                  placeholder="e.g. Guest posts"
                />
              </div>

              <div className="space-y-2">
                <Label htmlFor="triage-rule-target">Target Inbox</Label>
                <Select
                  value={form.targetMailboxId}
                  onValueChange={(value) => setForm((c) => ({ ...c, targetMailboxId: value }))}
                >
                  <SelectTrigger id="triage-rule-target">
                    <SelectValue placeholder="Select an inbox" />
                  </SelectTrigger>
                  <SelectContent>
                    {mailboxes.map((mailbox) => (
                      <SelectItem key={mailbox.id} value={mailbox.id}>
                        {mailbox.name}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              </div>

              <div className="space-y-2">
                <Label htmlFor="triage-rule-priority">Priority</Label>
                <Input
                  id="triage-rule-priority"
                  type="number"
                  min={0}
                  value={form.priority}
                  onChange={(event) => setForm((c) => ({ ...c, priority: event.target.value }))}
                />
              </div>
            </div>
          </div>

          {/* ── Conditions ─────────────────────────────────── */}
          <div className="space-y-4">
            <p className="text-xs font-medium uppercase tracking-wider text-muted-foreground">Conditions</p>

            <div className="space-y-2">
              <Label>Match Logic</Label>
              <Select
                value={form.conditionLogic}
                onValueChange={(value) => setForm((c) => ({ ...c, conditionLogic: value as 'any' | 'all' }))}
              >
                <SelectTrigger>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="any">Any condition matches</SelectItem>
                  <SelectItem value="all">All conditions match</SelectItem>
                </SelectContent>
              </Select>
            </div>

            <div className="space-y-2">
              <Label>Text Contains</Label>
              <ChipInput
                value={form.phraseChips}
                onValueChange={(v) => setForm((c) => ({ ...c, phraseChips: v }))}
                inputValue={form.phraseInput}
                onInputValueChange={(v) => setForm((c) => ({ ...c, phraseInput: v }))}
                placeholder="Type a keyword and press Enter..."
              />
              <p className="text-xs text-muted-foreground">Case-insensitive phrase matching against the conversation text.</p>
            </div>

            <div className="space-y-2">
              <Label>Email ID Contains</Label>
              <ChipInput
                value={form.domainChips}
                onValueChange={(v) => setForm((c) => ({ ...c, domainChips: v }))}
                inputValue={form.domainInput}
                onInputValueChange={(v) => setForm((c) => ({ ...c, domainInput: v }))}
                placeholder="Type an email fragment and press Enter..."
                normalize={(v) => v.toLowerCase()}
              />
              <p className="text-xs text-muted-foreground">Case-insensitive matching against the full sender email address.</p>
            </div>
          </div>

          {/* ── Enabled toggle ─────────────────────────────── */}
          <div className="flex items-start justify-between rounded-lg border p-3">
            <div className="space-y-0.5">
              <Label>Rule Enabled</Label>
              <p className="text-sm text-muted-foreground">Inactive rules stay saved but won't route conversations.</p>
            </div>
            <Switch
              checked={form.active}
              onCheckedChange={(checked) => setForm((c) => ({ ...c, active: checked }))}
            />
          </div>
        </div>

        <DialogFooter>
          <Button variant="outline" onClick={() => onOpenChange(false)}>
            Cancel
          </Button>
          <Button onClick={handleSubmit} disabled={isSaving}>
            {rule ? 'Save Rule' : 'Create Rule'}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
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
    <section className="overflow-hidden rounded-lg border bg-card">
      <div className="flex items-center border-b">
        <button
          type="button"
          onClick={() => onToggle(id)}
          className="flex min-w-0 flex-1 items-center gap-4 px-4 py-4 text-left transition-colors hover:bg-muted/35"
        >
          <span className="flex h-9 w-9 shrink-0 items-center justify-center rounded-md bg-muted text-muted-foreground">
            {icon}
          </span>
          <span className="min-w-0 flex-1">
            <span className="block text-sm font-medium text-foreground">{title}</span>
            <span className="mt-0.5 block text-sm text-muted-foreground">{description}</span>
          </span>
          {status}
          <ArrowDown01Icon className={cn(
            'h-4 w-4 shrink-0 text-muted-foreground transition-transform duration-150',
            expanded && 'rotate-180',
          )} />
        </button>
        {action ? <div className="shrink-0 pr-4">{action}</div> : null}
      </div>
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
    <div className={cn('grid gap-4 border-b px-5 py-4 last:border-b-0 md:grid-cols-[minmax(180px,0.42fr)_1fr]', className)}>
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
    <div className="space-y-4">
      <div className="flex items-center justify-between gap-3">
        <span className="text-sm text-muted-foreground">Minimum score for auto-routing</span>
        <span className="text-sm font-medium tabular-nums text-foreground">{percent}%</span>
      </div>
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
      <div className="grid grid-cols-3 gap-3 text-xs text-muted-foreground">
        <div>
          <p className="font-medium text-foreground">Low <span className="font-normal text-muted-foreground">(Manual Review)</span></p>
          <p>Most matches surface as inline suggestions.</p>
        </div>
        <div className="text-center">
          <p className="font-medium text-foreground">Balanced</p>
          <p>Confident matches auto-route, the rest stay inline.</p>
        </div>
        <div className="text-right">
          <p className="font-medium text-foreground">Aggressive <span className="font-normal text-muted-foreground">(Auto-routing)</span></p>
          <p>Lean into auto-move, fewer manual reviews.</p>
        </div>
      </div>
    </div>
  );
}

/* ── Main component ──────────────────────────────────────────────────── */

export function ConversationRoutingTab({ workspaceId }: { workspaceId: string }) {
  const confirm = useConfirm();
  const { data: installation, isLoading } = useChatSettings(workspaceId);
  const { data: mailboxes = [] } = useSupportMailboxes(workspaceId);
  const { data: rules = [] } = useSupportTriageRules(workspaceId);
  const updateSettings = useUpdateChatSettings(workspaceId);
  const createRule = useCreateSupportTriageRule(workspaceId);
  const updateRule = useUpdateSupportTriageRule(workspaceId);
  const deleteRule = useDeleteSupportTriageRule(workspaceId);

  const activeMailboxes = useMemo(
    () => mailboxes.filter((mailbox) => mailbox.active),
    [mailboxes],
  );

  const [draft, setDraft] = useState<RoutingSettingsDraft>(DEFAULT_ROUTING_SETTINGS);
  const [editingMailbox, setEditingMailbox] = useState<SupportMailbox | null>(null);
  const [mailboxDialogOpen, setMailboxDialogOpen] = useState(false);
  const [ruleDialogOpen, setRuleDialogOpen] = useState(false);
  const [editingRule, setEditingRule] = useState<SupportTriageRule | null>(null);

  const openCreateMailbox = () => {
    setEditingMailbox(null);
    setMailboxDialogOpen(true);
  };
  const openEditMailbox = (mailbox: SupportMailbox) => {
    setEditingMailbox(mailbox);
    setMailboxDialogOpen(true);
  };

  // Accordion state
  const [expandedSections, setExpandedSections] = useState<Set<string>>(
    new Set(['inbox-catalog', 'routing-rules', 'ai-triage', 'advanced']),
  );
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
  const controlsDisabled = !draft.triage_enabled || updateSettings.isPending;

  const handleSaveSettings = async () => {
    try {
      await updateSettings.mutateAsync(draft);
      toast.success('Conversation routing settings saved');
    } catch {
      // Mutation hook shows toast on failure.
    }
  };

  const openCreateRule = () => {
    setEditingRule(null);
    setRuleDialogOpen(true);
  };

  const openEditRule = (rule: SupportTriageRule) => {
    setEditingRule(rule);
    setRuleDialogOpen(true);
  };

  const handleDeleteRule = async (rule: SupportTriageRule) => {
    const ok = await confirm({
      title: `Delete ${rule.name}?`,
      description: 'This routing rule will be removed immediately.',
      confirmText: 'Delete',
      variant: 'destructive',
    });
    if (!ok) return;
    deleteRule.mutate(rule.id, {
      onSuccess: () => toast.success('Routing rule deleted'),
    });
  };

  const handleSaveRule = async (payload: CreateSupportTriageRuleRequest | UpdateSupportTriageRuleRequest) => {
    if (editingRule) {
      await updateRule.mutateAsync({ ruleId: editingRule.id, payload });
      toast.success('Routing rule updated');
      return;
    }
    await createRule.mutateAsync(payload as CreateSupportTriageRuleRequest);
    toast.success('Routing rule created');
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

      <RoutingSection
        id="inbox-catalog"
        title="Inboxes & Routing Prompts"
        description="Where conversations land and how AI identifies each inbox."
        icon={<InboxIcon className="h-4 w-4" />}
        status={<Badge variant="secondary" className="mr-2">{activeMailboxes.length + 1}</Badge>}
        action={(
          <Button size="sm" variant="outline" onClick={openCreateMailbox}>
            <PlusSignIcon className="mr-1.5 h-3.5 w-3.5" />
            Add Inbox
          </Button>
        )}
        expanded={isExpanded('inbox-catalog')}
        onToggle={toggleSection}
      >
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>Inbox</TableHead>
              <TableHead>Eligible</TableHead>
              <TableHead>Routing Prompt</TableHead>
              <TableHead className="w-[100px]" />
            </TableRow>
          </TableHeader>
          <TableBody>
            <TableRow>
              <TableCell>
                <div className="font-medium">Shared Inbox</div>
                <div className="text-xs text-muted-foreground">Default fallback queue</div>
              </TableCell>
              <TableCell><Badge variant="secondary">Always</Badge></TableCell>
              <TableCell className="max-w-[360px] whitespace-normal text-sm text-muted-foreground">
                General support and uncategorized conversations.
              </TableCell>
              <TableCell />
            </TableRow>
            {activeMailboxes.map((mailbox) => (
              <TableRow key={mailbox.id}>
                <TableCell>
                  <div className="font-medium">{mailbox.name}</div>
                  <div className="text-xs text-muted-foreground">
                    #{mailbox.handle}
                    {mailbox.linked_team_name ? ` · ${mailbox.linked_team_name}` : ''}
                  </div>
                </TableCell>
                <TableCell>
                  <Badge variant={mailbox.triage_eligible ? 'secondary' : 'outline'}>
                    {mailbox.triage_eligible ? 'Yes' : 'No'}
                  </Badge>
                </TableCell>
                <TableCell className="max-w-[360px] whitespace-normal text-sm text-muted-foreground">
                  {mailbox.routing_prompt?.trim() || mailbox.description?.trim() || 'No routing prompt configured.'}
                </TableCell>
                <TableCell>
                  <Button variant="ghost" size="sm" onClick={() => openEditMailbox(mailbox)}>
                    <PencilEdit01Icon className="mr-1.5 h-3.5 w-3.5" />
                    Edit
                  </Button>
                </TableCell>
              </TableRow>
            ))}
          </TableBody>
        </Table>
        <div className="flex items-center justify-center border-t px-5 py-3">
          <Button variant="ghost" size="sm" onClick={openCreateMailbox} className="text-muted-foreground hover:text-foreground">
            <PlusSignIcon className="mr-1.5 h-3.5 w-3.5" />
            Add Inbox
          </Button>
        </div>
      </RoutingSection>

      <RoutingSection
        id="routing-rules"
        title="Routing Rules"
        description="Exact-match rules that run first, before AI triage."
        icon={<WorkflowSquare01Icon className="h-4 w-4" />}
        status={rules.length > 0 ? <Badge variant="secondary" className="mr-2">{rules.length}</Badge> : null}
        action={(
          <Button size="sm" onClick={openCreateRule} disabled={activeMailboxes.length === 0}>
            <PlusSignIcon className="mr-1.5 h-3.5 w-3.5" />
            New Rule
          </Button>
        )}
        expanded={isExpanded('routing-rules')}
        onToggle={toggleSection}
      >
        {rules.length === 0 ? (
          <div className="flex min-h-[200px] flex-col items-center justify-center px-6 py-12 text-center">
            <span className="flex h-10 w-10 items-center justify-center rounded-full bg-muted text-muted-foreground">
              <WorkflowSquare01Icon className="h-4 w-4" />
            </span>
            <p className="mt-4 text-sm font-medium text-foreground">No routing rules yet</p>
            <p className="mt-1 max-w-md text-sm text-muted-foreground">
              Add rules for deterministic matches before AI triage runs.
            </p>
            <Button className="mt-5" variant="outline" size="sm" onClick={openCreateRule} disabled={activeMailboxes.length === 0}>
              <PlusSignIcon className="mr-1.5 h-3.5 w-3.5" />
              New Rule
            </Button>
          </div>
        ) : (
          <div className="divide-y">
            {rules.map((rule) => (
              <div key={rule.id} className={cn('grid gap-4 px-5 py-4 md:grid-cols-[42px_minmax(0,1fr)_auto]', !rule.active && 'opacity-65')}>
                <span className="flex h-8 w-8 items-center justify-center rounded-md bg-muted text-xs font-medium text-muted-foreground">
                  {rule.priority}
                </span>
                <div className="min-w-0">
                  <div className="flex flex-wrap items-center gap-2">
                    <p className="text-sm font-medium text-foreground">{rule.name}</p>
                    <Badge variant={rule.active ? 'secondary' : 'outline'} className="h-5 px-2 text-[11px]">
                      {rule.active ? 'Active' : 'Inactive'}
                    </Badge>
                    <Badge variant="outline" className="h-5 px-2 text-[11px]">
                      {rule.conditions.condition_logic === 'any' ? 'Any condition' : 'All conditions'}
                    </Badge>
                    <span className="text-xs text-muted-foreground">
                      Routes to {rule.target_mailbox_name ?? 'Unknown inbox'}
                    </span>
                  </div>
                  <div className="mt-3 flex flex-wrap gap-2">
                    {(rule.conditions.phrase_contains ?? []).map((phrase) => (
                      <Badge key={phrase} variant="outline" className="h-6 px-2 text-[11px] font-normal">
                        Text: {phrase}
                      </Badge>
                    ))}
                    {(rule.conditions.email_domain_equals ?? []).map((domain) => (
                      <Badge key={domain} variant="outline" className="h-6 px-2 text-[11px] font-normal">
                        Domain: {domain}
                      </Badge>
                    ))}
                    {(rule.conditions.sender_email_contains ?? []).map((email) => (
                      <Badge key={email} variant="outline" className="h-6 px-2 text-[11px] font-normal">
                        Email ID: {email}
                      </Badge>
                    ))}
                  </div>
                </div>
                <div className="flex items-start justify-end gap-1">
                  <Button variant="ghost" size="icon" className="h-8 w-8" onClick={() => openEditRule(rule)} aria-label={`Edit ${rule.name}`}>
                    <PencilEdit01Icon className="h-3.5 w-3.5" />
                  </Button>
                  <Button variant="ghost" size="icon" className="h-8 w-8" onClick={() => handleDeleteRule(rule)} aria-label={`Delete ${rule.name}`}>
                    <Delete01Icon className="h-3.5 w-3.5" />
                  </Button>
                </div>
              </div>
            ))}
          </div>
        )}
      </RoutingSection>

      <RoutingSection
        id="ai-triage"
        title="AI Triage"
        description="Classify unmatched conversations by meaning."
        icon={<BotIcon className="h-4 w-4" />}
        status={draft.triage_enabled ? (
          <Badge variant="outline" className="mr-2 gap-1.5 border-emerald-500/30 bg-emerald-500/10 text-emerald-700 dark:text-emerald-400">
            <span className="h-1.5 w-1.5 rounded-full bg-emerald-500" />
            On
          </Badge>
        ) : (
          <Badge variant="outline" className="mr-2 gap-1.5 text-muted-foreground">
            <span className="h-1.5 w-1.5 rounded-full bg-muted-foreground/50" />
            Off
          </Badge>
        )}
        expanded={isExpanded('ai-triage')}
        onToggle={toggleSection}
      >
        <div className="divide-y">
          <RoutingSettingRow
            title="Enable AI Triage"
            description="Automatically classify new conversations and suggest or assign the best inbox."
            className="md:grid-cols-[minmax(220px,0.42fr)_auto]"
          >
            <div className="flex justify-start md:justify-end">
              <Switch
                checked={draft.triage_enabled}
                disabled={updateSettings.isPending}
                onCheckedChange={(checked) => setDraft((c) => ({ ...c, triage_enabled: checked }))}
              />
            </div>
          </RoutingSettingRow>

          <RoutingSettingRow
            title="Triage Channels"
            description="Which conversation sources should AI classify."
            className="md:grid-cols-[minmax(220px,0.42fr)_auto]"
          >
            <div className={cn('flex flex-wrap justify-start gap-x-6 gap-y-3 md:justify-end', controlsDisabled && 'opacity-55')}>
              <label className="flex items-center gap-2 text-sm">
                <Checkbox
                  checked={draft.triage_widget_enabled}
                  disabled={controlsDisabled}
                  onCheckedChange={(checked) => setDraft((c) => ({ ...c, triage_widget_enabled: Boolean(checked) }))}
                />
                Widget
              </label>
              <label className="flex items-center gap-2 text-sm">
                <Checkbox
                  checked={draft.triage_email_enabled}
                  disabled={controlsDisabled}
                  onCheckedChange={(checked) => setDraft((c) => ({ ...c, triage_email_enabled: Boolean(checked) }))}
                />
                Email
              </label>
              <label className="flex items-center gap-2 text-sm">
                <Checkbox
                  checked={draft.triage_internal_enabled}
                  disabled={controlsDisabled}
                  onCheckedChange={(checked) => setDraft((c) => ({ ...c, triage_internal_enabled: Boolean(checked) }))}
                />
                Internal & API
              </label>
            </div>
          </RoutingSettingRow>

          <RoutingSettingRow title="Confidence Threshold">
            <ConfidenceThresholdControl
              value={draft.triage_confidence_threshold}
              disabled={controlsDisabled}
              onChange={(value) => setDraft((c) => ({ ...c, triage_confidence_threshold: value }))}
            />
          </RoutingSettingRow>

          <RoutingSettingRow
            title="Auto-Move Conversations"
            description="Move high-confidence matches automatically. When off, agents see suggestions instead."
            className="md:grid-cols-[minmax(220px,0.42fr)_auto]"
          >
            <div className="flex justify-start md:justify-end">
              <Switch
                checked={draft.triage_auto_move_enabled}
                disabled={controlsDisabled}
                onCheckedChange={(checked) => setDraft((c) => ({ ...c, triage_auto_move_enabled: checked }))}
              />
            </div>
          </RoutingSettingRow>
        </div>
      </RoutingSection>

      <RoutingSection
        id="advanced"
        title="Advanced Settings"
        description="Fallback behavior, daily limits, and spam prevention."
        icon={<Settings02Icon className="h-4 w-4" />}
        expanded={isExpanded('advanced')}
        onToggle={toggleSection}
      >
        <div className="divide-y">
          <RoutingSettingRow
            title="Fallback Behavior"
            description="Where unmatched conversations land."
            className="md:grid-cols-[minmax(220px,0.42fr)_auto]"
          >
            <div className="flex justify-start md:justify-end">
              <Select
                value={draft.triage_fallback_behavior}
                onValueChange={(value: 'shared' | 'default') => setDraft((c) => ({ ...c, triage_fallback_behavior: value }))}
                disabled={controlsDisabled}
              >
                <SelectTrigger id="triage-fallback-behavior" className="w-full sm:w-64">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="shared">Keep in Shared Inbox</SelectItem>
                  <SelectItem value="default">Keep in Default Inbox</SelectItem>
                </SelectContent>
              </Select>
            </div>
          </RoutingSettingRow>

          <RoutingSettingRow
            title="Re-Run When Meaning Changes"
            description="Re-classify if a follow-up message changes the topic."
            className="md:grid-cols-[minmax(220px,0.42fr)_auto]"
          >
            <div className="flex justify-start md:justify-end">
              <Switch
                checked={draft.triage_rerun_on_meaning_change}
                disabled={controlsDisabled}
                onCheckedChange={(checked) => setDraft((c) => ({ ...c, triage_rerun_on_meaning_change: checked }))}
              />
            </div>
          </RoutingSettingRow>

          <RoutingSettingRow
            title="Daily AI Budget"
            description="Maximum AI classifications per day across this workspace."
            className="md:grid-cols-[minmax(220px,0.42fr)_auto]"
          >
            <div className="flex justify-start md:justify-end">
              <Input
                id="triage-daily-budget"
                type="number"
                min={0}
                disabled={controlsDisabled}
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
            title="Skip Spam Conversations"
            description="Do not spend AI budget on conversations already flagged as spam."
            className="md:grid-cols-[minmax(220px,0.42fr)_auto]"
          >
            <div className="flex justify-start md:justify-end">
              <Switch
                checked={draft.triage_skip_spam_conversations}
                disabled={controlsDisabled}
                onCheckedChange={(checked) => setDraft((c) => ({ ...c, triage_skip_spam_conversations: checked }))}
              />
            </div>
          </RoutingSettingRow>

          <RoutingSettingRow
            title="Deduplicate First Messages"
            description="Reuse recent classifications when identical messages arrive in a burst."
            className="md:grid-cols-[minmax(220px,0.42fr)_auto]"
          >
            <div className="flex justify-start md:justify-end">
              <Switch
                checked={draft.triage_deduplicate_first_message}
                disabled={controlsDisabled}
                onCheckedChange={(checked) => setDraft((c) => ({ ...c, triage_deduplicate_first_message: checked }))}
              />
            </div>
          </RoutingSettingRow>
        </div>
      </RoutingSection>

      <div className="flex flex-col gap-3 rounded-lg border bg-card px-4 py-4 sm:flex-row sm:items-center sm:justify-between">
        <div className="flex min-w-0 items-center gap-3">
          <span className="flex h-9 w-9 shrink-0 items-center justify-center rounded-md border text-muted-foreground">
            <SparklesIcon className="h-4 w-4" />
          </span>
          <div className="min-w-0">
            <p className="text-sm font-medium text-foreground">How it works</p>
            <p className="text-sm text-muted-foreground">
              Rules run first <ArrowRight02Icon className="mx-1 inline h-3 w-3" /> AI triage classifies <ArrowRight02Icon className="mx-1 inline h-3 w-3" /> unmatched conversations go to fallback.
            </p>
          </div>
        </div>
        <Button variant="outline" size="sm" onClick={() => toggleSection('advanced')}>
          View routing controls
        </Button>
      </div>

      {/* Dialogs */}
      <RuleDialog
        open={ruleDialogOpen}
        onOpenChange={setRuleDialogOpen}
        rule={editingRule}
        mailboxes={activeMailboxes}
        isSaving={createRule.isPending || updateRule.isPending}
        onSave={handleSaveRule}
      />

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
