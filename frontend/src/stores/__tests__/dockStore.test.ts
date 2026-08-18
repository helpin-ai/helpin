import { beforeEach, describe, expect, it } from 'vitest';

import { useDockStore } from '../dockStore';

describe('dock transcript cache', () => {
  beforeEach(() => {
    useDockStore.setState({ workspaceId: null, transcripts: {} });
  });

  it('retains a transcript within a workspace and clears it on workspace change', () => {
    useDockStore.getState().activateWorkspace('ws-1');
    useDockStore.getState().cacheTranscript('chat-1', {
      detail: { chat: { id: 'chat-1' } } as never,
      messages: [{ id: 'message-1' }] as never,
      nextBefore: 42,
    });

    useDockStore.getState().activateWorkspace('ws-1');
    expect(useDockStore.getState().transcripts['chat-1']?.messages).toHaveLength(1);

    useDockStore.getState().cacheTranscript('chat-2', {
      detail: { chat: { id: 'chat-2' } } as never,
      messages: [] as never,
      nextBefore: null,
    });
    expect(Object.keys(useDockStore.getState().transcripts)).toEqual(['chat-2']);

    useDockStore.getState().activateWorkspace('ws-2');
    expect(useDockStore.getState().transcripts).toEqual({});
  });
});
