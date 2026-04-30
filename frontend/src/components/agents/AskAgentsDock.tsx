import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { createPortal } from 'react-dom';
import { toast } from 'sonner';
import { AiMagicIcon, BotIcon, Loading01Icon } from '@/lib/icons';
import { cn } from '@/lib/utils';
import { usePageContext } from '@/components/command-bar/pageContext';
import { useWorkspaceStore } from '@/stores/workspaceStore';
import { commandBarService } from '@/lib/services/commandBarService';
import { agentService } from '@/lib/services/agentService';
import { useCommandBarRunStore, type CommandBarRunPlan } from '@/stores/commandBarStore';
import { ACTIVE_RUN_STATUSES } from '@/components/pm/agentRunConstants';
import { PromotionDialog } from '@/components/command-bar/PromotionDialog';
import { CodingSessionDrawer } from '@/components/pm/CodingSession/CodingSessionDrawer';
import type {
  AgentRun,
  CommandBarPlanStep,
  CommandBarParseResponse,
} from '@/lib/pmTypes';
import { DockHeader } from './dock/DockHeader';
import { DockInput } from './dock/DockInput';
import { ExecutionStrip, type StripAction } from './dock/ExecutionStrip';
import { InlineResultCard } from './dock/InlineResultCard';
import { RunListView } from './dock/RunListView';
import { PlanPreview } from './dock/PlanPreview';
import { outputSummaryText } from './dock/utils';

const COLLAPSED_KEY = 'helpin:ask-agents-dock-collapsed';

type ThreadMessage = { kind: 'user'; id: string; text: string; ts: number };

type AskAgentsEventDetail = {
  query?: string;
  mode?: 'compose' | 'runs';
  runId?: string;
};

