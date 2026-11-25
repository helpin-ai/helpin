import { api, API_BASE } from '../api';

export type QuickstartAIMode = 'answer' | 'draft';

export type SupportQuickstartDetection = {
  website_url: string;
  company_name: string;
  /** False when the site couldn't be read; setup still works with less filled in. */
  website_readable: boolean;
  logo_url?: string;
  favicon_url?: string;
  brand_color?: string;
  /**
   * `platform` names what runs the help center, such as `intercom` or `zendesk`; its articles are copied.
   * A `website` help center can't be copied and is read by the AI like the website.
   */
  help_center?: { url: string; name?: string; platform: string };
};

export type SupportQuickstartStatus = {
  started: boolean;
  /** `current` is the article copied most recently, while the copy runs. */
  articles?: { status: 'importing' | 'done' | 'failed'; imported: number; total: number; failed: number; current?: string };
  sources: { name: string; url: string; status: string; indexed_pages: number }[];
  /** True once the AI has indexed knowledge to answer from. */
  agent_ready: boolean;
  /** True when nothing is still importing or crawling. */
  done: boolean;
  ai_mode: QuickstartAIMode | 'off';
};

/** One public article or page. */
export type PublicArticleRef = { title: string; url: string };

/** What a public help center holds, when its articles could be listed. */
export type HelpCenterCounts = {
  /** What runs the help center, such as `intercom` or `zendesk`. */
  platform?: string;
  articles: number;
  collections: number;
  top_collections: { name: string; articles: number }[];
};

/** The support agent answering one question from the company's public pages, before signup. */
export type SupportPreviewAnswer = {
  /** False when the server has no AI to answer with. */
  available: boolean;
  question: string;
  /** False when the pages read don't cover the question. */
  can_answer: boolean;
  answer?: string;
  sources: PublicArticleRef[];
};

/** The agent's work on a company's public pages. */
export type SupportPreview = {
  /** False when the server has no AI to answer with. */
  available: boolean;
  answers: SupportPreviewAnswer[];
  /** Common customer questions no article title covers. */
  gaps: string[];
};

export type SupportPreviewStep = 'site' | 'help_center' | 'articles' | 'answers';

/** One message of the preview as it runs: log lines while a step works, then its result. */
export type SupportPreviewEvent =
  | { type: 'log'; step: SupportPreviewStep; text: string }
  /** Without a step, the whole preview stopped. */
  | { type: 'failed'; step?: SupportPreviewStep; text: string }
  | { type: 'site'; site: SupportQuickstartDetection }
  | { type: 'help_center'; help_center_url?: string; help_center?: HelpCenterCounts }
  | { type: 'articles'; articles?: PublicArticleRef[] }
  | { type: 'preview'; preview: SupportPreview };

/**
 * Public: runs the preview for a website and calls `onEvent` for each step as
 * the server reports it. Rejects when the address is refused or the
 * connection fails; aborting `signal` stops it.
 */
async function streamPreview(
  data: { website_url: string; help_center_url?: string },
  onEvent: (event: SupportPreviewEvent) => void,
  signal: AbortSignal,
): Promise<void> {
  const response = await fetch(`${API_BASE}/onboarding/support/preview`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json', Accept: 'text/event-stream' },
    body: JSON.stringify(data),
    signal,
  });
  if (!response.ok || !response.body) {
    const payload = await response.json().catch(() => null) as { error?: string } | null;
    throw new Error(response.status === 429
      ? 'You’ve looked at a lot of sites from this network. Try again later.'
      : payload?.error ?? 'We couldn’t read that site. Try again.');
  }
  const reader = response.body.pipeThrough(new TextDecoderStream()).getReader();
  let buffer = '';
  for (;;) {
    const { value, done } = await reader.read();
    if (done) return;
    buffer += value;
    // Events are separated by a blank line, each as "data: <json>".
    for (let end = buffer.indexOf('\n\n'); end >= 0; end = buffer.indexOf('\n\n')) {
      const frame = buffer.slice(0, end);
      buffer = buffer.slice(end + 2);
      if (frame.startsWith('data: ')) onEvent(JSON.parse(frame.slice(6)) as SupportPreviewEvent);
    }
  }
}

export const supportQuickstartService = {
  streamPreview,
  /** Public: the agent answers one question a visitor typed. */
  previewAnswer: (data: { website_url: string; help_center_url?: string; question: string }) =>
    api.post<SupportPreviewAnswer>('/onboarding/support/preview-answer', data),
  start: (workspaceId: string, data: { website_url: string; company_name: string; help_center_url?: string; ai_mode: QuickstartAIMode }) =>
    api.post<SupportQuickstartStatus>(`/workspaces/${workspaceId}/support/quickstart`, data),
  status: (workspaceId: string) =>
    api.get<SupportQuickstartStatus>(`/workspaces/${workspaceId}/support/quickstart`),
};
