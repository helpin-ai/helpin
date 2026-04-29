import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { createPortal } from 'react-dom';
import { toast } from 'sonner';
import {
  AiMagicIcon,
  ArrowUp01Icon,
  BookOpen01Icon,
  BotIcon,
  Briefcase01Icon,
  Cancel01Icon,
  File01Icon,
  FolderKanbanIcon,
  Loading01Icon,
  RecordIcon,
  Target01Icon,
  UserIcon,
} from '@/lib/icons';
import { cn } from '@/lib/utils';
import { usePageContext } from '@/components/command-bar/pageContext';
import { useWorkspaceStore } from '@/stores/workspaceStore';
import { commandBarService } from '@/lib/services/commandBarService';
import { useCommandBarRunStore } from '@/stores/commandBarStore';
import type { AgentRun, AgentRunStatus } from '@/lib/pm-types/agents';
import type {
  CommandBarPageContext,
  CommandBarParseResponse,
} from '@/lib/pmTypes';

const COLLAPSED_KEY = 'helpin:ask-agents-dock-collapsed';

const TYPE_LABEL: Record<CommandBarPageContext['entity_type'], string> = {
  task: 'Task',
  epic: 'Epic',
  document: 'Doc',
  crm_contact: 'Contact',
  crm_deal: 'Deal',
  workspace: 'Workspace',
};

type ThreadStatus = 'running' | 'completed' | 'failed' | 'cancelled';

type ThreadMessage =
  | { kind: 'user'; id: string; text: string }
  | {
      kind: 'agent';
      id: string;
      planId?: string;
      runIds: string[];
      agentName: string;
      description: string;
    };

function chipIcon(type: CommandBarPageContext['entity_type']) {
  switch (type) {
    case 'task':
      return RecordIcon;
    case 'epic':
      return BookOpen01Icon;
    case 'document':
      return File01Icon;
    case 'crm_contact':
      return UserIcon;
    case 'crm_deal':
      return Briefcase01Icon;
    default:
      return FolderKanbanIcon;
  }
}

/**
 * Extract a short, human-readable description of what a step will do.
 * One-shot brief instructions look like "One-shot execution brief\nGoal:\n{goal}\nPlan:\n...";
 * we surface the Goal section. For other shapes, fall back to the raw instructions.
 */
function describeStep(instructions: string | undefined | null): string {
  const text = (instructions ?? '').trim();
  if (!text) return '';
  if (!text.toLowerCase().includes('goal:')) return text;
  const lines = text.split('\n');
  const goalIdx = lines.findIndex((l) => l.trim().toLowerCase() === 'goal:');
  if (goalIdx < 0) return text;
  const out: string[] = [];
  for (let i = goalIdx + 1; i < lines.length; i++) {
    const line = lines[i].trim();
    if (!line) continue;
    const lower = line.toLowerCase();
    if (
      lower === 'plan:' ||
      lower === 'constraints:' ||
      lower.startsWith('target:') ||
      lower.startsWith('user request:')
    )
      break;
    out.push(line.replace(/^[-*]\s+/, '').replace(/^\d+\.\s+/, ''));
  }
  return out.join(' ').trim() || text;
}

function aggregateStatus(
  runIds: string[],
  runsById: Record<string, AgentRun>,
): ThreadStatus {
  if (runIds.length === 0) return 'completed';
  const runs = runIds.map((id) => runsById[id]).filter(Boolean) as AgentRun[];
  if (runs.length === 0) return 'running';
  if (runs.some((r) => r.status === 'failed')) return 'failed';
  if (runs.some((r) => r.status === 'cancelled')) return 'cancelled';
  if (
    runs.every(
      (r) => r.status === ('completed' as AgentRunStatus),
    )
  )
    return 'completed';
  return 'running';
}

function ContextChip({ context }: { context: CommandBarPageContext }) {
  const Icon = chipIcon(context.entity_type);
  return (
    <span
      title={context.display_title || context.entity_id}
      className="inline-flex max-w-[260px] items-center gap-1.5 rounded-full border border-primary/20 bg-primary/5 px-2 py-0.5 text-[11px] text-foreground"
    >
      <Icon className="h-3 w-3 shrink-0" />
      <span className="text-[10px] font-medium uppercase tracking-wider text-muted-foreground">
        {TYPE_LABEL[context.entity_type] ?? context.entity_type}
      </span>
      <span className="truncate font-medium">{context.display_title || context.entity_id}</span>
    </span>
  );
}

