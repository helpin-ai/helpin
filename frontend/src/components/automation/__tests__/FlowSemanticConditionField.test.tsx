// @vitest-environment jsdom
import React, { act } from 'react';
import { createRoot, type Root } from 'react-dom/client';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { FlowSemanticConditionField, flowConditionOutcomeLabel } from '../FlowSemanticConditionField';

(globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true;
let container: HTMLDivElement;
let root: Root;
afterEach(() => { act(() => root?.unmount()); container?.remove(); });
function render(props: React.ComponentProps<typeof FlowSemanticConditionField>) {
 container = document.createElement('div'); document.body.appendChild(container);
 root = createRoot(container); act(() => root.render(<FlowSemanticConditionField availability={{ available: true }} {...props} />));
 return container.querySelector('input')!;
}
describe('Flow semantic condition', () => {
 it('labels the field, preserves edits and explains skip behavior', () => {
  const changed = vi.fn(); const input = render({ value: 'Customer-facing bug', scheduled: false, onChange: changed });
  expect(container.querySelector('label')?.htmlFor).toBe(input.id);
  expect(input.value).toBe('Customer-facing bug'); expect(input.disabled).toBe(false);
  input.focus(); expect(document.activeElement).toBe(input);
  act(() => {
   Object.getOwnPropertyDescriptor(HTMLInputElement.prototype, 'value')!.set!.call(input, 'Authentication issue');
   input.dispatchEvent(new Event('input', { bubbles: true }));
  });
  expect(changed).toHaveBeenCalledWith('Authentication issue');
  expect(container.textContent).toContain('Uncertain or unavailable checks skip the action');
 });
 it('allows removing an incompatible condition after switching to a schedule', () => {
  const input = render({ value: 'Customer-facing bug', scheduled: true, onChange: vi.fn() });
  expect(input.disabled).toBe(false); expect(input.getAttribute('aria-invalid')).toBe('true');
  expect(container.querySelector('[role="alert"]')?.textContent).toContain('Remove this condition');
 });
 it('disables conditions for an empty schedule and read-only forms', () => {
  const input = render({ value: '', scheduled: true, onChange: vi.fn() }); expect(input.disabled).toBe(true);
  act(() => root.render(<FlowSemanticConditionField value="Saved" scheduled={false} disabled onChange={vi.fn()} />));
  expect(container.querySelector('input')!.disabled).toBe(true);
 });
 it('distinguishes a matching condition from completed action', () => {
  expect(flowConditionOutcomeLabel('matched')).toContain('action may proceed');
  for (const status of ['no_match', 'uncertain', 'unavailable', 'shadow', 'input_limit', 'stale']) {
   expect(flowConditionOutcomeLabel(status)).toContain('action skipped');
  }
 });
});

describe('Jev availability', () => {
 it.each([
  [undefined, 'could not be confirmed'],
  [{ available: false, reason: 'not_configured' } as const, 'configure Jev'],
  [{ available: false, reason: 'shadow' } as const, 'evaluation mode'],
  [{ available: false, reason: 'disabled' } as const, 'disabled by your administrator'],
  [{ available: false, reason: 'workspace_disabled' } as const, 'not enabled for this workspace'],
 ])('disables unavailable new conditions (%j)', (availability, message) => {
  const changed = vi.fn();
  const input = render({ value: '', scheduled: false, availability, onChange: changed });
  expect(input.disabled).toBe(true);
  expect(container.textContent).toContain(message);
  expect(changed).not.toHaveBeenCalled();
 });
 it('preserves a saved guard and permits explicit keyboard-accessible removal', () => {
  const changed = vi.fn();
  const input = render({ value: 'Saved guard', scheduled: false, availability: { available: false, reason: 'not_configured' }, onChange: changed });
  expect(input.value).toBe('Saved guard'); expect(input.disabled).toBe(true);
  expect(changed).not.toHaveBeenCalled();
  expect(container.textContent).toContain('action will be skipped');
  const remove = container.querySelector('button')!;
  remove.focus(); expect(document.activeElement).toBe(remove);
  act(() => remove.click()); expect(changed).toHaveBeenCalledWith('');
 });
 it('does not permit removing a condition from a read-only form', () => {
  const changed = vi.fn();
  render({ value: 'Saved guard', scheduled: false, availability: undefined, disabled: true, onChange: changed });
  const remove = container.querySelector('button')!;
  expect(remove.disabled).toBe(true);
  act(() => remove.click()); expect(changed).not.toHaveBeenCalled();
 });
 it('disables editing while checking availability, even with a cached response', () => {
  const input = render({ value: '', scheduled: false, loading: true, availability: { available: true }, onChange: vi.fn() });
  expect(input.disabled).toBe(true); expect(container.textContent).toContain('Checking semantic condition availability');
 });
});
