// @vitest-environment jsdom
import { act } from 'react';
import { createRoot } from 'react-dom/client';
import { afterEach, expect, it, vi } from 'vitest';
import { SupportViewEditDialog } from '../SupportViewEditDialog';
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
