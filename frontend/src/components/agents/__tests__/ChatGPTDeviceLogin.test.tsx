// @vitest-environment jsdom
import { act } from 'react';
import { createRoot, type Root } from 'react-dom/client';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { afterEach, beforeEach, expect, it, vi } from 'vitest';
import { ChatGPTDeviceLogin, useChatGPTDeviceLogin } from '../ChatGPTDeviceLogin';
import { aiConnectionService, type AIConnectionLogin } from '@/lib/services/aiConnectionService';

vi.mock('@/lib/services/aiConnectionService', () => ({
  aiConnectionService: { poll: vi.fn() },
}));
vi.mock('sonner', () => ({ toast: { success: vi.fn(), error: vi.fn() } }));
(globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true;

let root: Root;
let client: QueryClient;

function pendingLogin(overrides: Partial<AIConnectionLogin> = {}): AIConnectionLogin {
  return {
    connection: {
      id: 'c1',
      scope: 'personal',
      user_id: 'u1',
      name: 'My ChatGPT',
      provider: 'openai_chatgpt',
      status: 'pending',
    },
    user_code: 'ABCD-1234',
    verification_url: 'https://example.test/device',
    interval_seconds: 5,
    expires_at: new Date(Date.now() + 300_000).toISOString(),
    ...overrides,
  };
}

function Harness({ login, onLogin }: { login: AIConnectionLogin; onLogin: (next: AIConnectionLogin) => void }) {
  const device = useChatGPTDeviceLogin({ workspaceId: 'ws', login, onLogin });
  return (
    <ChatGPTDeviceLogin
      login={login}
      phase={device.phase}
      failure={device.failure}
      countdown={device.countdown}
      isChecking={device.isChecking}
      onCheckNow={() => void device.checkNow()}
      onStartAgain={() => {}}
    />
  );
}

beforeEach(() => {
  vi.useFakeTimers();
  const container = document.createElement('div');
  document.body.append(container);
  root = createRoot(container);
  client = new QueryClient({ defaultOptions: { queries: { retry: false }, mutations: { retry: false } } });
});

afterEach(() => {
  act(() => root.unmount());
  client.clear();
  document.body.innerHTML = '';
  vi.useRealTimers();
  vi.clearAllMocks();
});

async function render(node: React.ReactNode) {
  await act(async () => root.render(<QueryClientProvider client={client}>{node}</QueryClientProvider>));
}

it('polls after the provider interval and reports a connected account', async () => {
  const login = pendingLogin();
  const connected = { ...login, user_code: undefined, connection: { ...login.connection, status: 'connected' as const } };
  vi.mocked(aiConnectionService.poll).mockResolvedValue({ data: connected, error: null });
  let current = login;
  const onLogin = vi.fn((next: AIConnectionLogin) => { current = next; });

  await render(<Harness login={current} onLogin={onLogin} />);
  expect(document.body.textContent).toContain('ABCD-1234');
  expect(aiConnectionService.poll).not.toHaveBeenCalled();

  await act(async () => { await vi.advanceTimersByTimeAsync(5000); });
  expect(aiConnectionService.poll).toHaveBeenCalledWith('ws', 'c1');
  expect(onLogin).toHaveBeenCalledWith(connected);

  await render(<Harness login={current} onLogin={onLogin} />);
  expect(document.body.textContent).toContain('ChatGPT connected');
});

it('stops polling once the code expires', async () => {
  vi.mocked(aiConnectionService.poll).mockResolvedValue({ data: pendingLogin(), error: null });
  const login = pendingLogin({ expires_at: new Date(Date.now() - 1000).toISOString() });
  await render(<Harness login={login} onLogin={() => {}} />);
  await act(async () => { await vi.advanceTimersByTimeAsync(30_000); });
  expect(aiConnectionService.poll).not.toHaveBeenCalled();
  expect(document.body.textContent).toContain('The code expired');
});

it('stops polling when the login block unmounts', async () => {
  vi.mocked(aiConnectionService.poll).mockResolvedValue({ data: pendingLogin(), error: null });
  await render(<Harness login={pendingLogin()} onLogin={() => {}} />);
  await act(async () => root.render(<div />));
  await act(async () => { await vi.advanceTimersByTimeAsync(30_000); });
  expect(aiConnectionService.poll).not.toHaveBeenCalled();
});

it('renders without a countdown when the provider omits an expiry', async () => {
  vi.mocked(aiConnectionService.poll).mockResolvedValue({ data: pendingLogin(), error: null });
  await render(<Harness login={pendingLogin({ expires_at: undefined })} onLogin={() => {}} />);
  expect(document.body.textContent).toContain('The code expires in a few minutes');
  expect(document.body.textContent).not.toContain('Invalid Date');
});

it('surfaces a failed check and offers to try again', async () => {
  vi.mocked(aiConnectionService.poll).mockResolvedValue({ data: null, error: 'device flow rejected' });
  await render(<Harness login={pendingLogin()} onLogin={() => {}} />);
  await act(async () => { await vi.advanceTimersByTimeAsync(5000); });
  expect(document.body.textContent).toContain('device flow rejected');
  expect(document.body.textContent).toContain('Check again');
});
