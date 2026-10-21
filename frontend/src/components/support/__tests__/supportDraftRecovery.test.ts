// @vitest-environment jsdom
import { beforeEach, expect, it } from 'vitest';
import { loadSupportRecoveryDraft, saveSupportRecoveryDraft } from '../supportDraftRecovery';

beforeEach(() => sessionStorage.clear());

it('restores the note/reply mode and uploaded attachment IDs only for the matching conversation', () => {
  saveSupportRecoveryDraft('ws-1', 'conv-1', { mode: 'note', attachments: [{ localId: 'local-1', attachmentId: 'att-1', status: 'done', fileName: 'image.png', fileType: 'image/png', previewUrl: 'blob:expired', previewObjectUrl: true }] });
  expect(loadSupportRecoveryDraft('ws-1', 'conv-1')).toEqual({ mode: 'note', attachments: [{ localId: 'local-1', attachmentId: 'att-1', status: 'done', fileName: 'image.png', fileType: 'image/png' }] });
  expect(loadSupportRecoveryDraft('ws-2', 'conv-1')).toBeNull();
  expect(loadSupportRecoveryDraft('ws-1', 'conv-2')).toBeNull();
});

it('never pretends an unfinished upload can survive a refresh', () => {
  expect(() => saveSupportRecoveryDraft('ws-1', 'conv-1', { mode: 'reply', attachments: [{ localId: 'pending', status: 'uploading', fileName: 'file.txt', fileType: 'text/plain' }] })).toThrow();
  expect(loadSupportRecoveryDraft('ws-1', 'conv-1')).toBeNull();
});
