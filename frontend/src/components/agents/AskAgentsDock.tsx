import { useCallback, useEffect, useRef, useState } from 'react';
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

export function AskAgentsDock() {
  const workspace = useWorkspaceStore((s) => s.currentWorkspace);
  const pageContext = usePageContext();
  const addPlan = useCommandBarRunStore((s) => s.addPlan);
  const addRuns = useCommandBarRunStore((s) => s.addRuns);

  const [collapsed, setCollapsed] = useState(() => {
    if (typeof window === 'undefined') return false;
    return localStorage.getItem(COLLAPSED_KEY) === '1';
  });
  const [value, setValue] = useState('');
  const [parsing, setParsing] = useState(false);
  const [dispatching, setDispatching] = useState(false);
  const [intentResult, setIntentResult] = useState<CommandBarParseResponse | null>(null);
  const textareaRef = useRef<HTMLTextAreaElement | null>(null);

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

  // Drop the current plan if the user keeps typing
  useEffect(() => {
    setIntentResult(null);
  }, [value]);

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

  const trimmed = value.trim();
  const showChip = !!pageContext && pageContext.entity_type !== 'workspace';

  const submit = useCallback(async () => {
    if (!workspace?.id || !pageContext || !trimmed) return;
    setParsing(true);
    setIntentResult(null);
    try {
      const res = await commandBarService.parseIntent(workspace.id, {
        text: trimmed,
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
  }, [pageContext, trimmed, workspace?.id]);

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
      toast.success(
        steps.length > 1 ? `Started step 1 of ${steps.length}` : 'Agent run started',
      );
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
    return (
      <div className="pointer-events-none absolute inset-x-0 bottom-3 z-30 flex justify-center">
        <button
          type="button"
          onClick={() => setCollapsed(false)}
          className="pointer-events-auto group inline-flex items-center gap-2 rounded-full border border-border/70 bg-background/95 px-3 py-1.5 text-xs font-medium text-muted-foreground shadow-[0_1px_2px_rgba(15,23,42,0.05),0_6px_20px_-8px_rgba(15,23,42,0.18)] backdrop-blur transition hover:border-primary/40 hover:bg-background hover:text-foreground hover:shadow-[0_2px_4px_rgba(15,23,42,0.06),0_10px_28px_-10px_rgba(15,23,42,0.25)]"
        >
          <AiMagicIcon className="h-3.5 w-3.5" />
          Ask agents
          {showChip ? (
            <span className="max-w-[160px] truncate text-foreground">
              · {pageContext!.display_title || pageContext!.entity_id}
            </span>
          ) : null}
          <kbd className="ml-1 rounded border bg-muted px-1 py-0 text-[10px] font-mono text-muted-foreground">
            /
          </kbd>
        </button>
      </div>
    );
  }

  const plan = intentResult?.status === 'plan' ? intentResult.plan : null;
  const noMatch = intentResult?.status === 'no_matching_agent' ? intentResult : null;
  const sendDisabled = !trimmed || parsing || dispatching || !!plan;

  return (
    <div className="pointer-events-none absolute inset-x-0 bottom-3 z-30 flex justify-center px-4">
      <div className="pointer-events-auto w-full max-w-2xl rounded-2xl border border-border/70 bg-background/95 shadow-[0_1px_2px_rgba(15,23,42,0.06),0_8px_24px_-12px_rgba(15,23,42,0.18),0_24px_64px_-28px_rgba(15,23,42,0.28)] ring-1 ring-black/[0.02] backdrop-blur transition-shadow focus-within:shadow-[0_1px_2px_rgba(15,23,42,0.06),0_12px_32px_-12px_rgba(15,23,42,0.22),0_32px_80px_-32px_rgba(15,23,42,0.34)] dark:ring-white/[0.04]">
        <div className="flex flex-col gap-2 px-3.5 pt-3 pb-2.5">
          <div className="flex items-start gap-2">
            <textarea
              ref={textareaRef}
              value={value}
              onChange={(e) => setValue(e.target.value)}
              onKeyDown={onTextareaKeyDown}
              placeholder={
                showChip
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

        {plan ? (
          <div className="border-t border-border/60 px-3.5 pt-2.5 pb-3">
            <div className="flex items-center justify-between gap-2 text-[11px] font-medium uppercase tracking-wider text-muted-foreground">
              <span>
                {plan.plan_kind === 'fan_out'
                  ? 'Fan-out plan'
                  : plan.plan_kind === 'one_shot_command'
                    ? 'One-shot agent'
                    : 'Plan'}
              </span>
              <span className="font-normal normal-case tracking-normal text-muted-foreground/80">
                {plan.plan_kind === 'fan_out'
                  ? `${plan.steps.length} targets`
                  : plan.steps.length === 1
                    ? '1 step'
                    : `${plan.steps.length} steps`}
              </span>
            </div>
            {intentResult?.status === 'plan' && intentResult.rationale ? (
              <p className="mt-1 line-clamp-2 text-[11px] text-muted-foreground/80">
                {intentResult.rationale}
              </p>
            ) : null}
            {plan.guardrails?.length ? (
              <div className="mt-1.5 space-y-1">
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
            <div
              className={cn(
                'mt-2 space-y-1.5',
                plan.steps.length > 1 && 'relative pl-3',
              )}
            >
              {plan.steps.length > 1 ? (
                <div
                  aria-hidden
                  className="absolute left-[8px] top-3 bottom-3 w-px bg-border/70"
                />
              ) : null}
              {plan.steps.map((step, index) => (
                <div key={`${step.agent_id}-${index}`} className="relative">
                  {plan.steps.length > 1 ? (
                    <span
                      aria-hidden
                      className="absolute -left-3 top-2 grid h-4 w-4 place-items-center rounded-full bg-background ring-2 ring-border/70 text-[9px] font-semibold text-muted-foreground"
                    >
                      {index + 1}
                    </span>
                  ) : null}
                  <div className="rounded-md border border-border/60 bg-muted/30 px-2.5 py-2">
                    <p className="text-sm font-medium leading-snug text-foreground line-clamp-2">
                      {step.instructions}
                    </p>
                    <div className="mt-1 flex items-center gap-1.5 text-[11px] text-muted-foreground">
                      <BotIcon className="h-3 w-3" />
                      <span className="truncate">{step.agent_name}</span>
                      <span className="opacity-60">·</span>
                      <span>{step.target.entity_type.replace('_', ' ')}</span>
                      {step.target.display_title ? (
                        <>
                          <span className="opacity-60">·</span>
                          <span className="truncate">{step.target.display_title}</span>
                        </>
                      ) : null}
                    </div>
                  </div>
                </div>
              ))}
            </div>
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
          <div className="border-t border-border/60 px-3.5 pt-2.5 pb-3">
            <div className="flex items-start gap-2">
              <BotIcon className="mt-0.5 h-3.5 w-3.5 text-muted-foreground" />
              <div className="min-w-0 text-sm">
                <p className="font-medium">No available agent can do that yet.</p>
                <p className="mt-0.5 text-xs text-muted-foreground">{noMatch.reason}</p>
                {noMatch.suggestions?.length ? (
                  <div className="mt-2 flex flex-wrap gap-1.5">
                    {noMatch.suggestions.map((s) => (
                      <span
                        key={s}
                        className="rounded border px-2 py-0.5 text-[11px] text-muted-foreground"
                      >
                        {s}
                      </span>
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
  );
}
