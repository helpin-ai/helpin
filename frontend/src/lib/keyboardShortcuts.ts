export function isEditableShortcutTarget(target: EventTarget | null): boolean {
  if (typeof HTMLElement === 'undefined' || !(target instanceof HTMLElement)) return false;

  const tagName = target.tagName.toLowerCase();
  if (tagName === 'input' || tagName === 'textarea' || tagName === 'select') {
    return true;
  }

  if (target.isContentEditable) {
    return true;
  }

  if (typeof target.closest !== 'function') {
    return false;
  }

  return Boolean(
    target.closest(
      '[contenteditable]:not([contenteditable="false"]), [role="textbox"]',
    ),
  );
}

/** Whether shortcuts use the Command key (Apple devices) rather than Ctrl. */
export function isMacPlatform(): boolean {
  if (typeof navigator === 'undefined') return false;

  return /Mac|iPhone|iPad|iPod/.test(navigator.platform) || /Mac|iPhone|iPad|iPod/.test(navigator.userAgent);
}
