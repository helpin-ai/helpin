import { useMutation, useQueryClient } from '@tanstack/react-query';
import { Switch } from '@/components/ui/switch';
import { setLiveTranslate, translationOptionsKey, useSupportTranslationOptions } from '@/hooks/queries/useSupportTranslation';
import { languageFlag } from './liveTranslateLanguages';

export function LiveTranslateBar({ workspaceId, conversationId, editable, forceVisible = false }: {
  workspaceId: string;
  conversationId: string;
  editable: boolean;
  forceVisible?: boolean;
}) {
  const query = useSupportTranslationOptions(workspaceId, conversationId);
  const client = useQueryClient();
  const save = useMutation({
    mutationFn: ({ enabled, language }: { enabled: boolean; language: string }) =>
      setLiveTranslate(workspaceId, conversationId, enabled, language),
    onSuccess: () => client.invalidateQueries({ queryKey: translationOptionsKey(workspaceId, conversationId).slice(0, 4) }),
  });
  const { data } = query;
  if (!data) return forceVisible ? <p className="text-sm text-muted-foreground">
    {query.isError ? <button type="button" onClick={() => void query.refetch()}>Could not load. Retry</button> : 'Loading…'}
  </p> : null;
  const language = data.conversation.customer_language || data.detected_customer_language || '';
  const reading = data.preference.reading_language;
  const enabled = data.conversation.translation_mode === 'on';
  if (language === reading && !forceVisible) return null;
  return <div className="flex shrink-0 flex-wrap items-center gap-2 border-b border-border/50 bg-muted/30 px-4 py-2 text-xs">
    <span className="font-medium" title="Automatically translates new messages and replies in this conversation.">Live Translate</span>
    <select
      aria-label="Customer language"
      className="min-w-0 max-w-44 rounded bg-transparent font-medium"
      disabled={!editable || save.isPending}
      value={data.conversation.customer_language}
      onChange={event => save.mutate({ enabled, language: event.target.value })}
    >
      <option value="">{language ? `${languageFlag(language)} ${data.languages[language]} (detected)` : 'Customer language unknown'}</option>
      {Object.entries(data.languages).map(([code, name]) => <option key={code} value={code}>{languageFlag(code)} {name}</option>)}
    </select>
    <span aria-hidden="true">↔</span>
    <span>{languageFlag(reading)} {data.languages[reading]}</span>
    <span className="ml-auto">{data.available ? (enabled ? 'On' : 'Off') : 'Unavailable'}</span>
    <Switch
      aria-label="Live Translate"
      checked={enabled}
      disabled={!editable || save.isPending}
      onCheckedChange={value => save.mutate({ enabled: value, language: data.conversation.customer_language })}
    />
    {save.isError && <span role="alert">Could not save. Try again.</span>}
  </div>;
}
