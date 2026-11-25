import { useEffect, useReducer, useState } from 'react';
import {
  supportQuickstartService,
  type HelpCenterCounts,
  type PublicArticleRef,
  type SupportPreview,
  type SupportPreviewEvent,
  type SupportPreviewStep,
  type SupportQuickstartDetection,
} from '@/lib/services/supportQuickstartService';

/** One run of the public support preview, filled in as the server reports each step. */
export type SupportPreviewRun = {
  website: string;
  /** `stopped` means the connection ended before the preview finished. */
  status: 'running' | 'done' | 'stopped';
  /** Why the run stopped, when it did. */
  error?: string;
  /** What each step is doing right now, oldest line first. */
  logs: Partial<Record<SupportPreviewStep, string[]>>;
  site?: SupportQuickstartDetection;
  /** Set once the help center step finished; `url` is empty when there is none. */
  helpCenter?: { url: string; counts?: HelpCenterCounts };
  /** The articles or pages the agent read. */
  articles?: PublicArticleRef[];
  preview?: SupportPreview;
  /** Why the agent couldn't answer, when it couldn't. */
  answersFailed?: string;
};

type Action =
  | SupportPreviewEvent
  | { type: 'start'; website: string }
  | { type: 'end' }
  | { type: 'stopped'; text: string };

const COULD_NOT_FINISH = 'We couldn’t finish reading your site. Try again.';
// How long one step's result has the screen before the next one's shows.
const RESULT_GAP_MS = 1000;

export function prefersReducedMotion() {
  return typeof window !== 'undefined' && Boolean(window.matchMedia?.('(prefers-reduced-motion: reduce)').matches);
}

/**
 * Passes actions on in order, holding each step's result until the one before
 * it has been on screen for `gap`. Results the server sends together (a site
 * it has seen before) would otherwise all tick at once.
 */
function paced(dispatch: (action: Action) => void, gap: number) {
  const queue: Action[] = [];
  let shownAt = 0;
  let timer: ReturnType<typeof setTimeout> | undefined;
  let closed = false;
  const isResult = (action: Action) => ['site', 'help_center', 'articles', 'preview', 'failed'].includes(action.type);
  const drain = () => {
    timer = undefined;
    while (queue.length > 0) {
      const result = isResult(queue[0]);
      const wait = result ? shownAt + gap - Date.now() : 0;
      if (wait > 0) {
        timer = setTimeout(drain, wait);
        return;
      }
      if (result) shownAt = Date.now();
      dispatch(queue.shift()!);
    }
  };
  return {
    push: (action: Action) => {
      if (closed) return;
      queue.push(action);
      if (!timer) drain();
    },
    cancel: () => {
      closed = true;
      clearTimeout(timer);
    },
  };
}

function reduce(run: SupportPreviewRun, action: Action): SupportPreviewRun {
  switch (action.type) {
    case 'start':
      // The same site read again (a help center was added) keeps its brand on screen.
      return { website: action.website, status: 'running', logs: {}, site: run.website === action.website ? run.site : undefined };
    case 'log':
      return { ...run, logs: { ...run.logs, [action.step]: [...(run.logs[action.step] ?? []), action.text] } };
    case 'site':
      return { ...run, site: action.site };
    case 'help_center':
      return { ...run, helpCenter: { url: action.help_center_url ?? '', counts: action.help_center } };
    case 'articles':
      return { ...run, articles: action.articles ?? [] };
    case 'preview':
      return { ...run, preview: action.preview };
    case 'failed':
      return action.step ? { ...run, answersFailed: action.text } : { ...run, status: 'stopped', error: action.text };
    case 'end':
      if (run.status !== 'running') return run;
      return run.site ? { ...run, status: 'done' } : { ...run, status: 'stopped', error: COULD_NOT_FINISH };
    case 'stopped':
      return { ...run, status: 'stopped', error: action.text };
  }
}

/**
 * Runs the public support preview for a website and returns it as it
 * progresses. It starts again when the website or the added help center
 * address changes, and on `retry`.
 */
export function useSupportPreview(website: string, helpCenterUrl?: string) {
  const [run, dispatch] = useReducer(reduce, { website, status: 'running', logs: {} });
  const [attempt, setAttempt] = useState(0);

  useEffect(() => {
    const controller = new AbortController();
    const steps = paced(dispatch, prefersReducedMotion() ? 0 : RESULT_GAP_MS);
    dispatch({ type: 'start', website });
    supportQuickstartService
      .streamPreview({ website_url: website, help_center_url: helpCenterUrl }, steps.push, controller.signal)
      .then(() => steps.push({ type: 'end' }))
      .catch((error: unknown) => {
        if (controller.signal.aborted) return;
        // fetch reports a lost connection as a TypeError with a browser-specific message.
        const text = error instanceof Error && !(error instanceof TypeError) ? error.message : COULD_NOT_FINISH;
        steps.push({ type: 'stopped', text });
      });
    return () => {
      controller.abort();
      steps.cancel();
    };
  }, [website, helpCenterUrl, attempt]);

  return { run, retry: () => setAttempt((count) => count + 1) };
}
