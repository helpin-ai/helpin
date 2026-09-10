// @vitest-environment jsdom
import React, { act } from 'react';
import { createRoot, type Root } from 'react-dom/client';
import { beforeEach, afterEach, describe, expect, it, vi } from 'vitest';
import { EmailForwardingSetup } from '../EmailForwardingSetup';
import type { SupportEmailRoute } from '@/lib/pmTypes';
(globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true;
let root: Root;
let container: HTMLDivElement;
const route = { id: 'route', workspace_id: 'workspace', inbound_address: 'team@helpin.email' } as SupportEmailRoute;
const send = vi.fn();
const check = vi.fn();
let currentRoute = route;
function render() { act(() => root.render(<EmailForwardingSetup key={currentRoute.id} route={currentRoute} busy={false} confirmationHref={currentRoute.confirmation_received_at ? '/confirmation?team_inbox=team' : null} inboxHref="/inbox" onCopy={vi.fn()} onSendTest={send} onCheck={check} />)); }
function click(text: string) { act(() => Array.from(container.querySelectorAll('button')).find(button => button.textContent === text)!.click()); }
beforeEach(() => { localStorage.clear(); send.mockReset(); check.mockReset(); currentRoute = route; container = document.createElement('div'); document.body.appendChild(container); root = createRoot(container); });
afterEach(() => { act(() => root.unmount()); container.remove(); });
describe('four-step forwarding setup', () => {
  it('persists acknowledgements per inbox and keeps Gmail refresh guidance available', () => {
    render();
    click('I’ve added the address');
    expect(container.textContent).toContain('Step 2 of 4');
    click('I’ve confirmed');
    expect(container.textContent).toContain('refresh the page');
    click('I’ve enabled forwarding');
    expect(container.textContent).toContain('Send test email');
    act(() => root.unmount()); root = createRoot(container); render();
    expect(container.textContent).toContain('Step 4 of 4');
    currentRoute = { ...route, id: 'other-inbox' }; render();
    expect(container.textContent).toContain('Step 1 of 4');
  });
  it('automatically shows received confirmation but does not mark approval complete', () => {
    render(); currentRoute = { ...route, confirmation_received_at: new Date().toISOString() }; render();
    expect(container.textContent).toContain('Step 2 of 4');
    expect(container.querySelector('a')?.getAttribute('href')).toBe('/confirmation?team_inbox=team');
    expect(container.textContent).toContain('I’ve confirmed');
  });
  it('restores a corrected source draft instead of an older server address', () => {
    localStorage.setItem('helpin:forwarding-setup:v1:workspace:route', JSON.stringify({ step: 4, source: 'correct@example.com', provider: 'gmail' }));
    currentRoute = { ...route, source_address: 'old@example.com' }; render();
    expect(container.querySelector('input')?.value).toBe('correct@example.com');
  });
  it('keeps instructions mounted while the outer verification transition runs', () => {
    currentRoute = { ...route, verification_sent_at: new Date().toISOString() }; render();
    currentRoute = { ...currentRoute, forwarding_verified_at: new Date().toISOString() }; render();
    expect(container.querySelector('form')).not.toBeNull();
  });
  it('reports failed status refresh beside the confirmation action', async () => {
    render(); click('I’ve added the address');
    check.mockRejectedValue(new Error('Offline'));
    await act(async () => { click('Check status'); });
    expect(container.querySelector('[role="alert"]')?.textContent).toContain('Could not refresh');
  });
  it('keeps a failed test actionable and shows delayed server tests on reopening', async () => {
    currentRoute = { ...route, source_address: 'support@example.com', verification_sent_at: new Date(Date.now() - 180_000).toISOString() }; render();
    expect(container.textContent).toContain('The test hasn’t arrived yet');
    send.mockRejectedValue(new Error('Mail delivery unavailable'));
    await act(async () => container.querySelector('form')!.dispatchEvent(new Event('submit', { bubbles: true, cancelable: true })));
    expect(send).toHaveBeenCalledWith('route', 'support@example.com');
    expect(container.querySelector('[role="alert"]')?.textContent).toBe('Mail delivery unavailable');
    expect(container.textContent).toContain('Send another test');
  });
});
