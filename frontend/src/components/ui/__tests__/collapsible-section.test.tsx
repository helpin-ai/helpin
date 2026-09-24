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
