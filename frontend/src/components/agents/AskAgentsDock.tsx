import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { createPortal } from 'react-dom';
import { toast } from 'sonner';
import { AiMagicIcon, BotIcon, Loading01Icon, PauseIcon, Tick01Icon } from '@/lib/icons';
import { cn } from '@/lib/utils';
import { usePageContextState } from '@/components/command-bar/pageContext';
import { useWorkspaceStore } from '@/stores/workspaceStore';
import { commandBarService } from '@/lib/services/commandBarService';
import { agentService } from '@/lib/services/agentService';
import { useCommandBarRunStore, type CommandBarRunPlan } from '@/stores/commandBarStore';
import { ACTIVE_RUN_STATUSES, isPausedAgentRun } from '@/components/pm/agentRunConstants';
import { PromotionDialog } from '@/components/command-bar/PromotionDialog';
import { CodingSessionDrawer } from '@/components/pm/CodingSession/CodingSessionDrawer';
import { MarkdownContent } from '@/components/pm/CodingSession/MarkdownContent';
import type {
  AgentRun,
  CommandBarMessageSummary,
  CommandBarProposal,
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

type ThreadMessage = {
  kind: 'user' | 'assistant';
  id: string;
  text: string;
  ts: number;
  proposal?: CommandBarProposal;
};

type AskAgentsEventDetail = {
  query?: string;
  mode?: 'compose' | 'runs';
  runId?: string;
};

function threadMessageFromSummary(message: CommandBarMessageSummary): ThreadMessage {
  return {
    kind: message.role,
    id: message.id,
    text: message.content,
    ts: Date.parse(message.created_at) || Date.now(),
    proposal: message.proposal,
  };
}

function normalizeTranscriptPrompt(text: string | undefined | null): string {
  return (text ?? '').replace(/\s+/g, ' ').trim().toLowerCase();
}

function isNearOrAfterMessage(itemTime: string | undefined, messageTs: number): boolean {
  const ts = itemTime ? Date.parse(itemTime) : 0;
  if (!ts || !messageTs) return true;
  const twoMinutes = 2 * 60 * 1000;
  const oneDay = 24 * 60 * 60 * 1000;
  return ts >= messageTs - twoMinutes && ts <= messageTs + oneDay;
}

function planMatchesRestoredThread(plan: CommandBarRunPlan, messages: ThreadMessage[]): boolean {
  const prompt = normalizeTranscriptPrompt(plan.prompt);
  if (!prompt) return false;
  return messages.some(
    (message) =>
      message.kind === 'user' &&
      normalizeTranscriptPrompt(message.text) === prompt &&
      isNearOrAfterMessage(plan.createdAt, message.ts),
  );
}

function runPrompt(run: AgentRun): string {
  const input = run.input ?? {};
  for (const key of ['text', 'prompt', 'instructions']) {
    const value = input[key];
    if (typeof value === 'string' && value.trim()) return value;
  }
  const trigger = input.trigger;
  if (trigger && typeof trigger === 'object') {
    const context = (trigger as { context?: unknown }).context;
    if (context && typeof context === 'object') {
      const text = (context as { text?: unknown; prompt?: unknown }).text;
      if (typeof text === 'string' && text.trim()) return text;
      const prompt = (context as { prompt?: unknown }).prompt;
      if (typeof prompt === 'string' && prompt.trim()) return prompt;
    }
  }
  return '';
}

function runMatchesRestoredThread(run: AgentRun, messages: ThreadMessage[]): boolean {
  const prompt = normalizeTranscriptPrompt(runPrompt(run));
  if (!prompt) return false;
  return messages.some(
    (message) =>
      message.kind === 'user' &&
      normalizeTranscriptPrompt(message.text) === prompt &&
      isNearOrAfterMessage(run.created_at, message.ts),
  );
}

export function AskAgentsDock() {
  const workspace = useWorkspaceStore((s) => s.currentWorkspace);
  const { pageContext, scopeOptions, activeScopeKey, setActiveScopeKey } = usePageContextState();

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
  const clearRuns = useCommandBarRunStore((s) => s.clear);

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
  const [chatThreadId, setChatThreadId] = useState<string | null>(null);
  const [sessionPlanIds, setSessionPlanIds] = useState<Set<string>>(() => new Set());
  const [sessionRunIds, setSessionRunIds] = useState<Set<string>>(() => new Set());
  const [busyRunId, setBusyRunId] = useState<string | null>(null);
  const [busyPlanId, setBusyPlanId] = useState<string | null>(null);
  const [selectedRunId, setSelectedRunId] = useState<string | null>(null);
  // When the dock first opens with pending approvals, auto-expand the first
  // one so the user lands on something actionable. Pressing "New" resets this
  // to false — pending work stays visible as collapsed one-liners above the
  // input but doesn't dominate the freshly-cleared canvas.
  const [autoExpandFirstPaused, setAutoExpandFirstPaused] = useState(true);
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

  // Split active runs into truly running (queued/running) vs awaiting human
  // (paused = awaiting input/approval/auth). Paused runs are not "doing work" —
  // surfacing them under the same spinner as a running agent misleads the user.
  const { runningCount, awaitingCount } = useMemo(() => {
    let running = 0;
    let awaiting = 0;
    for (const r of runs) {
      if (!ACTIVE_RUN_STATUSES.has(r.status)) continue;
      if (isPausedAgentRun(r)) awaiting += 1;
      else running += 1;
    }
    return { runningCount: running, awaitingCount: awaiting };
  }, [runs]);

  // Items visible in conversation = items submitted in *this* session, anything
  // currently active, and completed runs that can be matched back to the
  // restored chat thread. Other completed runs stay in history/list mode.
  const visiblePlanIds = useMemo(() => {
    const ids = new Set(sessionPlanIds);
    for (const plan of plans) {
      const hasActive = Object.values(plan.runIdsByStep)
        .map((id) => runsById[id])
        .some((r) => r && ACTIVE_RUN_STATUSES.has(r.status));
      if (hasActive || planMatchesRestoredThread(plan, messages)) ids.add(plan.id);
    }
    return ids;
  }, [messages, plans, runsById, sessionPlanIds]);

  const visibleRunIds = useMemo(() => {
    const ids = new Set(sessionRunIds);
    for (const run of standaloneRuns) {
      if (ACTIVE_RUN_STATUSES.has(run.status) || runMatchesRestoredThread(run, messages)) ids.add(run.id);
    }
    return ids;
  }, [messages, sessionRunIds, standaloneRuns]);

  // Conversation timeline: chat bubbles plus run items that belong to this
  // transcript, ordered by timestamp. Unrelated hydrated history lives in list
  // mode.
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

  // Every paused run (awaiting approval/input/auth) is collapsible — the
  // header chevron toggles it. Only the first paused run starts expanded so
  // the user lands on something actionable; subsequent ones start collapsed.
  const { compactRunIds, firstPausedRunId } = useMemo(() => {
    const compact = new Set<string>();
    let first: string | null = null;
    for (const item of timeline) {
      if (item.kind !== 'run') continue;
      if (!isPausedAgentRun(item.run)) continue;
      compact.add(item.run.id);
      if (!first) first = item.run.id;
    }
    return { compactRunIds: compact, firstPausedRunId: first };
  }, [timeline]);

  // Persist collapsed.
  useEffect(() => {
    localStorage.setItem(COLLAPSED_KEY, collapsed ? '1' : '0');
  }, [collapsed]);

  // Drop the current pending plan if the user keeps typing.
  useEffect(() => {
    setIntentResult(null);
  }, [value]);

  useEffect(() => {
    clearRuns();
    setChatThreadId(null);
    setMessages([]);
    setSessionPlanIds(new Set());
    setSessionRunIds(new Set());
    setSelectedRunId(null);
    setIntentResult(null);
  }, [clearRuns, workspace?.id]);

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
    void commandBarService.listChatThreads(workspace.id, 1).then((res) => {
      if (cancelled || !res.data?.threads?.length) return;
      const [latest] = res.data.threads;
      setChatThreadId((current) => current ?? latest.thread.id);
      setMessages((current) => {
        if (current.length > 0) return current;
        return latest.messages.map(threadMessageFromSummary);
      });
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

  // Hide while modal surfaces are open so the dock does not compete with
  // dialogs or drawers. Keep it mounted so CSS can animate the exit.
  useEffect(() => {
    const update = () => {
      const open = document.querySelectorAll(
        '[role="dialog"][data-state="open"], [role="alertdialog"][data-state="open"]',
      );
      let blocking = false;
      open.forEach((el) => {
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
        const res = await commandBarService.chatTurn(workspace.id, {
          thread_id: chatThreadId ?? undefined,
          text,
          page_context: pageContext,
        });
        if (res.error || !res.data) {
          toast.error(res.error ?? 'Failed to ask agents');
          return;
        }
        setChatThreadId(res.data.thread.id);
        const userTs = Date.parse(res.data.user_message.created_at) || Date.now();
        const assistantTs = Date.parse(res.data.assistant_message.created_at) || userTs + 1;
        setMessages((prev) => [
          ...prev,
          { kind: 'user', id: res.data!.user_message.id, text, ts: userTs },
          {
            kind: 'assistant',
            id: res.data!.assistant_message.id,
            text: res.data!.assistant_message.content,
            ts: assistantTs,
            proposal: res.data!.proposal ?? res.data!.assistant_message.proposal,
          },
        ]);
        const proposal = res.data.proposal ?? res.data.assistant_message.proposal;
        if (proposal?.type === 'run_plan' && proposal.plan) {
          setIntentResult({
            status: 'plan',
            plan: proposal.plan,
            rationale: res.data.assistant_message.content,
          });
        } else if (proposal?.type === 'no_match') {
          setIntentResult({
            status: 'no_matching_agent',
            reason: proposal.reason ?? res.data.assistant_message.content,
            suggestions: proposal.suggestions,
          });
        } else {
          setIntentResult(null);
        }
      } finally {
        setParsing(false);
      }
    },
    [chatThreadId, pageContext, value, viewMode, setViewMode, workspace?.id],
  );

  const confirmPlan = useCallback(async () => {
    if (!workspace?.id || !pageContext || !intentResult || intentResult.status !== 'plan') return;
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
            createdAt: res.data.runs[0]?.created_at,
            updatedAt: res.data.runs[0]?.updated_at ?? res.data.runs[0]?.created_at,
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

  const resumePlan = useCallback(
    async (plan: CommandBarRunPlan) => {
      if (!workspace?.id) return;
      setBusyPlanId(plan.id);
      try {
        const res = await commandBarService.resumePlan(workspace.id, plan.id);
        if (res.error || !res.data) {
          toast.error(res.error ?? 'Failed to resume plan');
          return;
        }
        const resumedRuns = res.data.runs ?? (res.data.run ? [res.data.run] : []);
        updatePlan(res.data.plan, resumedRuns);
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
      if (action === 'resume') {
        if (target.kind === 'plan') void resumePlan(target.plan);
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
      resumePlan,
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
    setChatThreadId(null);
    setSessionPlanIds(new Set());
    setSessionRunIds(new Set());
    setIntentResult(null);
    setValue('');
    setViewMode('conversation');
    setAutoExpandFirstPaused(false);
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
  const dockVisibilityClass = hiddenByModal
    ? 'translate-y-4 opacity-0'
    : 'translate-y-0 opacity-100';
  const dockInteractionClass = hiddenByModal ? 'pointer-events-none' : 'pointer-events-auto';

  if (collapsed) {
    if (typeof document === 'undefined') return null;
    return createPortal(
      <div
        data-helpin-dock="true"
        aria-hidden={hiddenByModal}
        className={cn(
          'pointer-events-none fixed inset-x-0 bottom-8 z-[60] flex justify-center transition-[opacity,transform] duration-200 ease-out',
          dockVisibilityClass,
        )}
      >
        <button
          type="button"
          onClick={() => setCollapsed(false)}
          className={cn(
            'group inline-flex items-center gap-2.5 rounded-full border border-border/70 bg-background/95 px-4 py-2.5 text-sm font-medium text-muted-foreground shadow-[0_1px_2px_rgba(15,23,42,0.05),0_8px_24px_-8px_rgba(15,23,42,0.22)] backdrop-blur transition hover:border-foreground/30 hover:bg-background hover:text-foreground',
            dockInteractionClass,
          )}
        >
          <AiMagicIcon className="h-4 w-4" />
          Ask agents
          {runningCount > 0 ? (
            <span
              className="inline-flex items-center gap-1 rounded-full bg-orange-500/10 px-2 py-0.5 text-[11px] font-medium text-orange-700 dark:text-orange-300"
              title={`${runningCount} running`}
            >
              <Loading01Icon className="h-3 w-3 animate-spin" />
              {runningCount}
            </span>
          ) : null}
          {awaitingCount > 0 ? (
            <span
              className="inline-flex items-center gap-1 rounded-full bg-amber-500/10 px-2 py-0.5 text-[11px] font-medium text-amber-700 dark:text-amber-300"
              title={`${awaitingCount} waiting on you`}
            >
              <PauseIcon className="h-3 w-3" />
              {awaitingCount}
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
      aria-hidden={hiddenByModal}
      className={cn(
        'pointer-events-none fixed inset-x-0 bottom-6 z-[60] flex justify-center px-4 transition-[opacity,transform] duration-200 ease-out',
        dockVisibilityClass,
      )}
    >
      <div
        className={cn(
          'flex w-full max-w-2xl flex-col rounded-2xl border border-border/70 bg-background/95 shadow-[0_1px_2px_rgba(15,23,42,0.06),0_8px_24px_-12px_rgba(15,23,42,0.18),0_24px_64px_-28px_rgba(15,23,42,0.28)] ring-1 ring-black/[0.02] backdrop-blur transition-shadow focus-within:shadow-[0_1px_2px_rgba(15,23,42,0.06),0_12px_32px_-12px_rgba(15,23,42,0.22),0_32px_80px_-32px_rgba(15,23,42,0.34)] dark:ring-white/[0.04] animate-in fade-in zoom-in-95 slide-in-from-bottom-2 duration-200 ease-out',
          dockInteractionClass,
        )}
      >
        <div className="border-b border-border/60">
          <DockHeader
            mode={viewMode}
            runningCount={runningCount}
            awaitingCount={awaitingCount}
            onSwapMode={() => setViewMode(viewMode === 'list' ? 'conversation' : 'list')}
            onNew={onNew}
            onClose={() => setCollapsed(true)}
          />
        </div>

        {viewMode === 'list' ? (
          <div
            // Keyed on viewMode so the cross-fade fires when the user toggles
            // between the list and the conversation. Tailwind's animate-in
            // utilities give us a quick, contained motion.
            key="list-view"
            ref={responseRef}
            className="max-h-[60vh] overflow-y-auto animate-in fade-in slide-in-from-top-1 duration-150 ease-out"
          >
            <RunListView
              plans={plans}
              standaloneRuns={standaloneRuns}
              runsById={runsById}
              filter={listFilter}
              busyPlanId={busyPlanId}
              busyRunId={busyRunId}
              onSelect={onListSelect}
              onResumePlan={(p) => void resumePlan(p)}
              onCancelPlan={(p) => void cancelPlan(p.id)}
              onRetryPlan={(p) => void retryPlan(p)}
              onRetryRun={retryRun}
            />
          </div>
        ) : hasResponseArea ? (
          <div
            key="conversation-view"
            ref={responseRef}
            className="flex max-h-[60vh] flex-col gap-3 overflow-y-auto px-3.5 py-3 animate-in fade-in slide-in-from-bottom-1 duration-150 ease-out"
          >
            {timeline.map((item) => {
              if (item.kind === 'msg') {
                return item.msg.kind === 'user' ? (
                  <div key={item.id} className="flex justify-end">
                    <div className="max-w-[80%] rounded-2xl rounded-tr-sm bg-primary/10 px-3 py-1.5 text-sm text-foreground">
                      {item.msg.text}
                    </div>
                  </div>
                ) : (
                  <AssistantMessageBlock
                    key={item.id}
                    message={item.msg}
                    busy={busyRunId === item.msg.id}
                    onCreateAgent={async (message, mode) => {
                      setBusyRunId(message.id);
                      try {
                        const res = await commandBarService.confirmChatCreateAgent(workspace.id, message.id);
                        if (res.error || !res.data) {
                          toast.error(res.error ?? 'Failed to create agent');
                          return;
                        }
                        toast.success(mode === 'create_agent_and_run' ? 'Agent created and run started' : 'Agent created');
                        if (res.data.run) {
                          addRuns([res.data.run]);
                          setSessionRunIds((prev) => {
                            const next = new Set(prev);
                            next.add(res.data!.run!.id);
                            return next;
                          });
                        }
                      } finally {
                        setBusyRunId(null);
                      }
                    }}
                  />
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
                    // One-shot / single-agent plans start expanded so the agent's
                    // output renders inline in the bar instead of behind a sheet.
                    defaultOpen={item.plan.steps.length === 1}
                    onAction={(a) => handleStripAction(a, { kind: 'plan', plan: item.plan })}
                    onOpenRun={openRunDrawer}
                    resultSlot={summary ? <InlineResultCard>{summary}</InlineResultCard> : null}
                  />
                );
              }
              const summary = outputSummaryText(item.run);
              const isCompact = compactRunIds.has(item.run.id);
              const shouldAutoOpen =
                isCompact && item.run.id === firstPausedRunId && autoExpandFirstPaused;
              // Bake the auto-open intent into the key so flipping
              // `autoExpandFirstPaused` (via "New") remounts the card with
              // the new defaultOpen — `useState(initialOpen)` only reads its
              // seed once. The cost is one websocket reconnect on the first
              // paused run, only on "New".
              const key = isCompact
                ? `${item.id}-${shouldAutoOpen ? 'auto-open' : 'auto-closed'}`
                : item.id;
              return (
                <ExecutionStrip
                  key={key}
                  kind="run"
                  workspaceId={workspace.id}
                  run={item.run}
                  busy={busyRunId === item.run.id}
                  compact={isCompact}
                  defaultOpen={shouldAutoOpen}
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
            contextOptions={scopeOptions}
            activeContextKey={activeScopeKey}
            onContextKeyChange={setActiveScopeKey}
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


function AssistantMessageBlock({
  message,
  busy,
  onCreateAgent,
}: {
  message: ThreadMessage;
  busy: boolean;
  onCreateAgent: (message: ThreadMessage, mode: 'create_agent' | 'create_agent_and_run') => Promise<void>;
}) {
  const proposal = message.proposal;
  const showText = message.text && proposal?.type !== 'run_plan';
  return (
    <div className="flex justify-start">
      <div className="max-w-[86%] rounded-2xl rounded-tl-sm border border-border/60 bg-muted/35 px-3 py-2 text-sm text-foreground">
        {showText ? <MarkdownContent content={message.text} className="text-sm leading-relaxed" /> : null}
        {proposal?.type === 'create_agent' || proposal?.type === 'create_agent_and_run' ? (
          <AgentDraftProposalCard
            proposal={proposal}
            busy={busy}
            onConfirm={() => {
              const mode = proposal.type === 'create_agent_and_run' ? 'create_agent_and_run' : 'create_agent';
              void onCreateAgent(message, mode);
            }}
          />
        ) : null}
      </div>
    </div>
  );
}

function AgentDraftProposalCard({
  proposal,
  busy,
  onConfirm,
}: {
  proposal: CommandBarProposal;
  busy: boolean;
  onConfirm: () => void;
}) {
  const draft = proposal.draft;
  if (!draft) return null;
  const targets = draft.allowed_targets?.length ? draft.allowed_targets.join(', ') : 'tasks';
  const tools = draft.allowed_tools?.length ?? 0;
  const action = proposal.type === 'create_agent_and_run' ? 'Create agent & run' : 'Create agent';
  return (
    <div className="mt-2 rounded-md border border-border/70 bg-background/80 p-2.5">
      <div className="flex items-center gap-2">
        <BotIcon className="h-3.5 w-3.5 text-muted-foreground" />
        <span className="truncate text-sm font-medium">{draft.name || 'Custom Agent'}</span>
      </div>
      {draft.role ? (
        <p className="mt-1 line-clamp-3 text-xs leading-snug text-muted-foreground">{draft.role}</p>
      ) : null}
      <div className="mt-2 flex flex-wrap gap-1.5 text-[11px] text-muted-foreground">
        <span className="rounded border border-border/70 px-1.5 py-0.5">{targets}</span>
        <span className="rounded border border-border/70 px-1.5 py-0.5">{tools} tools</span>
        <span className="rounded border border-border/70 px-1.5 py-0.5">{draft.default_invocation_mode}</span>
      </div>
      {proposal.warnings?.length ? (
        <p className="mt-2 text-[11px] text-amber-700 dark:text-amber-300">
          {proposal.warnings[0]}
        </p>
      ) : null}
      <div className="mt-2 flex justify-end">
        <button
          type="button"
          onClick={onConfirm}
          disabled={busy}
          className={cn(
            'inline-flex items-center gap-1.5 rounded-md px-2.5 py-1 text-[11px] font-medium transition',
            busy ? 'cursor-not-allowed bg-muted text-muted-foreground' : 'bg-orange-500 text-white hover:bg-orange-500/90',
          )}
        >
          {busy ? <Loading01Icon className="h-3 w-3 animate-spin" /> : <Tick01Icon className="h-3 w-3" />}
          {busy ? 'Creating...' : action}
        </button>
      </div>
    </div>
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
