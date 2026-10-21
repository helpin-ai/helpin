// @vitest-environment jsdom
import { act, useState } from 'react';
import { createRoot, type Root } from 'react-dom/client';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { TooltipProvider } from '@/components/ui/tooltip';
import { searchDocsEntityItems } from '@/components/docs/entitySearch';
import type { DockEntityReference } from '@/lib/dockTypes';
import { DockInput } from '../DockInput';

vi.mock('@/hooks/useVoiceComposer', () => ({
  useVoiceComposer: () => ({ busy: false, feedback: null, microphone: null, cancel: vi.fn() }),
}));
vi.mock('@/components/docs/entitySearch', async (importOriginal) => ({
  ...await importOriginal<typeof import('@/components/docs/entitySearch')>(),
  searchDocsEntityItems: vi.fn(),
}));

const task = { entityType: 'task' as const, entityId: 'task-1', title: 'Fix login', meta: 'Task' };
let container: HTMLDivElement;
let root: Root;
let textarea: HTMLTextAreaElement;
const onSubmit = vi.fn();
const onAddReference = vi.fn();

beforeEach(() => {
  (globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true;
  vi.clearAllMocks();
  vi.mocked(searchDocsEntityItems).mockResolvedValue({ items: [task], error: null });
  container = document.createElement('div');
  document.body.append(container);
  root = createRoot(container);
});
afterEach(() => {
  act(() => root.unmount());
  container.remove();
});

async function render(initial = '', references: DockEntityReference[] = []) {
  function Composer() {
    const [value, setValue] = useState(initial);
    return <DockInput mode="conversation" pageContext={null} workspaceId="ws" value={value}
      onChange={setValue} onSubmit={onSubmit} onAddReference={onAddReference} references={references} />;
  }
  await act(async () => root.render(<TooltipProvider><Composer /></TooltipProvider>));
  textarea = container.querySelector('textarea')!;
  act(() => textarea.focus());
}

function key(key: string, extra: KeyboardEventInit = {}) {
  const event = new KeyboardEvent('keydown', { key, bubbles: true, cancelable: true, ...extra });
  act(() => textarea.dispatchEvent(event));
  return event;
}

function edit(value: string, caret = value.length) {
  act(() => {
    Object.getOwnPropertyDescriptor(HTMLTextAreaElement.prototype, 'value')!.set!.call(textarea, value);
    textarea.setSelectionRange(caret, caret);
    textarea.dispatchEvent(new Event('input', { bubbles: true }));
  });
}

function type(text: string) {
  for (const character of text) {
    if (!key(character).defaultPrevented) {
      const start = textarea.selectionStart;
      edit(textarea.value.slice(0, start) + character + textarea.value.slice(textarea.selectionEnd), start + 1);
    }
    act(() => textarea.dispatchEvent(new KeyboardEvent('keyup', { key: character, bubbles: true })));
  }
}

async function results() {
  await act(async () => { await new Promise(resolve => setTimeout(resolve, 220)); });
}

function suggestions() { return document.querySelector('[role="listbox"]'); }

describe('Ask context autocomplete', () => {
  it('inserts @ and filters suggestions without moving focus out of the message', async () => {
    await render();
    type('@fix');
    await results();
    expect(textarea.value).toBe('@fix');
    expect(document.activeElement).toBe(textarea);
    expect(suggestions()?.textContent).toContain('Fix login');
    expect(document.querySelector('input')).toBeNull();
    expect(searchDocsEntityItems).toHaveBeenLastCalledWith(expect.objectContaining({ query: 'fix' }));
  });

  it('leaves email addresses and embedded @ characters alone', async () => {
    await render();
    type('name@example.com package@version');
    expect(textarea.value).toBe('name@example.com package@version');
    expect(suggestions()).toBeNull();
    expect(searchDocsEntityItems).not.toHaveBeenCalled();
  });

  it('dismisses with Escape, preserves the token, and only reopens for a new token', async () => {
    await render();
    type('@fix');
    key('Escape');
    type('es');
    expect(textarea.value).toBe('@fixes');
    expect(suggestions()).toBeNull();
    type(' @new');
    await results();
    expect(suggestions()).not.toBeNull();
    expect(document.activeElement).toBe(textarea);
  });

  it('consumes Escape before the dock shortcut and preserves focus', async () => {
    await render();
    type('@fix');
    const closeDock = vi.fn();
    document.addEventListener('keydown', closeDock);
    try {
      key('Escape');
      expect(closeDock).not.toHaveBeenCalled();
      expect(textarea.value).toBe('@fix');
      expect(document.activeElement).toBe(textarea);
    } finally { document.removeEventListener('keydown', closeDock); }
  });

  it('can switch from inline suggestions to the full picker without changing text', async () => {
    await render();
    type('@fix');
    act(() => Array.from(container.querySelectorAll('button')).find(button => button.textContent === 'Add context')!.click());
    await results();
    expect(document.activeElement).toBe(document.querySelector('input'));
    expect(textarea.value).toBe('@fix');
    act(() => document.querySelector<HTMLButtonElement>('[role="option"]')!.click());
    expect(onAddReference).toHaveBeenCalledTimes(1);
    expect(textarea.value).toBe('@fix');
    expect(suggestions()).toBeNull();
  });

  it('returns focus from the full picker on Escape without triggering the dock shortcut', async () => {
    await render();
    const trigger = Array.from(container.querySelectorAll('button')).find(button => button.textContent === 'Add context')!;
    act(() => trigger.click());
    await results();
    const closeDock = vi.fn();
    document.addEventListener('keydown', closeDock);
    try {
      act(() => document.querySelector('input')!.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape', bubbles: true, cancelable: true })));
      expect(closeDock).not.toHaveBeenCalled();
      expect(document.activeElement).toBe(trigger);
      expect(suggestions()).toBeNull();
    } finally { document.removeEventListener('keydown', closeDock); }
  });

  it('keeps the composer focused when switching the context type', async () => {
    await render();
    type('@fix');
    const docButton = Array.from(document.querySelectorAll('button')).find(button => button.textContent === 'Doc')!;
    act(() => {
      const mouse = new MouseEvent('mousedown', { bubbles: true, cancelable: true });
      if (docButton.dispatchEvent(mouse)) docButton.focus();
      docButton.click();
    });
    await results();
    expect(searchDocsEntityItems).toHaveBeenLastCalledWith(expect.objectContaining({ fixedEntityType: 'document', query: 'fix' }));
    expect(document.activeElement).toBe(textarea);
    expect(textarea.value).toBe('@fix');
  });

  it('dismisses on blur without changing text or stealing focus back', async () => {
    await render();
    type('@fix');
    const outside = document.createElement('button');
    document.body.append(outside);
    try {
      act(() => outside.focus());
      await act(async () => { await new Promise(resolve => setTimeout(resolve, 10)); });
      expect(suggestions()).toBeNull();
      expect(textarea.value).toBe('@fix');
      expect(document.activeElement).toBe(outside);
    } finally { outside.remove(); }
  });

  it('selects with arrows and Enter without sending or deleting surrounding text', async () => {
    await render();
    edit('Review @fix tomorrow', 11);
    await results();
    key('ArrowDown');
    const activeId = textarea.getAttribute('aria-activedescendant');
    expect(activeId).toBeTruthy();
    expect(document.getElementById(activeId!)?.textContent).toContain('Fix login');
    key('Enter');
    expect(onAddReference).toHaveBeenCalledWith({ entity_type: 'task', entity_id: 'task-1', display_title: 'Fix login' });
    expect(textarea.value).toBe('Review  tomorrow');
    expect(textarea.selectionStart).toBe(7);
    expect(document.activeElement).toBe(textarea);
    expect(onSubmit).not.toHaveBeenCalled();
    expect(suggestions()).toBeNull();
  });

  it('keeps @ as literal text when Enter is pressed without selecting a suggestion', async () => {
    await render();
    type('@');
    await results();
    key('Enter');
    expect(textarea.value).toBe('@');
    expect(onAddReference).not.toHaveBeenCalled();
    expect(onSubmit).toHaveBeenCalledTimes(1);
  });

  it('adds a clicked suggestion and restores the insertion point', async () => {
    await render();
    type('Use @fix');
    await results();
    const option = document.querySelector<HTMLButtonElement>('[role="option"]');
    expect(option).not.toBeNull();
    act(() => option!.click());
    expect(onAddReference).toHaveBeenCalledTimes(1);
    expect(textarea.value).toBe('Use ');
    expect(document.activeElement).toBe(textarea);
    expect(textarea.selectionStart).toBe(4);
  });

  it('retains the full picker and leaves the draft untouched when opened from Add context', async () => {
    await render('Keep @literal');
    act(() => Array.from(container.querySelectorAll('button')).find(button => button.textContent === 'Add context')!.click());
    await results();
    expect(document.activeElement).toBe(document.querySelector('input'));
    act(() => Array.from(document.querySelectorAll('button')).find(button => button.textContent?.includes('Fix login'))!.click());
    expect(onAddReference).toHaveBeenCalledTimes(1);
    expect(textarea.value).toBe('Keep @literal');
  });

  it('skips references that are already attached during keyboard navigation', async () => {
    vi.mocked(searchDocsEntityItems).mockResolvedValue({ items: [task, { ...task, entityId: 'task-2', title: 'Another fix' }], error: null });
    await render('', [{ entity_type: 'task', entity_id: 'task-1', display_title: 'Fix login' }]);
    type('@fix');
    await results();
    key('ArrowDown');
    key('Enter');
    expect(onAddReference).toHaveBeenCalledWith(expect.objectContaining({ entity_id: 'task-2' }));
  });

  it('never selects stale results while a new query is pending', async () => {
    await render();
    type('@fix');
    await results();
    key('ArrowDown');
    type('x');
    key('Enter');
    expect(onAddReference).not.toHaveBeenCalled();
    expect(textarea.value).toBe('@fixx');
  });

  it('allows Shift+Enter and Tab without selecting a result', async () => {
    await render();
    type('@fix');
    await results();
    key('ArrowDown');
    expect(key('Enter', { shiftKey: true }).defaultPrevented).toBe(false);
    expect(key('Tab').defaultPrevented).toBe(false);
    expect(onSubmit).not.toHaveBeenCalled();
    expect(onAddReference).not.toHaveBeenCalled();
    expect(suggestions()).toBeNull();
  });

  it('does not intercept composition keystrokes', async () => {
    await render();
    act(() => textarea.dispatchEvent(new CompositionEvent('compositionstart', { bubbles: true })));
    edit('@fix');
    expect(key('Enter', { isComposing: true }).defaultPrevented).toBe(false);
    expect(suggestions()).toBeNull();
    expect(onSubmit).not.toHaveBeenCalled();
  });

  it('shows search failures without losing the draft or preventing dismissal', async () => {
    vi.mocked(searchDocsEntityItems).mockRejectedValue(new Error('Network failed'));
    await render();
    type('@fix');
    await results();
    expect(document.body.textContent).toContain('Could not load context');
    key('Escape');
    expect(textarea.value).toBe('@fix');
    expect(document.activeElement).toBe(textarea);
  });
});
