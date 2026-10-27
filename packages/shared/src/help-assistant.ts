// Help assistant conversation shared by the public help center and the
// customer portal. It holds the conversation, sends questions, polls the
// session while an answer is written, and falls back to a one-shot answer
// when the assistant is busy. It has no dependencies, so both apps (and the
// help center's standalone image) can use it; each app adds a thin hook.

/** An article an assistant answer is based on. */
export interface AssistantCitation {
  article_id: string;
  title: string;
  url?: string;
  portal_article_id?: string;
}

export type AssistantMessageStatus = 'pending' | 'answered' | 'handed_off' | 'failed';

/** A question or answer in a help assistant conversation. */
export interface AssistantMessage {
  id: string;
  role: 'user' | 'assistant';
  content: string;
  status: AssistantMessageStatus;
  progress_label?: string;
  citations: AssistantCitation[];
  helpful?: boolean;
  created_at: string;
}

export interface AssistantSessionView {
  id: string;
  messages: AssistantMessage[];
  turns_left: number;
}

export interface AssistantTurn {
  session_id: string;
  question_id: string;
  answer_id: string;
  turns_left: number;
}

/** The result shape of both apps' API clients. */
export interface AssistantResult<T> {
  data: T | null;
  error: string | null;
  status: number;
}

/** How a conversation reaches the server and the browser. */
export interface AssistantTransport {
  visitorId: () => string;
  ask: (body: { visitor_id: string; session_id?: string; message: string; page_article_id?: string }) => Promise<AssistantResult<AssistantTurn>>;
  session: (sessionId: string, visitorId: string) => Promise<AssistantResult<AssistantSessionView>>;
  feedback: (messageId: string, body: { visitor_id: string; session_id: string; helpful: boolean }) => Promise<unknown>;
  /** Answers one question another way when the assistant cannot take it. */
  fallback?: (question: string) => Promise<{ content: string; citations: AssistantCitation[] } | null>;
  loadSessionId?: () => string | null;
  saveSessionId?: (sessionId: string | null) => void;
}

export interface AssistantConversationState {
  messages: AssistantMessage[];
  /** Answers that arrived while the conversation was open; reveal them gradually. */
  freshAnswerIds: ReadonlySet<string>;
  restoring: boolean;
  busy: boolean;
  notice: string | null;
  turnsLeft: number | null;
}

export const ASSISTANT_PENDING_LABEL = 'Searching the help articles…';
export const ASSISTANT_HANDOFF_TEXT = 'I could not find this in the help articles. Contact support and the team will help you.';
const POLL_MS = 1000;

let localSequence = 0;

/**
 * AssistantConversation is one help assistant conversation. Subscribe to it
 * (it works with React's useSyncExternalStore) and call dispose when done.
 */
export class AssistantConversation {
  private state: AssistantConversationState = { messages: [], freshAnswerIds: new Set(), restoring: false, busy: false, notice: null, turnsLeft: null };
  private listeners = new Set<() => void>();
  private sessionId: string | null = null;
  private generation = 0;
  private pollTimer: ReturnType<typeof setTimeout> | undefined;
  private lastQuestion: { text: string; pageArticleId?: string } | null = null;
  private restored = false;
  private readonly transport: AssistantTransport;
  private readonly pollMs: number;

  constructor(transport: AssistantTransport, pollMs = POLL_MS) {
    this.transport = transport;
    this.pollMs = pollMs;
  }

  subscribe = (listener: () => void) => {
    this.listeners.add(listener);
    return () => {
      this.listeners.delete(listener);
    };
  };

  getSnapshot = () => this.state;

  /** restore reloads the conversation kept for this browser tab, once. */
  async restore() {
    if (this.restored) return;
    this.restored = true;
    const stored = this.transport.loadSessionId?.();
    if (!stored) return;
    const generation = this.generation;
    this.update({ restoring: true });
    const result = await this.transport.session(stored, this.transport.visitorId());
    if (generation !== this.generation) return;
    if (!result.data) {
      this.transport.saveSessionId?.(null);
      this.update({ restoring: false });
      return;
    }
    this.sessionId = result.data.id;
    this.update({ restoring: false, messages: result.data.messages, turnsLeft: result.data.turns_left });
    this.schedulePoll();
  }

  /** send asks a question, optionally about the article being read. */
  send(raw: string, pageArticleId?: string) {
    const text = raw.trim();
    if (!text || this.state.busy) return;
    this.lastQuestion = { text, pageArticleId };
    const now = new Date().toISOString();
    const question: AssistantMessage = { id: localId('question'), role: 'user', content: text, status: 'answered', citations: [], created_at: now };
    const answer: AssistantMessage = {
      id: localId('answer'),
      role: 'assistant',
      content: '',
      status: 'pending',
      progress_label: ASSISTANT_PENDING_LABEL,
      citations: [],
      created_at: now,
    };
    this.update({ notice: null, messages: [...this.state.messages, question, answer] });
    void this.ask(text, pageArticleId, question.id, answer.id, this.sessionId, this.generation);
  }

