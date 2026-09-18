import { useSupportTranslationOptions } from './useSupportTranslation';

export function useOutgoingSupportTranslation(
  workspaceId: string,
  conversationId: string,
) {
  const options = useSupportTranslationOptions(workspaceId, conversationId);
  const language =
    options.data?.conversation.customer_language ||
    options.data?.detected_customer_language ||
    '';
  const enabled = Boolean(
    options.data?.available && options.data.preference.auto_translate_outgoing,
  );
  return { options, enabled, language };
}
