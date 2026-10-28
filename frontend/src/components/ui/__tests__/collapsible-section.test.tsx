// @vitest-environment jsdom
import { act } from 'react';
import { createRoot } from 'react-dom/client';
import { expect, it } from 'vitest';
import { CollapsibleSection } from '../collapsible-section';

(globalThis as unknown as { IS_REACT_ACT_ENVIRONMENT: boolean }).IS_REACT_ACT_ENVIRONMENT = true;

it('opens populated sections after loading, respects manual collapse, and resets for another conversation', async () => {
  const container = document.createElement('div');
  const root = createRoot(container);
  const render = async (count: number, key = 'first') => act(async () => root.render(
    <CollapsibleSection key={key} title="Tasks" count={count} autoOpenWhenPopulated>Linked task</CollapsibleSection>,
  ));
  await render(0);
  expect(container.textContent).not.toContain('Linked task');
  await render(1);
  expect(container.textContent).toContain('Linked task');
  await act(async () => (container.querySelector('[role="button"]') as HTMLElement).click());
  await render(2);
  expect(container.textContent).not.toContain('Linked task');
  await render(2, 'second');
  expect(container.textContent).toContain('Linked task');
  await render(0, 'third');
  expect(container.textContent).not.toContain('Linked task');
  await act(async () => root.unmount());
});

it('preserves the existing default for sections that do not opt in', async () => {
  const container = document.createElement('div');
  const root = createRoot(container);
  await act(async () => root.render(<CollapsibleSection title="Other" count={2}>Content</CollapsibleSection>));
  expect(container.textContent).not.toContain('Content');
  await act(async () => root.unmount());
});

it('remembers the open or closed choice per persist key', async () => {
  window.localStorage.clear();
  const container = document.createElement('div');
  const root = createRoot(container);
  const render = async (key: string) => act(async () => root.render(
    <CollapsibleSection key={key} title="Session details" persistKey="test.session">Visit</CollapsibleSection>,
  ));
  await render('first');
  expect(container.textContent).not.toContain('Visit');
  await act(async () => (container.querySelector('[role="button"]') as HTMLElement).click());
  expect(container.textContent).toContain('Visit');
  await render('second');
  expect(container.textContent).toContain('Visit');
  await act(async () => root.unmount());
  window.localStorage.clear();
});

it('shows the count right after the title', async () => {
  const container = document.createElement('div');
  const root = createRoot(container);
  await act(async () => root.render(<CollapsibleSection title="Tasks" count={3}>Content</CollapsibleSection>));
  const title = Array.from(container.querySelectorAll('span')).find((span) => span.textContent === 'Tasks');
  expect(title?.nextElementSibling?.textContent).toBe('3');
  await act(async () => root.unmount());
});

it('keeps the section open when its header action is clicked or used from the keyboard', async () => {
  const container = document.createElement('div');
  const root = createRoot(container);
  let used = 0;
  await act(async () => root.render(
    <CollapsibleSection title="Tags" defaultOpen action={<button type="button" onClick={() => { used += 1; }}>Manage</button>}>
      Content
    </CollapsibleSection>,
  ));
  const action = Array.from(container.querySelectorAll('button')).find((button) => button.textContent === 'Manage') as HTMLElement;
  await act(async () => action.click());
  await act(async () => action.dispatchEvent(new KeyboardEvent('keydown', { key: 'Enter', bubbles: true })));
  await act(async () => action.dispatchEvent(new KeyboardEvent('keydown', { key: ' ', bubbles: true })));
  expect(used).toBe(1);
  expect(container.textContent).toContain('Content');
  await act(async () => root.unmount());
});