  /** retry asks the last question again after a failed answer. */
  retry() {
    const last = this.lastQuestion;
    if (!last || this.state.busy) return;
    const messages = this.state.messages;
    const failed = messages[messages.length - 1];
    if (failed?.status === 'failed') this.update({ messages: messages.slice(0, -2) });
    this.send(last.text, last.pageArticleId);
  }

  feedback(messageId: string, helpful: boolean) {
    this.update({ messages: this.state.messages.map((message) => (message.id === messageId ? { ...message, helpful } : message)) });
    if (!this.sessionId || isLocalId(messageId)) return;
    void this.transport.feedback(messageId, { visitor_id: this.transport.visitorId(), session_id: this.sessionId, helpful });
  }

  /** reset starts a new conversation. */
  reset() {
    this.generation++;
    this.stopPolling();
    this.sessionId = null;
    this.lastQuestion = null;
    this.transport.saveSessionId?.(null);
    this.update({ messages: [], freshAnswerIds: new Set(), restoring: false, notice: null, turnsLeft: null });
  }

  dispose() {
    this.generation++;
    this.stopPolling();
    this.listeners.clear();
  }

  private async ask(
    text: string,
    pageArticleId: string | undefined,
    questionId: string,
    answerId: string,
    sessionId: string | null,
    generation: number,
  ): Promise<void> {
    const result = await this.transport.ask({
      visitor_id: this.transport.visitorId(),
      session_id: sessionId ?? undefined,
      message: text,
      page_article_id: pageArticleId,
    });
    if (generation !== this.generation) return;
    if (result.data) {
      const turn = result.data;
      this.sessionId = turn.session_id;
      this.transport.saveSessionId?.(turn.session_id);
      this.update({
        turnsLeft: turn.turns_left,
        messages: this.state.messages.map((message) =>
          message.id === questionId ? { ...message, id: turn.question_id } : message.id === answerId ? { ...message, id: turn.answer_id } : message,
        ),
      });
      this.schedulePoll();
      return;
    }
    if (result.status === 404 && sessionId) {
      // The kept conversation is gone, for example after signing in: start over.
      this.sessionId = null;
      this.transport.saveSessionId?.(null);
      return this.ask(text, pageArticleId, questionId, answerId, null, generation);
    }
    if (result.status === 409) {
      this.update({
        messages: this.state.messages.filter((message) => message.id !== questionId && message.id !== answerId),
        notice: result.error?.includes('full') ? 'This conversation is full. Start a new chat to keep asking.' : 'Wait for the current answer to finish.',
      });
      return;
    }
    if ((result.status === 429 || result.status === 503) && this.transport.fallback) {
      this.update({
        notice:
          result.status === 429
            ? 'The assistant has reached its limit for now, so this is a quick answer.'
            : 'The assistant is busy, so this is a quick answer.',
      });
      const fallback = await this.transport.fallback(text).catch(() => null);
      if (generation !== this.generation) return;
      this.replace(
        answerId,
        fallback
          ? { status: 'answered', content: fallback.content, citations: fallback.citations, progress_label: undefined }
          : { status: 'handed_off', content: ASSISTANT_HANDOFF_TEXT, citations: [], progress_label: undefined },
      );
      if (fallback) this.update({ freshAnswerIds: new Set([...this.state.freshAnswerIds, answerId]) });
      return;
    }
    this.replace(answerId, { status: 'failed', progress_label: undefined });
  }

  private schedulePoll() {
    this.stopPolling();
    if (!this.sessionId || !this.state.busy) return;
    const generation = this.generation;
    this.pollTimer = setTimeout(() => void this.poll(generation), this.pollMs);
  }

  private async poll(generation: number) {
    const sessionId = this.sessionId;
    if (!sessionId) return;
    const result = await this.transport.session(sessionId, this.transport.visitorId());
    if (generation !== this.generation) return;
    if (result.data) {
      const previous = this.state.messages;
      const wasPending = new Set(previous.filter((message) => message.status === 'pending').map((message) => message.id));
      const arrived = result.data.messages.filter(
        (message) => message.role === 'assistant' && message.status !== 'pending' && (wasPending.has(message.id) || !previous.some((p) => p.id === message.id)),
      );
      this.update({
        messages: result.data.messages,
        turnsLeft: result.data.turns_left,
        freshAnswerIds: arrived.length ? new Set([...this.state.freshAnswerIds, ...arrived.map((message) => message.id)]) : this.state.freshAnswerIds,
      });
    }
    this.schedulePoll();
  }

  private stopPolling() {
    if (this.pollTimer !== undefined) clearTimeout(this.pollTimer);
    this.pollTimer = undefined;
  }

  private replace(messageId: string, patch: Partial<AssistantMessage>) {
    this.update({ messages: this.state.messages.map((message) => (message.id === messageId ? { ...message, ...patch } : message)) });
  }

  private update(patch: Partial<AssistantConversationState>) {
    const next = { ...this.state, ...patch };
    next.busy = next.messages.some((message) => message.status === 'pending');
    this.state = next;
    for (const listener of this.listeners) listener();
  }
}

function localId(prefix: string) {
  return `${prefix}-local-${++localSequence}`;
}

function isLocalId(id: string) {
  return id.includes('-local-');
}
