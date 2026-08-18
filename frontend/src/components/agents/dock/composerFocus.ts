export function focusComposerAtEnd(textarea: HTMLTextAreaElement | null, value: string) {
  if (!textarea) return;
  textarea.focus();
  textarea.setSelectionRange(value.length, value.length);
}
