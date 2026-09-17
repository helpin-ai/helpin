// @vitest-environment jsdom
import { act, type ButtonHTMLAttributes, type ReactNode } from 'react';
import { createRoot, type Root } from 'react-dom/client';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { CLIAuthorizePage } from './CLIAuthorizePage';
import { cliService } from '@/lib/services/cliService';
import { parseCLIAuthorizationQuery } from '@/lib/cliTypes';

vi.mock('@/hooks/useTitle', () => ({ useTitle: () => {} }));
vi.mock('@/lib/services/cliService', () => ({ cliService: { consentRequest: vi.fn(), authorize: vi.fn() } }));
vi.mock('@/components/design-system/quiet', () => ({
  QuietPageHeader: ({ title }: { title: string }) => <h1>{title}</h1>,
  QuietSection: ({ title, children }: { title: string; children: ReactNode }) => <section><h2>{title}</h2>{children}</section>,
  QuietTextAction: (props: ButtonHTMLAttributes<HTMLButtonElement>) => <button {...props} />,
  QuietPrimaryAction: (props: ButtonHTMLAttributes<HTMLButtonElement>) => <button {...props} />,
  QuietDropdown: ({ selected, options, onSelect }: { selected: string[]; options: { value: string; label: string }[]; onSelect: (value: string) => void }) =>
    <select aria-label="Choose workspace" value={selected[0] ?? ''} onChange={(event) => onSelect(event.target.value)}><option value="">Choose a workspace</option>{options.map((option) => <option key={option.value} value={option.value}>{option.label}</option>)}</select>,
}));
const query = parseCLIAuthorizationQuery({ state: 'test-state', client_id: 'agent-runtime-cli', redirect_uri: 'http://127.0.0.1:34567/callback' });
let root: Root;
let container: HTMLDivElement;
async function render() {
  container = document.createElement('div'); document.body.appendChild(container); root = createRoot(container);
  await act(async () => { root.render(<QueryClientProvider client={new QueryClient({ defaultOptions: { queries: { retry: false } } })}><CLIAuthorizePage query={query} /></QueryClientProvider>); });
  await settle();
}
async function settle() { await act(async () => { await new Promise((resolve) => setTimeout(resolve, 20)); }); }
function button(label: string) { return [...container.querySelectorAll('button')].find((item) => item.textContent === label)!; }
afterEach(async () => { if (root) await act(async () => root.unmount()); container?.remove(); vi.resetAllMocks(); });
describe('CLI browser consent', () => {
  it('requires an explicit workspace choice and reports authorization failures', async () => {
    vi.mocked(cliService.consentRequest).mockResolvedValue({ data: { client_name: 'Agent Runtime CLI', query, workspaces: [{ id: 'ws-1', name: 'Workspace One', role: 'owner' }] }, error: null });
    vi.mocked(cliService.authorize).mockResolvedValue({ data: null, error: 'Permission changed' });
    await render();
    expect(button('Authorize CLI').disabled).toBe(true);
    await act(async () => { const select = container.querySelector('select')!; select.value = 'ws-1'; select.dispatchEvent(new Event('change', { bubbles: true })); });
    expect(button('Authorize CLI').disabled).toBe(false);
    await act(async () => button('Authorize CLI').click()); await settle();
    expect(cliService.authorize).toHaveBeenCalledWith(query, 'ws-1');
    expect(container.querySelector('[role="alert"]')?.textContent).toContain('Authorization could not be completed');
  });
  it('does not allow consent when no workspace is eligible', async () => {
    vi.mocked(cliService.consentRequest).mockResolvedValue({ data: { client_name: 'Agent Runtime CLI', query, workspaces: [] }, error: null });
    await render(); expect(container.textContent).toContain('No eligible workspaces'); expect(button('Authorize CLI').disabled).toBe(true);
  });
  it('does not offer a redirect or authorization on an invalid request', async () => {
    vi.mocked(cliService.consentRequest).mockResolvedValue({ data: null, error: 'Invalid request' });
    await render(); expect(container.querySelector('[role="alert"]')?.textContent).toContain('could not be verified'); expect(container.querySelectorAll('button')).toHaveLength(0);
  });
  it('shows progress before a request is verified', async () => {
    vi.mocked(cliService.consentRequest).mockReturnValue(new Promise(() => {}));
    await render(); expect(container.querySelector('[role="status"]')?.textContent).toContain('Checking'); expect(container.querySelectorAll('button')).toHaveLength(0);
  });
});