function ThreadAgentMessage({
  message,
  onView,
}: {
  message: Extract<ThreadMessage, { kind: 'agent' }>;
  onView: () => void;
}) {
  const runsById = useCommandBarRunStore((s) => s.runsById);
  const status = useMemo(
    () => aggregateStatus(message.runIds, runsById),
    [message.runIds, runsById],
  );

  const statusLabel =
    status === 'running'
      ? 'Running'
      : status === 'completed'
        ? 'Completed'
        : status === 'failed'
          ? 'Failed'
          : 'Cancelled';

  const statusClass =
    status === 'running'
      ? 'text-muted-foreground'
      : status === 'completed'
        ? 'text-emerald-600 dark:text-emerald-400'
        : 'text-destructive';

  return (
    <div className="rounded-md border border-border/60 bg-muted/30 px-2.5 py-2">
      <div className="flex items-center justify-between gap-2">
        <div className="flex min-w-0 items-center gap-2">
          <BotIcon className="h-3.5 w-3.5 shrink-0 text-muted-foreground" />
          <span className="truncate text-sm font-medium text-foreground">
            {message.agentName}
          </span>
        </div>
        <span className={cn('inline-flex items-center gap-1 text-[11px]', statusClass)}>
          {status === 'running' ? (
            <Loading01Icon className="h-3 w-3 animate-spin" />
          ) : (
            <span
              aria-hidden
              className={cn(
                'h-1.5 w-1.5 rounded-full',
                status === 'completed'
                  ? 'bg-emerald-500'
                  : 'bg-destructive',
              )}
            />
          )}
          {statusLabel}
        </span>
      </div>
      {message.description ? (
        <p className="mt-1 line-clamp-3 pl-[22px] text-xs leading-snug text-muted-foreground">
          {message.description}
        </p>
      ) : null}
      <div className="mt-1.5 flex justify-end pl-[22px]">
        <button
          type="button"
          onClick={onView}
          className="rounded px-1.5 py-0.5 text-[11px] font-medium text-muted-foreground transition hover:bg-muted hover:text-foreground"
        >
          View run →
        </button>
      </div>
    </div>
  );
}

