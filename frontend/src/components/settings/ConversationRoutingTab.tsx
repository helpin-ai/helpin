import { useEffect, useMemo, useState } from 'react';
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
    phraseChips: [...(rule?.conditions?.phrase_contains ?? [])],
    domainChips: [...(rule?.conditions?.email_domain_equals ?? [])],
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
      phrase_contains: form.phraseChips,
      email_domain_equals: form.domainChips.map((v) => v.toLowerCase()),
    };
    if (conditions.phrase_contains.length === 0 && conditions.email_domain_equals.length === 0) {
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
              <Label>Email Domain Equals</Label>
              <ChipInput
                value={form.domainChips}
                onValueChange={(v) => setForm((c) => ({ ...c, domainChips: v }))}
                inputValue={form.domainInput}
                onInputValueChange={(v) => setForm((c) => ({ ...c, domainInput: v }))}
                placeholder="Type a domain and press Enter..."
                normalize={(v) => v.toLowerCase()}
              />
              <p className="text-xs text-muted-foreground">Match sender email domains exactly.</p>
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
  const [ruleDialogOpen, setRuleDialogOpen] = useState(false);
  const [editingRule, setEditingRule] = useState<SupportTriageRule | null>(null);

  // Accordion state
  const [expandedSections, setExpandedSections] = useState<Set<string>>(
    new Set(['routing-rules', 'ai-triage']),
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
        <div className="fixed bottom-4 right-4 z-50 flex items-center gap-3 rounded-lg border bg-background/95 px-4 py-2.5 shadow-lg backdrop-blur animate-in fade-in slide-in-from-bottom-2 duration-200">
          <Badge variant="secondary">Unsaved changes</Badge>
          <Button size="sm" onClick={handleSaveSettings} disabled={updateSettings.isPending}>
            {updateSettings.isPending ? 'Saving...' : 'Save'}
          </Button>
        </div>
      )}

      {/* ── Pipeline overview ────────────────────────────────────────── */}
      <div className="flex items-center gap-2 rounded-lg border border-dashed border-border/80 bg-muted/30 px-4 py-3 text-xs text-muted-foreground">
        <span className="font-medium text-foreground">How routing works:</span>
        <span>Rules</span>
        <ArrowRight02Icon className="h-3 w-3 shrink-0" />
        <span>AI Triage</span>
        <ArrowRight02Icon className="h-3 w-3 shrink-0" />
        <span>Fallback Inbox</span>
      </div>

      {/* ── 1. Routing Rules ─────────────────────────────────────────── */}
      <div className={cn(
        'overflow-hidden rounded-lg border bg-background transition-colors',
        isExpanded('routing-rules') ? 'border-primary/20' : 'border-border/60',
      )}>
        <div className="flex items-center">
          <button
            type="button"
            onClick={() => toggleSection('routing-rules')}
            className="flex flex-1 items-center gap-4 px-4 py-4 text-left transition-colors hover:bg-muted/40"
          >
            <div className="flex h-9 w-9 shrink-0 items-center justify-center rounded-lg bg-muted text-muted-foreground">
              <WorkflowSquare01Icon className="h-4 w-4" />
            </div>
            <div className="min-w-0 flex-1">
              <p className="text-sm font-medium">Routing Rules</p>
              <p className="text-sm text-muted-foreground">Exact-match rules that run first</p>
            </div>
            {rules.length > 0 && <Badge variant="secondary" className="mr-2">{rules.length}</Badge>}
            <ArrowDown01Icon className={cn(
              'h-4 w-4 shrink-0 text-muted-foreground transition-transform duration-200',
              isExpanded('routing-rules') && 'rotate-180',
            )} />
          </button>
          <div className="pr-4">
            <Button size="sm" onClick={openCreateRule} disabled={activeMailboxes.length === 0}>
              <PlusSignIcon className="mr-1.5 h-3.5 w-3.5" />
              New Rule
            </Button>
          </div>
        </div>
        <div className="accordion-animate" data-open={isExpanded('routing-rules')}>
          <div>
            <div className="border-t border-border">
              {rules.length === 0 ? (
                <div className="px-6 py-10 text-center">
                  <SparklesIcon className="mx-auto h-6 w-6 text-muted-foreground/60" />
                  <p className="mt-3 text-sm font-medium">No routing rules yet</p>
                  <p className="mt-1 text-sm text-muted-foreground">
                    Add rules for deterministic matches before AI triage runs.
                  </p>
                </div>
              ) : (
                <div className="divide-y divide-border">
                  {rules.map((rule) => (
                    <div key={rule.id} className={cn('px-5 py-4', !rule.active && 'opacity-60')}>
                      {/* Header row: priority badge, name, status, actions */}
                      <div className="flex items-start gap-3">
                        <span className="flex h-7 w-7 shrink-0 items-center justify-center rounded-md bg-muted text-xs font-semibold text-muted-foreground">
                          {rule.priority}
                        </span>
                        <div className="min-w-0 flex-1">
                          <div className="flex items-center gap-2">
                            <span className="text-sm font-medium">{rule.name}</span>
                            <span className={cn(
                              'inline-flex h-1.5 w-1.5 rounded-full',
                              rule.active ? 'bg-emerald-500' : 'bg-muted-foreground/40',
                            )} />
                            <span className="text-xs text-muted-foreground">
                              {rule.active ? 'Active' : 'Inactive'}
                            </span>
                          </div>
                          <p className="mt-0.5 text-xs text-muted-foreground">
                            Route to <span className="font-medium text-foreground">{rule.target_mailbox_name ?? 'Unknown inbox'}</span>
                          </p>
                        </div>
                        <div className="flex shrink-0 gap-1">
                          <Button variant="ghost" size="icon" className="h-7 w-7" onClick={() => openEditRule(rule)}>
                            <PencilEdit01Icon className="h-3.5 w-3.5" />
                          </Button>
                          <Button variant="ghost" size="icon" className="h-7 w-7" onClick={() => handleDeleteRule(rule)}>
                            <Delete01Icon className="h-3.5 w-3.5" />
                          </Button>
                        </div>
                      </div>

                      {/* Condition chips */}
                      <div className="mt-3 space-y-2 pl-10">
                        {(rule.conditions.phrase_contains?.length ?? 0) > 0 && (
                          <div className="flex flex-wrap items-center gap-1.5">
                            <span className="text-xs text-muted-foreground">Text contains:</span>
                            {rule.conditions.phrase_contains.map((phrase) => (
                              <Badge key={phrase} variant="outline" className="h-5 rounded-full px-2 text-[11px] font-normal">
                                {phrase}
                              </Badge>
                            ))}
                          </div>
                        )}
                        {(rule.conditions.email_domain_equals?.length ?? 0) > 0 && (
                          <div className="flex flex-wrap items-center gap-1.5">
                            <span className="text-xs text-muted-foreground">Email domain:</span>
                            {rule.conditions.email_domain_equals.map((domain) => (
                              <Badge key={domain} variant="outline" className="h-5 rounded-full px-2 text-[11px] font-normal">
                                {domain}
                              </Badge>
                            ))}
                          </div>
                        )}
                      </div>
                    </div>
                  ))}

                  {/* Footer hint */}
                  <div className="flex items-center justify-center gap-2 px-5 py-3 text-xs text-muted-foreground">
                    <SparklesIcon className="h-3 w-3" />
                    <span>No more rules — AI triage handles the rest</span>
                  </div>
                </div>
              )}
            </div>
          </div>
        </div>
      </div>

      {/* ── 2. AI Triage ─────────────────────────────────────────────── */}
      <div className={cn(
        'overflow-hidden rounded-lg border bg-background transition-colors',
        isExpanded('ai-triage') ? 'border-primary/20' : 'border-border/60',
      )}>
        <button
          type="button"
          onClick={() => toggleSection('ai-triage')}
          className="flex w-full items-center gap-4 px-4 py-4 text-left transition-colors hover:bg-muted/40"
        >
          <div className="flex h-9 w-9 shrink-0 items-center justify-center rounded-lg bg-muted text-muted-foreground">
            <BotIcon className="h-4 w-4" />
          </div>
          <div className="min-w-0 flex-1">
            <p className="text-sm font-medium">AI Triage</p>
            <p className="text-sm text-muted-foreground">Classify unmatched conversations by meaning</p>
          </div>
          {draft.triage_enabled && <Badge variant="secondary" className="mr-2">On</Badge>}
          <ArrowDown01Icon className={cn(
            'h-4 w-4 shrink-0 text-muted-foreground transition-transform duration-200',
            isExpanded('ai-triage') && 'rotate-180',
          )} />
        </button>
        <div className="accordion-animate" data-open={isExpanded('ai-triage')}>
          <div>
            <div className="border-t border-border px-6 py-6 space-y-5">
              {/* Enable toggle */}
              <div className="flex items-start justify-between gap-4">
                <div className="space-y-1">
                  <Label>Enable AI Triage</Label>
                  <p className="text-sm text-muted-foreground">
                    Automatically classify new conversations and suggest or assign the best inbox.
                  </p>
                </div>
                <Switch
                  checked={draft.triage_enabled}
                  onCheckedChange={(checked) => setDraft((c) => ({ ...c, triage_enabled: checked }))}
                />
              </div>

              {draft.triage_enabled && (
                <div className="border-t border-border pt-5 space-y-5">
                  {/* Channels */}
                  <div className="space-y-3">
                    <div>
                      <Label>Triage Channels</Label>
                      <p className="text-sm text-muted-foreground">Which conversation sources should AI classify</p>
                    </div>
                    <div className="flex flex-wrap gap-x-6 gap-y-2">
                      <label className="flex items-center gap-2 text-sm">
                        <Checkbox
                          checked={draft.triage_widget_enabled}
                          disabled={updateSettings.isPending}
                          onCheckedChange={(checked) => setDraft((c) => ({ ...c, triage_widget_enabled: Boolean(checked) }))}
                        />
                        Widget
                      </label>
                      <label className="flex items-center gap-2 text-sm">
                        <Checkbox
                          checked={draft.triage_email_enabled}
                          disabled={updateSettings.isPending}
                          onCheckedChange={(checked) => setDraft((c) => ({ ...c, triage_email_enabled: Boolean(checked) }))}
                        />
                        Email
                      </label>
                      <label className="flex items-center gap-2 text-sm">
                        <Checkbox
                          checked={draft.triage_internal_enabled}
                          disabled={updateSettings.isPending}
                          onCheckedChange={(checked) => setDraft((c) => ({ ...c, triage_internal_enabled: Boolean(checked) }))}
                        />
                        Internal & API
                      </label>
                    </div>
                  </div>

                  {/* Confidence threshold */}
                  <div className="space-y-3">
                    <div className="flex items-center justify-between">
                      <div>
                        <Label>Confidence Threshold</Label>
                        <p className="text-sm text-muted-foreground">Minimum score for auto-routing</p>
                      </div>
                      <span className="text-sm font-medium tabular-nums">
                        {Math.round(draft.triage_confidence_threshold * 100)}%
                      </span>
                    </div>
                    <input
                      type="range"
                      min="0.5"
                      max="1"
                      step="0.01"
                      value={draft.triage_confidence_threshold}
                      disabled={updateSettings.isPending}
                      onChange={(event) => setDraft((c) => ({
                        ...c,
                        triage_confidence_threshold: Number(event.target.value),
                      }))}
                      className="w-full accent-primary"
                    />
                  </div>

                  {/* Auto-move */}
                  <div className="flex items-start justify-between gap-4">
                    <div className="space-y-1">
                      <Label>Auto-Move Conversations</Label>
                      <p className="text-sm text-muted-foreground">
                        Move high-confidence matches automatically. When off, agents see suggestions instead.
                      </p>
                    </div>
                    <Switch
                      checked={draft.triage_auto_move_enabled}
                      disabled={updateSettings.isPending}
                      onCheckedChange={(checked) => setDraft((c) => ({ ...c, triage_auto_move_enabled: checked }))}
                    />
                  </div>
                </div>
              )}
            </div>
          </div>
        </div>
      </div>

      {/* ── 3. Inbox Catalog ─────────────────────────────────────────── */}
      <div className={cn(
        'overflow-hidden rounded-lg border bg-background transition-colors',
        isExpanded('inbox-catalog') ? 'border-primary/20' : 'border-border/60',
      )}>
        <button
          type="button"
          onClick={() => toggleSection('inbox-catalog')}
          className="flex w-full items-center gap-4 px-4 py-4 text-left transition-colors hover:bg-muted/40"
        >
          <div className="flex h-9 w-9 shrink-0 items-center justify-center rounded-lg bg-muted text-muted-foreground">
            <InboxIcon className="h-4 w-4" />
          </div>
          <div className="min-w-0 flex-1">
            <p className="text-sm font-medium">Inboxes & Routing Prompts</p>
            <p className="text-sm text-muted-foreground">Where conversations land and how AI identifies each inbox</p>
          </div>
          <ArrowDown01Icon className={cn(
            'h-4 w-4 shrink-0 text-muted-foreground transition-transform duration-200',
            isExpanded('inbox-catalog') && 'rotate-180',
          )} />
        </button>
        <div className="accordion-animate" data-open={isExpanded('inbox-catalog')}>
          <div>
            <div className="border-t border-border">
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
                    <TableCell className="max-w-[300px] whitespace-normal text-sm text-muted-foreground">
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
                      <TableCell className="max-w-[300px] whitespace-normal text-sm text-muted-foreground">
                        {mailbox.routing_prompt?.trim() || mailbox.description?.trim() || 'No routing prompt configured.'}
                      </TableCell>
                      <TableCell>
                        <Button variant="ghost" size="sm" onClick={() => setEditingMailbox(mailbox)}>
                          <PencilEdit01Icon className="mr-1.5 h-3.5 w-3.5" />
                          Edit
                        </Button>
                      </TableCell>
                    </TableRow>
                  ))}
                </TableBody>
              </Table>
            </div>
          </div>
        </div>
      </div>

      {/* ── 4. Advanced Settings ──────────────────────────────────────── */}
      <div className={cn(
        'overflow-hidden rounded-lg border bg-background transition-colors',
        isExpanded('advanced') ? 'border-primary/20' : 'border-border/60',
      )}>
        <button
          type="button"
          onClick={() => toggleSection('advanced')}
          className="flex w-full items-center gap-4 px-4 py-4 text-left transition-colors hover:bg-muted/40"
        >
          <div className="flex h-9 w-9 shrink-0 items-center justify-center rounded-lg bg-muted text-muted-foreground">
            <Settings02Icon className="h-4 w-4" />
          </div>
          <div className="min-w-0 flex-1">
            <p className="text-sm font-medium">Advanced Settings</p>
            <p className="text-sm text-muted-foreground">Fallback behavior, daily limits, and spam prevention</p>
          </div>
          <ArrowDown01Icon className={cn(
            'h-4 w-4 shrink-0 text-muted-foreground transition-transform duration-200',
            isExpanded('advanced') && 'rotate-180',
          )} />
        </button>
        <div className="accordion-animate" data-open={isExpanded('advanced')}>
          <div>
            <div className="border-t border-border px-6 py-6 space-y-5">
              {/* Fallback behavior */}
              <div className="space-y-2">
                <Label htmlFor="triage-fallback-behavior">Fallback Behavior</Label>
                <p className="text-sm text-muted-foreground">
                  Where unmatched conversations land when AI can't confidently classify them
                </p>
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

              {/* Re-run on meaning change */}
              <div className="flex items-start justify-between gap-4">
                <div className="space-y-1">
                  <Label>Re-Run When Meaning Changes</Label>
                  <p className="text-sm text-muted-foreground">
                    Re-classify if a follow-up message changes the topic. Off by default for predictable routing.
                  </p>
                </div>
                <Switch
                  checked={draft.triage_rerun_on_meaning_change}
                  disabled={controlsDisabled}
                  onCheckedChange={(checked) => setDraft((c) => ({ ...c, triage_rerun_on_meaning_change: checked }))}
                />
              </div>

              <div className="border-t border-border pt-5 space-y-5">
                {/* Daily budget */}
                <div className="space-y-2">
                  <Label htmlFor="triage-daily-budget">Daily AI Budget</Label>
                  <p className="text-sm text-muted-foreground">
                    Maximum AI classifications per day across this workspace
                  </p>
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

                {/* Skip spam */}
                <div className="flex items-start justify-between gap-4">
                  <div className="space-y-1">
                    <Label>Skip Spam Conversations</Label>
                    <p className="text-sm text-muted-foreground">
                      Don't spend AI budget on conversations already flagged as spam
                    </p>
                  </div>
                  <Switch
                    checked={draft.triage_skip_spam_conversations}
                    disabled={controlsDisabled}
                    onCheckedChange={(checked) => setDraft((c) => ({ ...c, triage_skip_spam_conversations: checked }))}
                  />
                </div>

                {/* Deduplicate */}
                <div className="flex items-start justify-between gap-4">
                  <div className="space-y-1">
                    <Label>Deduplicate First Messages</Label>
                    <p className="text-sm text-muted-foreground">
                      Reuse recent classifications when identical messages arrive in a burst
                    </p>
                  </div>
                  <Switch
                    checked={draft.triage_deduplicate_first_message}
                    disabled={controlsDisabled}
                    onCheckedChange={(checked) => setDraft((c) => ({ ...c, triage_deduplicate_first_message: checked }))}
                  />
                </div>
              </div>
            </div>
          </div>
        </div>
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
        open={!!editingMailbox}
        onOpenChange={(open) => {
          if (!open) {
            setEditingMailbox(null);
          }
        }}
        mailbox={editingMailbox}
      />
    </div>
  );
}
