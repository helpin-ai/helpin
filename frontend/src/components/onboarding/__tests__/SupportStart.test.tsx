// @vitest-environment jsdom

import { act, type ReactNode } from 'react';
import { createRoot, type Root } from 'react-dom/client';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

const state = vi.hoisted(() => ({
  navigate: vi.fn(),
  user: null as { email: string } | null,
  signUp: vi.fn(),
  streamPreview: vi.fn(),
  previewAnswer: vi.fn(),
  mountWidget: vi.fn(),
}));

vi.mock('@tanstack/react-router', () => ({
  useNavigate: () => state.navigate,
  Link: ({ children }: { children: ReactNode }) => <a>{children}</a>,
}));
vi.mock('@/components/layout/PublicPageShell', () => ({
  PublicPageShell: ({ children }: { children: ReactNode }) => <main>{children}</main>,
}));
// The real chat widget draws itself; the tests read what it was given to show.
vi.mock('@helpin-ai/widget-core', () => ({ mountWidget: state.mountWidget, unmountWidget: vi.fn() }));
vi.mock('@/lib/services/supportQuickstartService', () => ({
  supportQuickstartService: { streamPreview: state.streamPreview, previewAnswer: state.previewAnswer },
}));
vi.mock('@/stores/authStore', () => ({
  ensureAuthConfiguration: vi.fn(),
  useAuthStore: (selector: (value: unknown) => unknown) =>
    selector({ user: state.user, signUp: state.signUp, configuration: { signup_mode: 'open' } }),
}));

import { readPendingSupportSetup } from '@/lib/pendingSupportSetup';
import { SupportStart } from '../SupportStart';

(globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true;

let container: HTMLDivElement | null = null;
let root: Root | null = null;

type Emit = (event: Record<string, unknown>) => void;

const site = { website_url: 'https://acme.com', company_name: 'Acme', website_readable: true, brand_color: '#ff5500' };
const refunds = {
  available: true,
  question: 'How do I get a refund?',
  can_answer: true,
  answer: 'Refunds are issued within **5 days**.\n- Open Billing\n- Choose Refund',
  sources: [{ title: 'Refunds', url: 'https://help.acme.com/en/articles/4-refunds' }],
};
const billing = { ...refunds, question: 'How am I billed?', answer: 'Plans are billed monthly.', sources: [{ title: 'Billing basics', url: 'https://help.acme.com/en/articles/3-billing' }] };
const events = {
  site: { type: 'site', site },
  helpCenter: {
    type: 'help_center',
    help_center_url: 'https://help.acme.com/en',
    help_center: { platform: 'intercom', articles: 402, collections: 12, top_collections: [{ name: 'Billing', articles: 30 }] },
  },
  articles: { type: 'articles', articles: [{ title: 'Refunds', url: 'https://help.acme.com/en/articles/4-refunds' }] },
  preview: { type: 'preview', preview: { available: true, answers: [refunds, billing], gaps: ['Do you have a mobile app?'] } },
};

/** The server sends every step and closes the stream. */
function streamAll(...sent: Record<string, unknown>[]) {
  state.streamPreview.mockImplementation(async (_data: unknown, emit: Emit) => sent.forEach(emit));
}

/** The server sends what it is given and keeps the stream open; returns how to send more. */
function streamOpen() {
  let emit: Emit = () => {};
  state.streamPreview.mockImplementation((_data: unknown, onEvent: Emit) => {
    emit = onEvent;
    return new Promise(() => {});
  });
  return (...sent: Record<string, unknown>[]) => act(() => sent.forEach((event) => emit(event)));
}

function render(website?: string) {
  container = document.createElement('div');
  document.body.appendChild(container);
  root = createRoot(container);
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  act(() => root?.render(<QueryClientProvider client={client}><SupportStart website={website} /></QueryClientProvider>));
}

/** Whether the visitor asked their device for less motion; with it, results show at once. */
function reducedMotion(reduced: boolean) {
  window.matchMedia = ((query: string) => ({ matches: reduced && query.includes('reduce') })) as unknown as typeof window.matchMedia;
}

function button(label: string) {
  return Array.from(container?.querySelectorAll('button') ?? []).find((element) => element.textContent?.trim() === label) as HTMLButtonElement | undefined;
}

function type(input: HTMLInputElement, value: string) {
  const setter = Object.getOwnPropertyDescriptor(HTMLInputElement.prototype, 'value')?.set;
  setter?.call(input, value);
  input.dispatchEvent(new Event('input', { bubbles: true }));
}

type WidgetMessage = { role: string; content: string; isStreaming?: boolean; sources?: { title: string }[] };

/** What the chat widget is showing now. */
function widget() {
  return state.mountWidget.mock.lastCall![1] as { messages: WidgetMessage[]; isAIThinking: boolean; onSendMessage: (text: string) => void };
}

function reply() {
  return widget().messages.at(-1)!;
}

async function settle() {
  for (let i = 0; i < 6; i++) {
    await act(async () => { await new Promise((resolve) => setTimeout(resolve, 0)); });
  }
}

beforeEach(() => {
  localStorage.clear();
  reducedMotion(true);
  Element.prototype.scrollIntoView = vi.fn();
  state.navigate.mockReset();
  state.user = null;
  state.signUp.mockReset().mockResolvedValue({ error: null });
  state.streamPreview.mockReset();
  streamAll(events.site, events.helpCenter, events.articles, events.preview);
  state.previewAnswer.mockReset();
  state.mountWidget.mockClear();
});

afterEach(() => {
  vi.useRealTimers();
  act(() => root?.unmount());
  container?.remove();
  container = null;
  root = null;
});

describe('SupportStart', () => {
  it('asks for a website when none is given', () => {
    render();
    act(() => type(container?.querySelector('input') as HTMLInputElement, 'acme.com'));
    act(() => button('Show me')?.click());

    expect(state.navigate).toHaveBeenCalledWith({ to: '/start', search: { website: 'acme.com' } });
  });

  it('shows each step as the server reports it', async () => {
    const send = streamOpen();
    render('acme.com');
    await settle();
    expect(container?.textContent).toContain('Reading acme.com');
    expect(container?.textContent).toContain('While your agent works');

    send({ type: 'log', step: 'site', text: 'Opening acme.com' });
    expect(container?.textContent).toContain('Opening acme.com');

    send(events.site, { type: 'log', step: 'help_center', text: 'Looking for your help center' });
    expect(container?.textContent).toContain('Read acme.com');
    expect((container?.querySelector('input[autocomplete="organization"]') as HTMLInputElement).value).toBe('Acme');
    expect(container?.textContent).toContain('Finding your help center');
    expect(container?.textContent).toContain('Looking for your help center');
    expect(button('Put this on acme.com')).toBeUndefined();

    send(events.helpCenter, { type: 'log', step: 'articles', text: 'Read “Refunds”' });
    expect(container?.textContent).toContain('Found 402 help articles');
    // The platform found at the address is named next to it.
    expect(container?.textContent).toContain('help.acme.com · Intercom · 12 collections');
    expect(container?.textContent).toContain('Reading your articles');
    expect(container?.textContent).toContain('Read “Refunds”');
    // Going live is offered once the help center is known, before the answers arrive.
    expect(button('Put this on acme.com')).toBeDefined();

    send(events.articles, events.preview);
    expect(container?.textContent).toContain('Read 1 article');
    expect(container?.textContent).toContain('Answered 2 customer questions, found 1 without an article');
    expect(container?.textContent).not.toContain('While your agent works');
    expect(state.streamPreview).toHaveBeenCalledWith({ website_url: 'acme.com', help_center_url: undefined }, expect.any(Function), expect.any(AbortSignal));
  });

  it('shows one result at a time, then folds it into its title', async () => {
    reducedMotion(false);
    vi.useFakeTimers();
    render('acme.com');
    const found = () => Array.from(container?.querySelectorAll('button[aria-expanded]') ?? []) as HTMLButtonElement[];
    await act(async () => { await vi.advanceTimersByTimeAsync(50); });
    // Everything arrived together; only the first result is on screen.
    expect(container?.textContent).toContain('Read acme.com');
    expect(container?.textContent).toContain('Finding your help center');
    expect(found().map((step) => step.getAttribute('aria-expanded'))).toEqual(['true']);

    await act(async () => { await vi.advanceTimersByTimeAsync(1000); });
    expect(container?.textContent).toContain('help articles');
    expect(container?.textContent).not.toContain('Answered');
    expect(found().map((step) => step.getAttribute('aria-expanded'))).toEqual(['false', 'true']);
    expect(found()[0].textContent).toBe('Read acme.comAcme');

    await act(async () => { await vi.advanceTimersByTimeAsync(6000); });
    // The numbers count up on the browser's frame clock, which this test doesn't run.
    expect(container?.textContent).toMatch(/Found \d+ help articles/);
    expect(container?.textContent).toMatch(/Answered \d customer questions/);
    expect(found().map((step) => step.getAttribute('aria-expanded'))).toEqual(['false', 'false', 'false']);

    act(() => found()[0].click());
    expect(found()[0].getAttribute('aria-expanded')).toBe('true');
    expect(container?.querySelector('[inert] input[autocomplete="organization"]')).toBeNull();
  });

  it('shows the agent’s answers with their sources in the chat widget, and the hand-off for a question with no article', async () => {
    render('acme.com');
    await settle();

    expect(widget().messages.map((message) => message.role)).toEqual(['customer', 'ai']);
    expect(widget().messages[0].content).toBe('How do I get a refund?');
    expect(reply().content).toBe(refunds.answer);
    expect(reply().sources?.map((source) => source.title)).toEqual(['Refunds']);

    act(() => button('How am I billed?')?.click());
    expect(reply().content).toBe('Plans are billed monthly.');
    expect(reply().sources?.map((source) => source.title)).toEqual(['Billing basics']);

    act(() => button('Do you have a mobile app?')?.click());
    expect(reply().content).toContain('I’d pass this to your team');
    expect(reply().sources).toBeUndefined();
    expect(container?.textContent).toContain('among your 402 article titles');
  });

  it('writes an answer out a few words at a time, then names its source', async () => {
    reducedMotion(false);
    vi.useFakeTimers();
    render('acme.com');
    // Past the paced steps, to the moment the answers arrive.
    await act(async () => { await vi.advanceTimersByTimeAsync(3100); });
    expect(reply()).toMatchObject({ content: '', isStreaming: true });

    await act(async () => { await vi.advanceTimersByTimeAsync(90); });
    expect(reply()).toMatchObject({ content: 'Refunds are', isStreaming: true });
    expect(reply().sources).toBeUndefined();

    await act(async () => { await vi.advanceTimersByTimeAsync(2000); });
    expect(reply()).toMatchObject({ content: refunds.answer, isStreaming: false });
    expect(reply().sources).toHaveLength(1);
  });

  it('answers a question the visitor types into the widget', async () => {
    render('acme.com');
    await settle();
    let answer: (value: unknown) => void = () => {};
    state.previewAnswer.mockReturnValueOnce(new Promise((resolve) => { answer = resolve; }));
    act(() => widget().onSendMessage('Do you ship to Mars?'));
    await settle();

    expect(state.previewAnswer).toHaveBeenLastCalledWith({ website_url: 'https://acme.com', help_center_url: 'https://help.acme.com/en', question: 'Do you ship to Mars?' });
    expect(widget().isAIThinking).toBe(true);
    expect(widget().messages.map((message) => message.content)).toEqual(['Do you ship to Mars?']);

    await act(async () => answer({ data: { available: true, question: 'Do you ship to Mars?', can_answer: false, sources: [] }, error: null }));
    await settle();
    expect(button('Do you ship to Mars?')?.getAttribute('aria-pressed')).toBe('true');
    expect(reply().content).toContain('I’d pass this to your team');

    // Asking something already on the list shows it again without another request.
    act(() => widget().onSendMessage('How am I billed?'));
    expect(state.previewAnswer).toHaveBeenCalledTimes(1);
    expect(reply().content).toBe('Plans are billed monthly.');
  });

  it('saves what was found and asks a visitor for an account in place, without any choice to make', async () => {
    render('acme.com');
    await settle();
    expect(container?.querySelector('input[type="radio"]')).toBeNull();
    act(() => button('Put this on acme.com')?.click());

    expect(readPendingSupportSetup()).toMatchObject({
      companyName: 'Acme',
      aiMode: 'answer',
      helpCenterUrl: 'https://help.acme.com/en',
      detection: { website_url: 'https://acme.com', help_center: { url: 'https://help.acme.com/en', platform: 'intercom' } },
    });
    expect(container?.textContent).toContain('Last step: create your account');

    act(() => type(container?.querySelector('input[autocomplete="name"]') as HTMLInputElement, 'Ana Lee'));
    act(() => type(container?.querySelector('input[type="email"]') as HTMLInputElement, 'ana@acme.com'));
    act(() => type(container?.querySelector('input[type="password"]') as HTMLInputElement, 'correct-horse'));
    await act(async () => { button('Create account and go live')?.click(); });
    await settle();

    expect(state.signUp).toHaveBeenCalledWith('ana@acme.com', 'correct-horse', 'Ana Lee');
    expect(state.navigate).toHaveBeenCalledWith({ to: '/onboarding' });
  });

  it('takes a signed-in person straight to setup', async () => {
    state.user = { email: 'ana@acme.com' };
    render('acme.com');
    await settle();
    act(() => button('Put this on acme.com')?.click());

    expect(state.navigate).toHaveBeenCalledWith({ to: '/onboarding' });
    expect(container?.textContent).not.toContain('Last step: create your account');
  });

  it('reads again with a help center address the visitor adds', async () => {
    streamAll(events.site, { type: 'help_center' }, { type: 'articles', articles: [] }, { type: 'preview', preview: { available: true, answers: [], gaps: [] } });
    render('acme.com');
    await settle();
    expect(container?.textContent).toContain('No help center found');
    expect(container?.textContent).toContain('Your agent is ready to learn');

    act(() => button('Have one? Add its address')?.click());
    act(() => type(container?.querySelector('input[aria-label="Help center address"]') as HTMLInputElement, 'help.acme.com'));
    await act(async () => { button('Use it')?.click(); });
    await settle();

    expect(state.streamPreview).toHaveBeenLastCalledWith({ website_url: 'acme.com', help_center_url: 'help.acme.com' }, expect.any(Function), expect.any(AbortSignal));
  });

  it('keeps going live available when the agent could not answer', async () => {
    streamAll(events.site, events.helpCenter, events.articles, { type: 'failed', step: 'answers', text: 'Your agent couldn’t answer just now.' });
    render('acme.com');
    await settle();

    expect(container?.textContent).toContain('Your agent couldn’t answer just now');
    expect(container?.textContent).toContain('You can still put it live');
    expect(button('Put this on acme.com')).toBeDefined();
  });

  it('leaves out the reading and answering steps when the server has no AI', async () => {
    streamAll(events.site, events.helpCenter, { type: 'preview', preview: { available: false, answers: [], gaps: [] } });
    render('acme.com');
    await settle();

    expect(container?.textContent).toContain('Found 402 help articles');
    expect(container?.textContent).not.toContain('customer questions');
    expect(container?.textContent).not.toContain('your articles');
    expect(button('Put this on acme.com')).toBeDefined();
  });

  it('asks for the website again when it cannot be read at all', async () => {
    state.streamPreview.mockRejectedValue(new Error('Enter a public website address.'));
    render('localhost');
    await settle();

    expect(container?.querySelector('[role="alert"]')?.textContent).toContain('Enter a public website address.');
    expect(button('Show me')).toBeDefined();
  });
});
