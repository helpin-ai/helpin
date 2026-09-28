import { useMutation, useQueryClient } from '@tanstack/react-query';
import { QuietIconAction } from '@/components/design-system/quiet';
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/tooltip';
import { PauseIcon, PlayIcon } from '@/lib/icons';
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
  const actionLabel = enabled ? 'Pause live translation' : 'Resume live translation';
  return <div className="flex shrink-0 flex-wrap items-center justify-center gap-x-2 gap-y-1 text-center border-b border-border/50 bg-muted/30 px-4 py-2 text-xs">
    <span className="font-medium" title="Automatically translates new messages and replies in this conversation.">Live translate from</span>
    <select
      aria-label="Customer language"
      className="max-w-full shrink-0 rounded bg-transparent font-medium"
      disabled={!editable || save.isPending}
      value={data.conversation.customer_language}
      onChange={event => save.mutate({ enabled, language: event.target.value })}
    >
      <option value="">{language ? `${languageFlag(language)} ${data.languages[language]} (detected)` : 'Unknown language'}</option>
      {Object.entries(data.languages).map(([code, name]) => <option key={code} value={code}>{languageFlag(code)} {name}</option>)}
    </select>
    <span>to</span>
    <span className="inline-flex items-center gap-1.5">
      <span>{languageFlag(reading)} {data.languages[reading]}</span>
      {!data.available ? <span className="text-muted-foreground">Unavailable</span> : !enabled ? <span className="text-muted-foreground">Paused</span> : null}
      <Tooltip>
        <TooltipTrigger asChild>
          <QuietIconAction
            aria-label={actionLabel}
            disabled={!editable || save.isPending}
            onClick={() => save.mutate({ enabled: !enabled, language: data.conversation.customer_language })}
            className="shrink-0"
          >
            {enabled ? <PauseIcon className="h-3.5 w-3.5" /> : <PlayIcon className="h-3.5 w-3.5" />}
          </QuietIconAction>
        </TooltipTrigger>
        <TooltipContent>{actionLabel}</TooltipContent>
      </Tooltip>
    </span>
    {save.isError && <span role="alert" className="basis-full text-destructive">Could not save. Try again.</span>}
  </div>;
}