export function AskAgentsDock() {
  const workspace = useWorkspaceStore((s) => s.currentWorkspace);
  const pageContext = usePageContext();

  const addPlan = useCommandBarRunStore((s) => s.addPlan);
  const addRuns = useCommandBarRunStore((s) => s.addRuns);
  const runIds = useCommandBarRunStore((s) => s.runIds);
  const runsById = useCommandBarRunStore((s) => s.runsById);
  const planIds = useCommandBarRunStore((s) => s.planIds);
  const plansById = useCommandBarRunStore((s) => s.plansById);
  const hydratePlans = useCommandBarRunStore((s) => s.hydratePlans);
  const updateRun = useCommandBarRunStore((s) => s.updateRun);
  const updatePlan = useCommandBarRunStore((s) => s.updatePlan);
  const viewMode = useCommandBarRunStore((s) => s.viewMode);
  const setViewMode = useCommandBarRunStore((s) => s.setViewMode);
  const listFilter = useCommandBarRunStore((s) => s.listFilter);
  const setListFilter = useCommandBarRunStore((s) => s.setListFilter);

  const [collapsed, setCollapsed] = useState(() => {
    if (typeof window === 'undefined') return false;
    return localStorage.getItem(COLLAPSED_KEY) === '1';
  });
  const [hiddenByModal, setHiddenByModal] = useState(false);
  const [isFocused, setIsFocused] = useState(false);
  const [value, setValue] = useState('');
  const [parsing, setParsing] = useState(false);
  const [dispatching, setDispatching] = useState(false);
  const [intentResult, setIntentResult] = useState<CommandBarParseResponse | null>(null);
  const [messages, setMessages] = useState<ThreadMessage[]>([]);
  const [sessionPlanIds, setSessionPlanIds] = useState<Set<string>>(() => new Set());
  const [sessionRunIds, setSessionRunIds] = useState<Set<string>>(() => new Set());
  const [busyRunId, setBusyRunId] = useState<string | null>(null);
  const [busyPlanId, setBusyPlanId] = useState<string | null>(null);
  const [selectedRunId, setSelectedRunId] = useState<string | null>(null);
  const [promotionRun, setPromotionRun] = useState<{
    run: AgentRun;
    step: CommandBarPlanStep | null;
    planPrompt?: string;
  } | null>(null);

  const textareaRef = useRef<HTMLTextAreaElement | null>(null);
  const responseRef = useRef<HTMLDivElement | null>(null);
  const planRefetchTimers = useRef<Map<string, ReturnType<typeof setTimeout>>>(new Map());

  const runs = useMemo(
    () => runIds.map((id) => runsById[id]).filter(Boolean),
    [runIds, runsById],
  );
  const plans = useMemo(
    () => planIds.map((id) => plansById[id]).filter(Boolean),
    [planIds, plansById],
  );
  const planRunIds = useMemo(() => {
    const ids = new Set<string>();
    for (const plan of plans) {
      for (const id of Object.values(plan.runIdsByStep)) ids.add(id);
    }
    return ids;
  }, [plans]);
  const standaloneRuns = useMemo(
    () => runs.filter((run) => !planRunIds.has(run.id)),
    [planRunIds, runs],
  );

  const activeCount = useMemo(
    () => runs.filter((r) => ACTIVE_RUN_STATUSES.has(r.status)).length,
    [runs],
  );

  // Items visible in conversation = items submitted in *this* session
  // (kept after completion so the user can see the result) plus anything
  // currently active (so refreshes show in-flight work). Completed items from
  // prior sessions live in history (list mode), not conversation.
  const visiblePlanIds = useMemo(() => {
    const ids = new Set(sessionPlanIds);
    for (const plan of plans) {
      const hasActive = Object.values(plan.runIdsByStep)
        .map((id) => runsById[id])
        .some((r) => r && ACTIVE_RUN_STATUSES.has(r.status));
      if (hasActive) ids.add(plan.id);
    }
    return ids;
  }, [plans, runsById, sessionPlanIds]);

  const visibleRunIds = useMemo(() => {
    const ids = new Set(sessionRunIds);
    for (const run of standaloneRuns) {
      if (ACTIVE_RUN_STATUSES.has(run.status)) ids.add(run.id);
    }
    return ids;
  }, [sessionRunIds, standaloneRuns]);

  // Conversation timeline: only items started in THIS session, plus the user
  // bubbles, ordered by timestamp. Hydrated history lives in list mode.
  type TimelineItem =
    | { kind: 'msg'; id: string; ts: number; msg: ThreadMessage }
    | { kind: 'plan'; id: string; ts: number; plan: CommandBarRunPlan }
    | { kind: 'run'; id: string; ts: number; run: AgentRun };

  const timeline = useMemo<TimelineItem[]>(() => {
    const items: TimelineItem[] = [];
    for (const m of messages) items.push({ kind: 'msg', id: m.id, ts: m.ts, msg: m });
    for (const plan of plans) {
      if (!visiblePlanIds.has(plan.id)) continue;
      const ts = Math.max(
        ...Object.values(plan.runIdsByStep)
          .map((id) => runsById[id])
          .filter(Boolean)
          .map((r) => Date.parse(r.created_at) || 0),
        0,
      );
      items.push({ kind: 'plan', id: `plan-${plan.id}`, ts, plan });
    }
    for (const run of standaloneRuns) {
      if (!visibleRunIds.has(run.id)) continue;
      items.push({
        kind: 'run',
        id: `run-${run.id}`,
        ts: Date.parse(run.created_at) || 0,
        run,
      });
    }
    return items.sort((a, b) => a.ts - b.ts);
  }, [messages, plans, runsById, standaloneRuns, visiblePlanIds, visibleRunIds]);

  // Persist collapsed.
  useEffect(() => {
    localStorage.setItem(COLLAPSED_KEY, collapsed ? '1' : '0');
  }, [collapsed]);

  // Drop the current pending plan if the user keeps typing.
  useEffect(() => {
    setIntentResult(null);
  }, [value]);

  // Auto-scroll on new content.
  useEffect(() => {
    const el = responseRef.current;
    if (!el) return;
    el.scrollTop = el.scrollHeight;
  }, [timeline.length, intentResult, parsing]);

  // Initial hydration.
  useEffect(() => {
    if (!workspace?.id) return;
    let cancelled = false;
    void commandBarService.listPlans(workspace.id, 10).then((res) => {
      if (cancelled || !res.data?.plans) return;
      hydratePlans(res.data.plans);
      // Any plan whose runs include an active one belongs in the conversation —
      // the user expects to see in-flight work when they reopen the dock.
      const activePlanIds = res.data.plans
        .filter((p) =>
          (p.runs ?? []).some((r) => ACTIVE_RUN_STATUSES.has(r.status)),
        )
        .map((p) => p.id);
      if (activePlanIds.length) {
        setSessionPlanIds((prev) => {
          const next = new Set(prev);
          for (const id of activePlanIds) next.add(id);
          return next;
        });
      }
    });
    void agentService.listRecentRuns(workspace.id, 20).then((res) => {
      if (cancelled || !res.data?.runs?.length) return;
      addRuns(res.data.runs);
      const activeRunIds = res.data.runs
        .filter((r) => ACTIVE_RUN_STATUSES.has(r.status))
        .map((r) => r.id);
      if (activeRunIds.length) {
        setSessionRunIds((prev) => {
          const next = new Set(prev);
          for (const id of activeRunIds) next.add(id);
          return next;
        });
      }
    });
    return () => {
      cancelled = true;
    };
  }, [addRuns, hydratePlans, workspace?.id]);

  const findPlanIdForRun = useCallback(
    (runId: string): string | null => {
      for (const plan of plans) {
        for (const id of Object.values(plan.runIdsByStep)) {
          if (id === runId) return plan.id;
        }
      }
      return null;
    },
    [plans],
  );

  const schedulePlanRefetch = useCallback(
    (planId: string) => {
      if (!workspace?.id) return;
      const existing = planRefetchTimers.current.get(planId);
      if (existing) clearTimeout(existing);
      const timer = setTimeout(() => {
        planRefetchTimers.current.delete(planId);
        void commandBarService.getPlan(workspace.id, planId).then((res) => {
          if (res.data?.plan) updatePlan(res.data.plan, res.data.plan.runs ?? []);
        });
      }, 600);
      planRefetchTimers.current.set(planId, timer);
    },
    [updatePlan, workspace?.id],
  );

  useEffect(() => {
    const timers = planRefetchTimers.current;
    return () => {
      for (const t of timers.values()) clearTimeout(t);
      timers.clear();
    };
  }, []);

  const refreshRun = useCallback(
    async (runId: string, allowUnknown = false) => {
      if (!workspace?.id || (!allowUnknown && !runIds.includes(runId))) return;
      const res = await agentService.getRun(workspace.id, runId);
      if (!res.data) return;
      updateRun(res.data);
      const terminal =
        res.data.status === 'completed' ||
        res.data.status === 'failed' ||
        res.data.status === 'cancelled';
      if (terminal) {
        const planId = findPlanIdForRun(res.data.id);
        if (planId) schedulePlanRefetch(planId);
      }
    },
    [findPlanIdForRun, runIds, schedulePlanRefetch, updateRun, workspace?.id],
  );

  // WebSocket-driven refresh.
  useEffect(() => {
    const createdHandler = (event: Event) => {
      const detail = (event as CustomEvent<{ entity_id?: string }>).detail;
      if (detail?.entity_id) void refreshRun(detail.entity_id, true);
    };
    const updatedHandler = (event: Event) => {
      const detail = (event as CustomEvent<{ entity_id?: string }>).detail;
      if (detail?.entity_id) void refreshRun(detail.entity_id);
    };
    window.addEventListener('agent_run-created', createdHandler);
    window.addEventListener('agent_run-updated', updatedHandler);
    return () => {
      window.removeEventListener('agent_run-created', createdHandler);
      window.removeEventListener('agent_run-updated', updatedHandler);
    };
  }, [refreshRun]);

  // "/" focuses the dock when no other input is focused.
  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if (e.key !== '/') return;
      const target = e.target as HTMLElement | null;
      if (!target) return;
      const tag = target.tagName?.toLowerCase();
      const isEditable = tag === 'input' || tag === 'textarea' || target.isContentEditable;
      if (isEditable) return;
      e.preventDefault();
      setCollapsed(false);
      setViewMode('conversation');
      requestAnimationFrame(() => textareaRef.current?.focus());
    };
    document.addEventListener('keydown', onKey);
    return () => document.removeEventListener('keydown', onKey);
  }, [setViewMode]);

  // Auto-collapse to pill when idle.
  useEffect(() => {
    if (collapsed) return;
    if (parsing || dispatching) return;
    if (intentResult) return;
    if (value.trim().length > 0) return;
    if (isFocused) return;
    const IDLE_COLLAPSE_MS = 60_000;
    const timer = setTimeout(() => setCollapsed(true), IDLE_COLLAPSE_MS);
    return () => clearTimeout(timer);
  }, [collapsed, parsing, dispatching, intentResult, value, isFocused]);

  // Hide while a centered modal dialog is open. Sheets pass through.
  useEffect(() => {
    const update = () => {
      const open = document.querySelectorAll(
        '[role="dialog"][data-state="open"], [role="alertdialog"][data-state="open"]',
      );
      let blocking = false;
      open.forEach((el) => {
        if (el.hasAttribute('data-side')) return;
        if (el.closest('[data-helpin-dock]')) return;
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

  // External callers: window.dispatchEvent(new CustomEvent('helpin:ask-agents', { detail }))
  useEffect(() => {
    const onAsk = (event: Event) => {
      const detail = (event as CustomEvent<AskAgentsEventDetail>).detail;
      const next = detail?.query?.trim() ?? '';
      if (!next && !detail?.mode && !detail?.runId) return;
      setCollapsed(false);
      if (detail?.mode === 'runs') {
        setViewMode('list');
      }
      if (detail?.runId) {
        // jump straight to that run's conversation context — switch to conversation mode
        setViewMode('conversation');
      }
      if (next) {
        setValue(next);
        setIntentResult(null);
        requestAnimationFrame(() => textareaRef.current?.focus());
      }
    };
    window.addEventListener('helpin:ask-agents', onAsk);
    return () => window.removeEventListener('helpin:ask-agents', onAsk);
  }, [setViewMode]);

  const trimmed = value.trim();

  const submit = useCallback(
    async (override?: string) => {
      const text = (override ?? value).trim();
      if (!workspace?.id || !pageContext || !text) return;
      // List-mode submit: if any rows match, treat as filter; otherwise dispatch a new run.
      if (viewMode === 'list') {
        setViewMode('conversation');
      }
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
    [pageContext, value, viewMode, setViewMode, workspace?.id],
  );

  const confirmPlan = useCallback(async () => {
    if (!workspace?.id || !pageContext || !intentResult || intentResult.status !== 'plan') return;
    const submittedAt = Date.now();
    setMessages((prev) => [
      ...prev,
      { kind: 'user', id: `user-${submittedAt}`, text: trimmed, ts: submittedAt },
    ]);
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
            runIdsByStep: Object.fromEntries(res.data.runs.map((run, index) => [index, run.id])),
            planKind: intentResult.plan.plan_kind,
            status: 'running',
            prompt: trimmed,
            currentStepIndex: 0,
          },
          res.data.runs,
        );
        setSessionPlanIds((prev) => {
          const next = new Set(prev);
          next.add(res.data!.plan_id!);
          return next;
        });
      } else {
        addRuns(res.data.runs);
        setSessionRunIds((prev) => {
          const next = new Set(prev);
          for (const r of res.data!.runs) next.add(r.id);
          return next;
        });
      }
      setValue('');
      setIntentResult(null);
    } finally {
      setDispatching(false);
    }
  }, [addPlan, addRuns, intentResult, pageContext, trimmed, workspace?.id]);

  // ⌘↵ confirms a pending plan from anywhere in the dock.
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

  const cancelPlan = useCallback(
    async (planId: string) => {
      if (!workspace?.id) return;
      setBusyPlanId(planId);
      try {
        const res = await commandBarService.cancelPlan(workspace.id, planId);
        if (res.error || !res.data) {
          toast.error(res.error ?? 'Failed to cancel plan');
          return;
        }
        updatePlan(res.data.plan, res.data.runs ?? []);
        toast.success('Command plan cancelled');
      } finally {
        setBusyPlanId(null);
      }
    },
    [updatePlan, workspace?.id],
  );

  const retryPlan = useCallback(
    async (plan: CommandBarRunPlan) => {
      if (!workspace?.id) return;
      const failedIndex = plan.steps.findIndex((_, i) => {
        const runId = plan.runIdsByStep[i];
        const run = runId ? runsById[runId] : null;
        return run && (run.status === 'failed' || run.status === 'cancelled');
      });
      const stepIndex = failedIndex >= 0 ? failedIndex : 0;
      setBusyPlanId(plan.id);
      try {
        const res = await commandBarService.retryPlan(workspace.id, plan.id, stepIndex);
        if (res.error || !res.data) {
          toast.error(res.error ?? 'Failed to retry plan');
          return;
        }
        const retryRuns = res.data.runs ?? (res.data.run ? [res.data.run] : []);
        updatePlan(res.data.plan, retryRuns);
      } finally {
        setBusyPlanId(null);
      }
    },
    [runsById, updatePlan, workspace?.id],
  );

  const retryRun = useCallback(
    (run: AgentRun) => {
      // Standalone runs are usually recoverable by re-issuing the original prompt.
      const text = (run.input as { text?: string } | null)?.text;
      if (text) {
        setCollapsed(false);
        setViewMode('conversation');
        setValue(text);
        void submit(text);
      } else {
        toast.error('Cannot retry: original prompt unavailable.');
      }
    },
    [submit, setViewMode],
  );

  const runApprove = useCallback(
    async (run: AgentRun) => {
      if (!workspace?.id) return;
      setBusyRunId(run.id);
      try {
        const res = await agentService.approveRun(workspace.id, run.id);
        if (res.error || !res.data) {
          toast.error(res.error ?? 'Failed to approve run');
          return;
        }
        updateRun(res.data);
      } finally {
        setBusyRunId(null);
      }
    },
    [updateRun, workspace?.id],
  );

  const runCancel = useCallback(
    async (run: AgentRun) => {
      if (!workspace?.id) return;
      setBusyRunId(run.id);
      try {
        const res = await agentService.cancelRun(workspace.id, run.id);
        if (res.error || !res.data) {
          toast.error(res.error ?? 'Failed to cancel run');
          return;
        }
        updateRun(res.data);
      } finally {
        setBusyRunId(null);
      }
    },
    [updateRun, workspace?.id],
  );

  const findRunStep = useCallback(
    (run: AgentRun): { step: CommandBarPlanStep | null; planPrompt?: string } => {
      for (const plan of plans) {
        for (const [indexStr, runId] of Object.entries(plan.runIdsByStep)) {
          if (runId !== run.id) continue;
          const step = plan.steps[Number(indexStr)] ?? null;
          return { step, planPrompt: plan.prompt };
        }
      }
      return { step: null };
    },
    [plans],
  );

  const openRunDrawer = useCallback((runId: string) => {
    setSelectedRunId(runId);
  }, []);

  const handleStripAction = useCallback(
    (
      action: StripAction,
      target: { kind: 'plan'; plan: CommandBarRunPlan } | { kind: 'run'; run: AgentRun },
    ) => {
      if (action === 'open') {
        const runId = target.kind === 'plan'
          ? Object.values(target.plan.runIdsByStep)[0]
          : target.run.id;
        if (runId) openRunDrawer(runId);
        return;
      }
      if (action === 'rerun') {
        if (target.kind === 'plan') {
          if (target.plan.prompt) {
            setValue(target.plan.prompt);
            void submit(target.plan.prompt);
          }
        } else {
          const display = target.run.approval_state === 'approved' ? null : target.run;
          if (display) void runApprove(target.run);
        }
        return;
      }
      if (action === 'save_as_agent') {
        if (target.kind === 'run') {
          const { step, planPrompt } = findRunStep(target.run);
          if (!step) {
            toast.error('Cannot save: this run is missing its plan context.');
            return;
          }
          setPromotionRun({ run: target.run, step, planPrompt });
        } else {
          // Plan: promote the first/only completed run.
          const firstRunId = Object.values(target.plan.runIdsByStep)[0];
          const run = firstRunId ? runsById[firstRunId] : null;
          if (run) {
            const { step, planPrompt } = findRunStep(run);
            if (step) setPromotionRun({ run, step, planPrompt: planPrompt ?? target.plan.prompt });
          }
        }
        return;
      }
      if (action === 'retry') {
        if (target.kind === 'plan') void retryPlan(target.plan);
        else retryRun(target.run);
        return;
      }
      if (action === 'cancel') {
        if (target.kind === 'plan') void cancelPlan(target.plan.id);
        else void runCancel(target.run);
      }
    },
    [
      cancelPlan,
      findRunStep,
      openRunDrawer,
      retryPlan,
      retryRun,
      runApprove,
      runCancel,
      runsById,
      submit,
    ],
  );

  const onNew = useCallback(() => {
    setMessages([]);
    setSessionPlanIds(new Set());
    setSessionRunIds(new Set());
    setIntentResult(null);
    setValue('');
    setViewMode('conversation');
    requestAnimationFrame(() => textareaRef.current?.focus());
  }, [setViewMode]);

  const onListSelect = useCallback(
    (item: { kind: 'plan'; plan: CommandBarRunPlan } | { kind: 'run'; run: AgentRun }) => {
      const runId =
        item.kind === 'plan' ? Object.values(item.plan.runIdsByStep)[0] : item.run.id;
      if (runId) openRunDrawer(runId);
    },
    [openRunDrawer],
  );

  if (!workspace) return null;
  if (hiddenByModal) return null;

  if (collapsed) {
    if (typeof document === 'undefined') return null;
    return createPortal(
      <div
        data-helpin-dock="true"
        className="pointer-events-none fixed inset-x-0 bottom-8 z-[60] flex justify-center"
      >
        <button
          type="button"
          onClick={() => setCollapsed(false)}
          className="pointer-events-auto group inline-flex items-center gap-2.5 rounded-full border border-border/70 bg-background/95 px-4 py-2.5 text-sm font-medium text-muted-foreground shadow-[0_1px_2px_rgba(15,23,42,0.05),0_8px_24px_-8px_rgba(15,23,42,0.22)] backdrop-blur transition hover:border-foreground/30 hover:bg-background hover:text-foreground"
        >
          <AiMagicIcon className="h-4 w-4" />
          Ask agents
          {activeCount > 0 ? (
            <span className="inline-flex items-center gap-1 rounded-full bg-orange-500/10 px-2 py-0.5 text-[11px] font-medium text-orange-700 dark:text-orange-300">
              <Loading01Icon className="h-3 w-3 animate-spin" />
              {activeCount}
            </span>
          ) : null}
          <kbd className="ml-1 rounded border bg-muted px-1.5 py-0.5 text-[11px] font-mono text-muted-foreground">
            /
          </kbd>
        </button>
      </div>,
      document.body,
    );
  }

  const plan = intentResult?.status === 'plan' ? intentResult.plan : null;
  const noMatch = intentResult?.status === 'no_matching_agent' ? intentResult : null;
  const hasResponseArea = timeline.length > 0 || !!plan || !!noMatch;

  if (typeof document === 'undefined') return null;
  return createPortal(
    <div
      data-helpin-dock="true"
      className="pointer-events-none fixed inset-x-0 bottom-6 z-[60] flex justify-center px-4"
    >
      <div className="pointer-events-auto flex w-full max-w-2xl flex-col rounded-2xl border border-border/70 bg-background/95 shadow-[0_1px_2px_rgba(15,23,42,0.06),0_8px_24px_-12px_rgba(15,23,42,0.18),0_24px_64px_-28px_rgba(15,23,42,0.28)] ring-1 ring-black/[0.02] backdrop-blur transition-shadow focus-within:shadow-[0_1px_2px_rgba(15,23,42,0.06),0_12px_32px_-12px_rgba(15,23,42,0.22),0_32px_80px_-32px_rgba(15,23,42,0.34)] dark:ring-white/[0.04] animate-in fade-in zoom-in-95 slide-in-from-bottom-2 duration-200 ease-out">
        <div className="border-b border-border/60">
          <DockHeader
            mode={viewMode}
            activeCount={activeCount}
            onSwapMode={() => setViewMode(viewMode === 'list' ? 'conversation' : 'list')}
            onNew={onNew}
            onClose={() => setCollapsed(true)}
          />
        </div>

        {viewMode === 'list' ? (
          <div ref={responseRef} className="max-h-[60vh] overflow-y-auto">
            <RunListView
              plans={plans}
              standaloneRuns={standaloneRuns}
              runsById={runsById}
              filter={listFilter}
              busyPlanId={busyPlanId}
              busyRunId={busyRunId}
              onSelect={onListSelect}
              onRetryPlan={(p) => void retryPlan(p)}
              onRetryRun={retryRun}
            />
          </div>
        ) : hasResponseArea ? (
          <div ref={responseRef} className="flex max-h-[60vh] flex-col gap-3 overflow-y-auto px-3.5 py-3">
            {timeline.map((item) => {
              if (item.kind === 'msg') {
                return (
                  <div key={item.id} className="flex justify-end">
                    <div className="max-w-[80%] rounded-2xl rounded-tr-sm bg-primary/10 px-3 py-1.5 text-sm text-foreground">
                      {item.msg.text}
                    </div>
                  </div>
                );
              }
              if (item.kind === 'plan') {
                const onlyRunId = Object.values(item.plan.runIdsByStep)[0];
                const onlyRun = onlyRunId ? runsById[onlyRunId] : null;
                const summary =
                  item.plan.steps.length === 1 && onlyRun ? outputSummaryText(onlyRun) : '';
                return (
                  <ExecutionStrip
                    key={item.id}
                    kind="plan"
                    workspaceId={workspace.id}
                    plan={item.plan}
                    runsById={runsById}
                    busyPlanId={busyPlanId}
                    onAction={(a) => handleStripAction(a, { kind: 'plan', plan: item.plan })}
                    resultSlot={summary ? <InlineResultCard>{summary}</InlineResultCard> : null}
                  />
                );
              }
              const summary = outputSummaryText(item.run);
              return (
                <ExecutionStrip
                  key={item.id}
                  kind="run"
                  workspaceId={workspace.id}
                  run={item.run}
                  busy={busyRunId === item.run.id}
                  onAction={(a) => handleStripAction(a, { kind: 'run', run: item.run })}
                  resultSlot={summary ? <InlineResultCard>{summary}</InlineResultCard> : null}
                />
              );
            })}

            {plan ? (
              <PlanPreview
                plan={plan}
                rationale={
                  intentResult?.status === 'plan' ? intentResult.rationale ?? null : null
                }
                dispatching={dispatching}
                onConfirm={() => void confirmPlan()}
                onEdit={() => {
                  setIntentResult(null);
                  requestAnimationFrame(() => textareaRef.current?.focus());
                }}
                onDiscard={() => setIntentResult(null)}
              />
            ) : null}

            {noMatch ? (
              <NoMatchBlock
                reason={noMatch.reason}
                suggestions={noMatch.suggestions}
                onPick={(s) => {
                  setValue(s);
                  void submit(s);
                }}
                onDismiss={() => setIntentResult(null)}
              />
            ) : null}
          </div>
        ) : null}

        <div className={cn(hasResponseArea && viewMode === 'conversation' ? 'border-t border-border/60' : viewMode === 'list' ? 'border-t border-border/60' : null)}>
          <DockInput
            mode={viewMode}
            value={viewMode === 'list' ? listFilter : value}
            onChange={(v) => (viewMode === 'list' ? setListFilter(v) : setValue(v))}
            onSubmit={() => {
              if (viewMode === 'list') {
                const text = listFilter.trim();
                if (!text) return;
                setListFilter('');
                setValue(text);
                void submit(text);
              } else {
                void submit();
              }
            }}
            onFocusChange={setIsFocused}
            pageContext={pageContext ?? null}
            busy={parsing || dispatching}
            textareaRef={textareaRef}
          />
        </div>
      </div>
      <PromotionDialog
        open={!!promotionRun}
        onOpenChange={(open) => {
          if (!open) setPromotionRun(null);
        }}
        workspaceId={workspace.id}
        run={promotionRun?.run ?? null}
        step={promotionRun?.step ?? null}
        planPrompt={promotionRun?.planPrompt}
      />
      <CodingSessionDrawer
        sessionId={selectedRunId}
        open={!!selectedRunId}
        onOpenChange={(open) => {
          if (!open) setSelectedRunId(null);
        }}
      />
    </div>,
    document.body,
  );
}


function NoMatchBlock({
  reason,
  suggestions,
  onPick,
  onDismiss,
}: {
  reason: string;
  suggestions?: string[];
  onPick: (s: string) => void;
  onDismiss: () => void;
}) {
  return (
    <div>
      <div className="flex items-start gap-2">
        <BotIcon className="mt-0.5 h-3.5 w-3.5 text-muted-foreground" />
        <div className="min-w-0 text-sm">
          <p className="font-medium">No available agent can do that yet.</p>
          <p className="mt-0.5 text-xs text-muted-foreground">{reason}</p>
          {suggestions?.length ? (
            <div className="mt-2 flex flex-wrap gap-1.5">
              {suggestions.map((s) => (
                <button
                  type="button"
                  key={s}
                  onClick={() => onPick(s)}
                  className="rounded border border-border/70 bg-background/80 px-2 py-0.5 text-[11px] text-foreground transition hover:border-foreground/30 hover:bg-muted/60"
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
          onClick={onDismiss}
          className="rounded-md px-2 py-1 text-[11px] font-medium text-muted-foreground transition hover:bg-muted hover:text-foreground"
        >
          Dismiss
        </button>
      </div>
    </div>
  );
}
