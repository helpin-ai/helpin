// @vitest-environment jsdom
import { act } from 'react';
import { createRoot } from 'react-dom/client';
import { afterEach, expect, it, vi } from 'vitest';
import { SupportViewEditDialog } from '../SupportViewEditDialog';
import { useSupportInboxStore } from '@/stores/supportInboxStore';
import { defaultConversationListFiltersForNav } from '@/lib/supportInboxFilters';
import type { SupportInboxView } from '@/lib/pmTypes';
Object.assign(globalThis, { IS_REACT_ACT_ENVIRONMENT: true });
afterEach(() => { document.body.innerHTML = ''; });
const view = { id: 'v1', name: 'Urgent work', is_shared: false, filters: { nav_filter: 'waiting', states: 'waiting_on_customer', priority: 'urgent', search: 'billing', future_field: 'preserved' } } as SupportInboxView;
function setup(slaEnabled = true) {
  const onSave = vi.fn(); const onClose = vi.fn();
  const node = document.createElement('div'); document.body.appendChild(node); const root = createRoot(node);
  act(() => root.render(<SupportViewEditDialog view={view} canManageSettings tags={[]} mailboxes={[]} slaEnabled={slaEnabled} isPending={false} onSave={onSave} onClose={onClose} />));
  const button = (label: string) => [...document.body.querySelectorAll('button')].find(b => b.textContent === label)!;
  const click = (label: string) => act(() => { button(label).click(); });
  return { root, button, click, onSave, onClose };
}
it('edits saved filters locally and submits them with existing search and scope', () => {
  const { root, click, onSave } = setup();
  click('High'); expect(onSave).not.toHaveBeenCalled(); click('Save');
  expect(onSave).toHaveBeenCalledWith(expect.objectContaining({ name: 'Urgent work', filters: expect.objectContaining({ nav_filter: 'waiting', priority: 'urgent,high', search: 'billing', future_field: 'preserved' }) }));
  expect(onSave).toHaveBeenCalledWith(expect.objectContaining({ is_shared: false }));
  expect(view.filters.priority).toBe('urgent'); act(() => root.unmount());
});
it('cancels without saving a draft', () => {
  const { root, click, onSave, onClose } = setup(); click('High'); click('Cancel');
  expect(onSave).not.toHaveBeenCalled(); expect(onClose).toHaveBeenCalled(); act(() => root.unmount());
});
it.each([true, false])('uses settings for SLA availability: %s', (enabled) => {
  const { root, button } = setup(enabled);
  expect(button('Breached only').disabled).toBe(!enabled); expect(button('Next SLA due').disabled).toBe(!enabled);
  act(() => root.unmount());
});

it('creates a shared view from shared inbox defaults without inheriting current filters', () => {
  const original = useSupportInboxStore.getState();
  const currentFilters = { ...defaultConversationListFiltersForNav('resolved'), priorities: ['low'] as const };
  useSupportInboxStore.setState({ navFilter: 'resolved', searchQuery: 'old search', selectedMailboxId: 'another-inbox', conversationListFilters: { ...currentFilters, priorities: [...currentFilters.priorities] } });
  const onSave = vi.fn(); const root = createRoot(document.body.appendChild(document.createElement('div')));
  act(() => root.render(<SupportViewEditDialog canManageSettings tags={[]} mailboxes={[]} slaEnabled isPending={false} onSave={onSave} onClose={vi.fn()} />));
  const button = (label: string) => [...document.body.querySelectorAll('button')].find(b => b.textContent === label)!;
  for (const label of ['Open', 'Waiting', 'AI handoff', 'Assigned to me', 'Mentioned me', 'Opened by me', 'Unassigned', 'Assigned to others', 'Main inbox', 'Newest']) {
    expect(button(label).getAttribute('aria-pressed')).toBe('true');
  }
  for (const label of ['High', 'Urgent', 'Breached only']) expect(button(label).getAttribute('aria-pressed')).toBe('false');
  const name = document.body.querySelector('input')!;
  expect(name.value).toBe(''); expect(button('Create view').disabled).toBe(true);
  act(() => {
    Object.getOwnPropertyDescriptor(HTMLInputElement.prototype, 'value')!.set!.call(name, 'New queue');
    name.dispatchEvent(new Event('input', { bubbles: true }));
  });
  act(() => button('Create view').click());
  expect(onSave).toHaveBeenCalledWith({ name: 'New queue', is_shared: true, filters: { nav_filter: 'inbox', states: 'open,waiting_on_customer' } });
  expect(useSupportInboxStore.getState()).toMatchObject({ navFilter: 'resolved', searchQuery: 'old search', selectedMailboxId: 'another-inbox', conversationListFilters: currentFilters });
  useSupportInboxStore.setState(original);
  act(() => root.unmount());
});
