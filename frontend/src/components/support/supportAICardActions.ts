/** Event the composer listens for to restore a draft. */
export const RESTORE_SUPPORT_DRAFT_EVENT = 'support:restore-draft';
/** Event the thread listens for to open the create-task window. */
export const CREATE_SUPPORT_TASK_EVENT = 'support:create-task';

/** Text-action style for buttons inside AI cards. */
export const cardAction = 'mt-1 inline-flex w-fit items-center gap-1.5 rounded-sm text-xs font-medium text-amber-700 hover:text-amber-900 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring disabled:opacity-60 dark:text-amber-300 dark:hover:text-amber-100';

/** Puts a reply in the conversation's composer as an editable draft. */
export function restoreReplyDraft(conversationId: string, markdown: string) {
  window.dispatchEvent(new CustomEvent(RESTORE_SUPPORT_DRAFT_EVENT, { detail: { conversationId, markdown } }));
}
