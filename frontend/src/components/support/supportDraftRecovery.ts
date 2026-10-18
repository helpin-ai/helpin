import type { PendingSupportAttachment } from './draftAttachments';

type RecoveryDraft = { mode: 'reply' | 'note'; attachments: PendingSupportAttachment[] };
const key = (workspaceId: string, conversationId: string) => `support-refresh-draft:${workspaceId}:${conversationId}`;

export function loadSupportRecoveryDraft(workspaceId: string, conversationId: string): RecoveryDraft | null {
  try {
    const saved = JSON.parse(sessionStorage.getItem(key(workspaceId, conversationId)) ?? 'null');
    if (!saved || !Number.isFinite(saved.savedAt) || Date.now() - saved.savedAt > 86_400_000 || !['reply', 'note'].includes(saved.mode) || !Array.isArray(saved.attachments)) return null;
    if (saved.attachments.some((item: PendingSupportAttachment) => item.status !== 'done' || typeof item.attachmentId !== 'string' || typeof item.fileName !== 'string' || typeof item.fileType !== 'string')) return null;
    return { mode: saved.mode, attachments: saved.attachments };
  } catch { return null; }
}

export function saveSupportRecoveryDraft(workspaceId: string, conversationId: string, draft: RecoveryDraft) {
  if (draft.attachments.some(item => item.status !== 'done' || !item.attachmentId)) throw new Error('Unfinished attachment');
  // Object URLs expire on refresh; keep the uploaded IDs and filenames. The
  // attachment can still be sent, without pretending a local preview survived.
  const attachments = draft.attachments.map(({ localId, attachmentId, fileName, fileType, status }) => ({ localId, attachmentId, fileName, fileType, status }));
  sessionStorage.setItem(key(workspaceId, conversationId), JSON.stringify({ mode: draft.mode, attachments, savedAt: Date.now() }));
}

export function clearSupportRecoveryDraft(workspaceId: string, conversationId: string) {
  try { sessionStorage.removeItem(key(workspaceId, conversationId)); } catch { /* Storage may be disabled. */ }
}
