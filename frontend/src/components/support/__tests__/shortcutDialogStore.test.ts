import { afterEach, describe, expect, it } from 'vitest';

import { useShortcutComposerStore } from '../shortcutDialogStore';

describe('shortcutDialogStore', () => {
  afterEach(() => {
    useShortcutComposerStore.setState({
      openRequest: 0,
      requestSeq: 0,
      seedShortCode: undefined,
      seedContent: undefined,
    });
  });

  it('clears consumed shortcut create requests without reusing request ids', () => {
    useShortcutComposerStore.getState().openCreate({ seedContent: 'First' });
    const firstRequest = useShortcutComposerStore.getState().openRequest;

    useShortcutComposerStore.getState().clear();
    expect(useShortcutComposerStore.getState().openRequest).toBe(0);
    expect(useShortcutComposerStore.getState().seedContent).toBeUndefined();

    useShortcutComposerStore.getState().openCreate({ seedContent: 'Second' });
    const secondRequest = useShortcutComposerStore.getState().openRequest;

    expect(secondRequest).toBeGreaterThan(firstRequest);
    expect(useShortcutComposerStore.getState().seedContent).toBe('Second');
  });

  it('treats an empty-seed create as a pending request', () => {
    useShortcutComposerStore.getState().openCreate();

    expect(useShortcutComposerStore.getState().openRequest).toBeGreaterThan(0);
    expect(useShortcutComposerStore.getState().seedShortCode).toBeUndefined();
    expect(useShortcutComposerStore.getState().seedContent).toBeUndefined();
  });
});
