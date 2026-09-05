// @vitest-environment jsdom
import { act } from 'react';
import { createRoot, type Root } from 'react-dom/client';
import { Editor } from '@tiptap/core';
import StarterKit from '@tiptap/starter-kit';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { useComposerRewrite } from '../useComposerRewrite';

(globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true;

function deferred() {
  let resolve!: (value: string) => void;
  let reject!: (reason: Error) => void;
  const promise = new Promise<string>((res, rej) => { resolve = res; reject = rej; });
  return { promise, resolve, reject };
}

describe('composer AI rewrite', () => {
  let editor: Editor;
  let root: Root;
  let container: HTMLDivElement;
  let current: ReturnType<typeof useComposerRewrite>;

  function Harness({ scope }: { scope: string }) {
    current = useComposerRewrite(editor, scope);
    return <span role="status">{current.isRewriting ? 'Rewriting draft…' : 'Ready'}</span>;
  }

  function setup() {
    editor = new Editor({ extensions: [StarterKit], content: '<p>Original draft</p>' });
    container = document.createElement('div');
    document.body.appendChild(container);
    root = createRoot(container);
    act(() => { root.render(<Harness scope="conversation-1" />); });
  }

  afterEach(() => {
    act(() => { root?.unmount(); });
    editor?.destroy();
    container?.remove();
  });

  it('locks editing immediately, rejects duplicate actions, and unlocks after success', async () => {
    setup();
    const result = deferred();
    let running!: Promise<boolean>;
    act(() => { running = current.runRewrite(() => result.promise); });
    expect(editor.isEditable).toBe(false);
    expect(container.textContent).toBe('Rewriting draft…');
    const duplicate = vi.fn(async () => 'Duplicate draft');
    expect(await current.runRewrite(duplicate)).toBe(false);
    expect(duplicate).not.toHaveBeenCalled();
    await act(async () => { result.resolve('Rewritten draft'); await running; });
    expect(editor.getText()).toBe('Rewritten draft');
    expect(editor.isEditable).toBe(true);
    expect(container.textContent).toBe('Ready');
  });

  it('preserves the draft and releases the lock when the request fails', async () => {
    setup();
    const result = deferred();
    let running!: Promise<boolean>;
    act(() => { running = current.runRewrite(() => result.promise); });
    await act(async () => {
      result.reject(new Error('Provider unavailable'));
      await expect(running).rejects.toThrow('Provider unavailable');
    });
    expect(editor.getText()).toBe('Original draft');
    expect(editor.isEditable).toBe(true);
    expect(container.textContent).toBe('Ready');
  });

  it('does not overwrite another conversation or release its newer rewrite lock', async () => {
    setup();
    const oldResult = deferred();
    let oldRun!: Promise<boolean>;
    act(() => { oldRun = current.runRewrite(() => oldResult.promise); });
    act(() => { root.render(<Harness scope="conversation-2" />); });
    editor.commands.setContent('Second conversation draft');
    const newResult = deferred();
    let newRun!: Promise<boolean>;
    act(() => { newRun = current.runRewrite(() => newResult.promise); });
    await act(async () => { oldResult.resolve('Wrong conversation'); expect(await oldRun).toBe(false); });
    expect(editor.getText()).toBe('Second conversation draft');
    expect(editor.isEditable).toBe(false);
    await act(async () => { newResult.resolve('Correct conversation'); await newRun; });
    expect(editor.getText()).toBe('Correct conversation');
    expect(editor.isEditable).toBe(true);
  });

  it('preserves a draft restored from the message thread during a rewrite', async () => {
    setup();
    const result = deferred();
    let running!: Promise<boolean>;
    act(() => { running = current.runRewrite(() => result.promise); });
    editor.commands.setContent('Restored reply');
    await act(async () => { result.resolve('Old rewritten draft'); expect(await running).toBe(false); });
    expect(editor.getText()).toBe('Restored reply');
    expect(editor.isEditable).toBe(true);
  });

  it('ignores a result after the composer unmounts', async () => {
    setup();
    const result = deferred();
    let running!: Promise<boolean>;
    act(() => { running = current.runRewrite(() => result.promise); });
    act(() => { root.unmount(); });
    result.resolve('Late result');
    expect(await running).toBe(false);
    expect(editor.getText()).toBe('Original draft');
    expect(editor.isEditable).toBe(true);
  });
});
