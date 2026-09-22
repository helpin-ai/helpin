// @vitest-environment jsdom
import { act } from 'react';
import { createRoot, type Root } from 'react-dom/client';
import { afterEach, beforeEach, expect, it } from 'vitest';
import { DockInteractionLayer, DockInteractionPrompt } from '../DockInteractionLayer';

(globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true;
let container: HTMLDivElement;
let root: Root;

beforeEach(() => {
  container = document.createElement('div');
  document.body.append(container);
  root = createRoot(container);
});

afterEach(() => {
  act(() => root.unmount());
  container.remove();
});

function renderPrompt(open: boolean, active = true) {
  act(() => root.render(
    <DockInteractionLayer active={active} interactionId={open ? 'question-1' : undefined} prompt={open ? <label>Which project?<input /></label> : null}>
      <div data-transcript>Earlier messages</div>
      <div data-plan>Current work plan</div>
      <textarea defaultValue="Unsent draft" />
    </DockInteractionLayer>,
  ));
}

it('keeps the transcript, plan and draft mounted while a prompt overlays them', () => {
  renderPrompt(false);
  const transcript = container.querySelector('[data-transcript]')!;
  const plan = container.querySelector('[data-plan]')!;
  const composer = container.querySelector('textarea')!;
  transcript.scrollTop = 120;
  composer.value = 'Keep this draft';
  composer.focus();

  renderPrompt(true);
  const background = container.querySelector('[data-dock-interaction-background]')!;
  const prompt = container.querySelector('[role="dialog"]')!;
  expect(background.hasAttribute('inert')).toBe(true);
  expect(background.contains(prompt)).toBe(false);
  expect(background.contains(plan)).toBe(true);
  expect(container.querySelector('[data-transcript]')).toBe(transcript);
  expect(container.querySelector('textarea')).toBe(composer);
  expect(document.activeElement).toBe(prompt);
  expect(transcript.scrollTop).toBe(120);

  renderPrompt(false);
  expect(background.hasAttribute('inert')).toBe(false);
  expect(composer.value).toBe('Keep this draft');
  expect(document.activeElement).toBe(composer);
  expect(container.querySelector('[data-plan]')).toBe(plan);
});

it('does not take keyboard focus when its dock view is inactive', () => {
  renderPrompt(false);
  const composer = container.querySelector('textarea')!;
  composer.focus();
  renderPrompt(true, false);
  expect(document.activeElement).toBe(composer);
});

it('queues delegated prompts in the overlay and removes them when resolved', () => {
  const render = (first: boolean, label = 'Approve first run') => act(() => root.render(
    <DockInteractionLayer>
      <textarea defaultValue="Draft" />
      {first && <DockInteractionPrompt id="first"><button>{label}</button></DockInteractionPrompt>}
      <DockInteractionPrompt id="second"><button>Answer second run</button></DockInteractionPrompt>
    </DockInteractionLayer>,
  ));
  render(true);
  expect(container.querySelector('[role="dialog"]')?.textContent).toBe('Approve first run');
  expect(container.querySelector('[data-dock-interaction-background]')?.textContent).not.toContain('Approve first run');
  render(true, 'Updated first run');
  expect(container.querySelector('[role="dialog"]')?.textContent).toBe('Updated first run');
  render(false);
  expect(container.querySelector('[role="dialog"]')?.textContent).toBe('Answer second run');
});