export function AskAgentsDock() {
  const workspace = useWorkspaceStore((s) => s.currentWorkspace);
  const pageContext = usePageContext();
  const addPlan = useCommandBarRunStore((s) => s.addPlan);
  const addRuns = useCommandBarRunStore((s) => s.addRuns);
  const setRailMode = useCommandBarRunStore((s) => s.setRailMode);
  const setRailFilter = useCommandBarRunStore((s) => s.setRailFilter);
  const setSelectedRunId = useCommandBarRunStore((s) => s.setSelectedRunId);

  const [collapsed, setCollapsed] = useState(() => {
    if (typeof window === 'undefined') return false;
    return localStorage.getItem(COLLAPSED_KEY) === '1';
  });
  const [hiddenByModal, setHiddenByModal] = useState(false);
  const [value, setValue] = useState('');
  const [parsing, setParsing] = useState(false);
  const [dispatching, setDispatching] = useState(false);
  const [intentResult, setIntentResult] = useState<CommandBarParseResponse | null>(null);
  const [messages, setMessages] = useState<ThreadMessage[]>([]);
  const textareaRef = useRef<HTMLTextAreaElement | null>(null);
  const responseRef = useRef<HTMLDivElement | null>(null);

  // Auto-grow textarea (max 160px ≈ 8 lines)
  useEffect(() => {
    const el = textareaRef.current;
    if (!el) return;
    el.style.height = 'auto';
    el.style.height = `${Math.min(el.scrollHeight, 160)}px`;
  }, [value, collapsed]);

  useEffect(() => {
    localStorage.setItem(COLLAPSED_KEY, collapsed ? '1' : '0');
  }, [collapsed]);

  // Drop the current pending plan if the user keeps typing
  useEffect(() => {
    setIntentResult(null);
  }, [value]);

  // Auto-scroll the response area to the bottom when new messages or a plan appears
  useEffect(() => {
    const el = responseRef.current;
    if (!el) return;
    el.scrollTop = el.scrollHeight;
  }, [messages.length, intentResult, parsing]);

  // "/" focuses the dock when no other input is focused
  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if (e.key !== '/') return;
      const target = e.target as HTMLElement | null;
      if (!target) return;
      const tag = target.tagName?.toLowerCase();
      const isEditable =
        tag === 'input' || tag === 'textarea' || target.isContentEditable;
      if (isEditable) return;
      e.preventDefault();
      setCollapsed(false);
      requestAnimationFrame(() => textareaRef.current?.focus());
    };
    document.addEventListener('keydown', onKey);
    return () => document.removeEventListener('keydown', onKey);
  }, []);

  // Hide the dock while a centered modal dialog is open. Sheets (anything with
  // data-side, like the task panel) do NOT trigger this — the dock stays visible
  // over them. Tracks Radix's [role="dialog"][data-state="open"] tree.
  useEffect(() => {
    const update = () => {
      const open = document.querySelectorAll(
        '[role="dialog"][data-state="open"], [role="alertdialog"][data-state="open"]',
      );
      let blocking = false;
      open.forEach((el) => {
        if (el.hasAttribute('data-side')) return; // sheets pass through
        if (el.closest('[data-helpin-dock]')) return; // the dock itself
        blocking = true;
      });
      setHiddenByModal(blocking);
    };
    const obs = new MutationObserver(update);
    obs.observe(document.body, {
      childList: true,
      subtree: true,
      attributes: true,
      attributeFilter: ['data-state', 'role'],
    });
    update();
    return () => obs.disconnect();
  }, []);

  // External callers can pre-fill the dock via window.dispatchEvent(new CustomEvent('helpin:ask-agents', { detail: { query } }))
  useEffect(() => {
    const onAsk = (event: Event) => {
      const detail = (event as CustomEvent<{ query?: string }>).detail;
      const next = detail?.query?.trim() ?? '';
      if (!next) return;
      setCollapsed(false);
      setValue(next);
      setIntentResult(null);
      requestAnimationFrame(() => textareaRef.current?.focus());
    };
    window.addEventListener('helpin:ask-agents', onAsk);
    return () => window.removeEventListener('helpin:ask-agents', onAsk);
  }, []);

  const trimmed = value.trim();
  const showChip = !!pageContext && pageContext.entity_type !== 'workspace';

  const submit = useCallback(
    async (override?: string) => {
      const text = (override ?? value).trim();
      if (!workspace?.id || !pageContext || !text) return;
      setParsing(true);
      setIntentResult(null);
      try {
        const res = await commandBarService.parseIntent(workspace.id, {
          text,
          page_context: pageContext,
        });
        if (res.error || !res.data) {
          toast.error(res.error ?? 'Failed to parse command');
          return;
        }
        setIntentResult(res.data);
      } finally {
        setParsing(false);
      }
    },
    [pageContext, value, workspace?.id],
  );

  const confirmPlan = useCallback(async () => {
    if (
      !workspace?.id ||
      !pageContext ||
      !intentResult ||
      intentResult.status !== 'plan'
    )
      return;
    setDispatching(true);
    try {
      const res = await commandBarService.dispatchPlan(workspace.id, {
        text: trimmed,
        page_context: pageContext,
        steps: intentResult.plan.steps,
      });
      if (res.error || !res.data) {
        toast.error(res.error ?? 'Failed to start command run');
        return;
      }
      const steps = res.data.steps ?? intentResult.plan.steps;
      if (res.data.plan_id) {
        addPlan(
          {
            id: res.data.plan_id,
            steps,
            runIdsByStep: Object.fromEntries(
              res.data.runs.map((run, index) => [index, run.id]),
            ),
            planKind: intentResult.plan.plan_kind,
            status: 'running',
            prompt: trimmed,
            currentStepIndex: 0,
          },
          res.data.runs,
        );
      } else {
        addRuns(res.data.runs);
      }

      const firstStep = steps[0];
      const runIds = res.data.runs.map((r) => r.id);
      const userMsg: ThreadMessage = {
        kind: 'user',
        id: `user-${Date.now()}`,
        text: trimmed,
      };
      const agentMsg: ThreadMessage = {
        kind: 'agent',
        id: res.data.plan_id ?? `agent-${Date.now()}`,
        planId: res.data.plan_id,
        runIds,
        agentName:
          steps.length > 1
            ? `${firstStep.agent_name} +${steps.length - 1}`
            : firstStep.agent_name,
        description: describeStep(firstStep.instructions),
      };
      setMessages((prev) => [...prev, userMsg, agentMsg]);

      setValue('');
      setIntentResult(null);
    } finally {
      setDispatching(false);
    }
  }, [addPlan, addRuns, intentResult, pageContext, trimmed, workspace?.id]);

  // ⌘↵ confirms a pending plan from anywhere in the dock
  useEffect(() => {
    if (intentResult?.status !== 'plan') return;
    const handler = (e: KeyboardEvent) => {
      if ((e.metaKey || e.ctrlKey) && e.key === 'Enter') {
        e.preventDefault();
        void confirmPlan();
      }
    };
    window.addEventListener('keydown', handler);
    return () => window.removeEventListener('keydown', handler);
  }, [intentResult, confirmPlan]);

  if (!workspace) return null;
  if (hiddenByModal) return null;

  const onTextareaKeyDown = (e: React.KeyboardEvent<HTMLTextAreaElement>) => {
    if (e.key === 'Enter' && !e.shiftKey) {
      e.preventDefault();
      void submit();
    } else if (e.key === 'Escape') {
      if (intentResult) {
        setIntentResult(null);
      } else {
        e.currentTarget.blur();
      }
    }
  };

  if (collapsed) {
    if (typeof document === 'undefined') return null;
    return createPortal(
      <div
        data-helpin-dock="true"
        className="pointer-events-none fixed inset-x-0 bottom-3 z-[60] flex justify-center"
      >
        <button
          type="button"
          onClick={() => setCollapsed(false)}
          className="pointer-events-auto group inline-flex items-center gap-2 rounded-full border border-border/70 bg-background/95 px-3 py-1.5 text-xs font-medium text-muted-foreground shadow-[0_1px_2px_rgba(15,23,42,0.05),0_6px_20px_-8px_rgba(15,23,42,0.18)] backdrop-blur transition hover:border-primary/40 hover:bg-background hover:text-foreground hover:shadow-[0_2px_4px_rgba(15,23,42,0.06),0_10px_28px_-10px_rgba(15,23,42,0.25)]"
        >
          <AiMagicIcon className="h-3.5 w-3.5" />
          Ask agents
          {messages.length > 0 ? (
            <span className="rounded-full bg-primary/10 px-1.5 text-[10px] font-medium text-primary">
              {messages.filter((m) => m.kind === 'agent').length}
            </span>
          ) : null}
          {showChip && messages.length === 0 ? (
            <span className="max-w-[160px] truncate text-foreground">
              · {pageContext!.display_title || pageContext!.entity_id}
            </span>
          ) : null}
          <kbd className="ml-1 rounded border bg-muted px-1 py-0 text-[10px] font-mono text-muted-foreground">
            /
          </kbd>
        </button>
      </div>,
      document.body,
    );
  }

  const plan = intentResult?.status === 'plan' ? intentResult.plan : null;
  const noMatch = intentResult?.status === 'no_matching_agent' ? intentResult : null;
  const sendDisabled = !trimmed || parsing || dispatching || !!plan;
  const hasResponseArea = messages.length > 0 || !!plan || !!noMatch;

  const openRail = (runId?: string) => {
    setRailFilter('all');
    setRailMode('open');
    if (runId) setSelectedRunId(runId);
  };

  if (typeof document === 'undefined') return null;
  return createPortal(
    <div
      data-helpin-dock="true"
      className="pointer-events-none fixed inset-x-0 bottom-3 z-[60] flex justify-center px-4"
    >
      <div className="pointer-events-auto flex w-full max-w-2xl flex-col rounded-2xl border border-border/70 bg-background/95 shadow-[0_1px_2px_rgba(15,23,42,0.06),0_8px_24px_-12px_rgba(15,23,42,0.18),0_24px_64px_-28px_rgba(15,23,42,0.28)] ring-1 ring-black/[0.02] backdrop-blur transition-shadow focus-within:shadow-[0_1px_2px_rgba(15,23,42,0.06),0_12px_32px_-12px_rgba(15,23,42,0.22),0_32px_80px_-32px_rgba(15,23,42,0.34)] dark:ring-white/[0.04]">
        {hasResponseArea ? (
          <div className="order-1 flex max-h-[60vh] flex-col">
            {messages.length > 0 ? (
              <div className="flex items-center justify-between gap-2 border-b border-border/60 px-3.5 py-2 text-[11px] font-medium uppercase tracking-wider text-muted-foreground">
                <span>Recent runs</span>
                <button
                  type="button"
                  onClick={() => {
                    setMessages([]);
                    setIntentResult(null);
                  }}
                  className="rounded px-1.5 py-0.5 text-[11px] font-medium normal-case tracking-normal text-muted-foreground transition hover:bg-muted hover:text-foreground"
                >
                  New chat
                </button>
              </div>
            ) : null}

            <div ref={responseRef} className="flex flex-col gap-3 overflow-y-auto px-3.5 pt-3 pb-3">
              {messages.map((msg) =>
                msg.kind === 'user' ? (
                  <div key={msg.id} className="flex justify-end">
                    <div className="max-w-[80%] rounded-2xl rounded-tr-sm bg-primary/10 px-3 py-1.5 text-sm text-foreground">
                      {msg.text}
                    </div>
                  </div>
                ) : (
                  <ThreadAgentMessage
                    key={msg.id}
                    message={msg}
                    onView={() => openRail(msg.runIds[0])}
                  />
                ),
              )}

              {plan ? (
                <div>
                  {plan.steps.length > 1 || plan.plan_kind === 'fan_out' ? (
                    <div className="mb-2 flex items-center gap-2 text-[11px] text-muted-foreground">
                      <span className="font-medium uppercase tracking-wider">
                        {plan.plan_kind === 'fan_out' ? 'Fan-out' : 'Plan'}
                      </span>
                      <span className="opacity-60">·</span>
                      <span>
                        {plan.plan_kind === 'fan_out'
                          ? `${plan.steps.length} targets`
                          : `${plan.steps.length} steps`}
                      </span>
                    </div>
                  ) : null}
                  {intentResult?.status === 'plan' &&
                  intentResult.rationale &&
                  plan.plan_kind !== 'one_shot_command' ? (
                    <p className="mb-2 line-clamp-3 text-xs text-muted-foreground">
                      {intentResult.rationale}
                    </p>
                  ) : null}
                  {plan.guardrails?.length ? (
                    <div className="mb-2 space-y-1">
                      {plan.guardrails.map((g, i) => (
                        <div
                          key={i}
                          className="flex items-center gap-1.5 text-[11px] text-muted-foreground"
                        >
                          <Target01Icon className="h-3 w-3" />
                          <span>{g.message}</span>
                        </div>
                      ))}
                    </div>
                  ) : null}
                  {plan.steps.length === 1 ? (
                    <div className="rounded-md border border-border/60 bg-muted/30 px-2.5 py-2">
                      <div className="flex items-center gap-2">
                        <BotIcon className="h-3.5 w-3.5 shrink-0 text-muted-foreground" />
                        <span className="truncate text-sm font-medium text-foreground">
                          {plan.steps[0].agent_name}
                        </span>
                      </div>
                      {(() => {
                        const desc = describeStep(plan.steps[0].instructions);
                        return desc ? (
                          <p className="mt-1 line-clamp-3 pl-[22px] text-xs leading-snug text-muted-foreground">
                            {desc}
                          </p>
                        ) : null;
                      })()}
                    </div>
                  ) : (
                    <div className="relative space-y-1.5 pl-3">
                      <div
                        aria-hidden
                        className="absolute left-[8px] top-3 bottom-3 w-px bg-border/70"
                      />
                      {plan.steps.map((step, index) => (
                        <div key={`${step.agent_id}-${index}`} className="relative">
                          <span
                            aria-hidden
                            className="absolute -left-3 top-2 grid h-4 w-4 place-items-center rounded-full bg-background ring-2 ring-border/70 text-[9px] font-semibold text-muted-foreground"
                          >
                            {index + 1}
                          </span>
                          <div className="rounded-md border border-border/60 bg-muted/30 px-2.5 py-2">
                            <p className="text-sm font-medium leading-snug text-foreground line-clamp-2">
                              {step.instructions}
                            </p>
                            <div className="mt-1 flex items-center gap-1.5 text-[11px] text-muted-foreground">
                              <BotIcon className="h-3 w-3" />
                              <span className="truncate">{step.agent_name}</span>
                            </div>
                          </div>
                        </div>
                      ))}
                    </div>
                  )}
                  <div className="mt-2.5 flex items-center justify-between gap-2">
                    <button
                      type="button"
                      onClick={() => setIntentResult(null)}
                      disabled={dispatching}
                      className="rounded-md px-2 py-1 text-[11px] font-medium text-muted-foreground transition hover:bg-muted hover:text-foreground disabled:opacity-50"
                    >
                      Cancel
                    </button>
                    <button
                      type="button"
                      onClick={() => void confirmPlan()}
                      disabled={dispatching}
                      className={cn(
                        'inline-flex items-center gap-1.5 rounded-md px-2.5 py-1 text-[11px] font-medium transition',
                        dispatching
                          ? 'cursor-not-allowed bg-muted text-muted-foreground'
                          : 'bg-foreground text-background hover:bg-foreground/90',
                      )}
                    >
                      {dispatching ? (
                        <Loading01Icon className="h-3 w-3 animate-spin" />
                      ) : null}
                      {dispatching
                        ? 'Starting…'
                        : `Confirm ${
                            (plan.estimated_runs ?? plan.run_count) === 1
                              ? '1 run'
                              : `${plan.estimated_runs ?? plan.run_count} runs`
                          }`}
                      {!dispatching ? (
                        <kbd className="ml-1 rounded border border-background/30 bg-background/15 px-1 py-0 text-[9px] font-mono">
                          ⌘↵
                        </kbd>
                      ) : null}
                    </button>
                  </div>
                </div>
              ) : null}

              {noMatch ? (
                <div>
                  <div className="flex items-start gap-2">
                    <BotIcon className="mt-0.5 h-3.5 w-3.5 text-muted-foreground" />
                    <div className="min-w-0 text-sm">
                      <p className="font-medium">No available agent can do that yet.</p>
                      <p className="mt-0.5 text-xs text-muted-foreground">{noMatch.reason}</p>
                      {noMatch.suggestions?.length ? (
                        <div className="mt-2 flex flex-wrap gap-1.5">
                          {noMatch.suggestions.map((s) => (
                            <button
                              type="button"
                              key={s}
                              onClick={() => {
                                setValue(s);
                                void submit(s);
                              }}
                              className="rounded border border-border/70 bg-background/80 px-2 py-0.5 text-[11px] text-foreground transition hover:border-primary/40 hover:bg-primary/5"
                            >
                              {s}
                            </button>
                          ))}
                        </div>
                      ) : null}
                    </div>
                  </div>
                  <div className="mt-2 flex justify-end">
                    <button
                      type="button"
                      onClick={() => setIntentResult(null)}
                      className="rounded-md px-2 py-1 text-[11px] font-medium text-muted-foreground transition hover:bg-muted hover:text-foreground"
                    >
                      Dismiss
                    </button>
                  </div>
                </div>
              ) : null}
            </div>
          </div>
        ) : null}

        <div
          className={cn(
            'order-2 flex flex-col gap-2 px-3.5 pt-3 pb-2.5',
            hasResponseArea && 'border-t border-border/60',
          )}
        >
          <div className="flex items-start gap-2">
            <textarea
              ref={textareaRef}
              value={value}
              onChange={(e) => setValue(e.target.value)}
              onKeyDown={onTextareaKeyDown}
              placeholder={
                messages.length > 0
                  ? 'Ask another question...'
                  : showChip
                    ? 'Ask agents about this...'
                    : 'Ask agents anything about your workspace...'
              }
              rows={1}
              disabled={parsing || dispatching}
              className="block w-full flex-1 resize-none bg-transparent text-sm leading-5 placeholder:text-muted-foreground focus:outline-none disabled:opacity-60"
            />
            <button
              type="button"
              onClick={() => setCollapsed(true)}
              title="Hide"
              className="-mt-0.5 rounded-full p-1 text-muted-foreground transition hover:bg-muted hover:text-foreground"
            >
              <Cancel01Icon className="h-3.5 w-3.5" />
            </button>
          </div>
          <div className="flex items-center justify-between gap-2">
            <div className="min-w-0">
              {pageContext ? <ContextChip context={pageContext} /> : null}
            </div>
            <div className="flex items-center gap-1.5 shrink-0">
              {parsing ? (
                <span className="inline-flex items-center gap-1 text-[11px] text-muted-foreground">
                  <Loading01Icon className="h-3 w-3 animate-spin" />
                  Thinking…
                </span>
              ) : (
                <kbd className="hidden rounded border bg-muted px-1.5 py-0.5 text-[10px] font-mono text-muted-foreground sm:inline">
                  ↵
                </kbd>
              )}
              <button
                type="button"
                onClick={() => void submit()}
                disabled={sendDisabled}
                title="Ask agents"
                className={cn(
                  'inline-flex h-8 w-8 items-center justify-center rounded-full transition',
                  sendDisabled
                    ? 'cursor-not-allowed bg-muted text-muted-foreground'
                    : 'bg-foreground text-background hover:bg-foreground/90',
                )}
              >
                {parsing ? (
                  <Loading01Icon className="h-4 w-4 animate-spin" />
                ) : (
                  <ArrowUp01Icon className="h-4 w-4" />
                )}
              </button>
            </div>
          </div>
        </div>
      </div>
    </div>,
    document.body,
  );
}
