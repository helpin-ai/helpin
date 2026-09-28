export const DEFAULT_PRIVACY_NOTICE_TEXT = 'By chatting with us, you agree to our';

/** Only web links are allowed in a visitor-facing policy link. */
export function getPrivacyPolicyURL(value?: string): string | undefined {
  if (!value || !/^https?:\/\//i.test(value.trim())) return undefined;
  try {
    const url = new URL(value.trim());
    if (!['https:', 'http:'].includes(url.protocol) || !url.hostname || url.username || url.password) return undefined;
    return url.href;
  } catch {
    return undefined;
  }
}
