import { useMutation, useQueryClient } from '@tanstack/react-query';
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/tooltip';
import { ArrowDown01Icon } from '@/lib/icons';
import { cn } from '@/lib/utils';
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
  const detectedLanguage = data.detected_customer_language || '';
  const language = data.conversation.customer_language || detectedLanguage;
  const reading = data.preference.reading_language;
  const enabled = data.conversation.translation_mode === 'on';
  if ((!language || language === reading) && !forceVisible) return null;
  const actionLabel = enabled ? 'Pause live translation' : 'Enable live translation';
  const active = enabled && data.available;
  return <div className={cn(
    'flex shrink-0 flex-wrap items-center justify-center gap-x-2 gap-y-1 border-b px-4 py-2 text-center text-xs',
    active
      ? 'border-teal-300 bg-teal-100 text-teal-800 dark:border-[#28746d] dark:bg-[#123b39] dark:text-teal-200'
      : 'border-border/50 bg-muted/30 text-muted-foreground',
  )}>
    <span className="font-medium" title={language
      ? `Customer messages are translated into ${data.languages[reading]}. Your replies are translated into ${data.languages[language]}.`
      : 'Choose the customer’s language to translate incoming messages and your replies.'}>Live translation</span>
    <span aria-hidden="true">·</span>
    <span className="relative inline-flex max-w-full min-w-0 items-center">
      <select
        aria-label="Customer language"
        className={cn(
          'max-w-full appearance-none rounded-full border py-1 pl-2.5 pr-7 font-medium leading-4 outline-none focus-visible:ring-2 focus-visible:ring-current disabled:cursor-not-allowed disabled:opacity-50 [&>option]:bg-background [&>option]:text-foreground',
          active
            ? 'border-teal-400/70 bg-white/30 dark:border-[#28746d] dark:bg-white/5'
            : 'border-border bg-transparent',
        )}
        disabled={!editable || save.isPending}
        value={data.conversation.customer_language}
        onChange={event => save.mutate({ enabled, language: event.target.value })}
      >
        <option value="">{detectedLanguage ? `${languageFlag(detectedLanguage)} ${data.languages[detectedLanguage]} (detected)` : 'Select customer language'}</option>
        {Object.entries(data.languages).map(([code, name]) => <option key={code} value={code}>{languageFlag(code)} {name}</option>)}
      </select>
      <span aria-hidden="true" className="pointer-events-none absolute right-2 inline-flex items-center">
        <ArrowDown01Icon className="h-3 w-3" />
      </span>
    </span>
    <span aria-label="both ways">↔</span>
    <span>{languageFlag(reading)} {data.languages[reading]}</span>
    <span className="inline-flex items-center gap-1.5">
      {!data.available ? <span className="text-muted-foreground">Unavailable</span> : !enabled ? <span className="text-muted-foreground">Paused</span> : null}
      <Tooltip>
        <TooltipTrigger asChild>
          <button
            type="button"
            aria-label={actionLabel}
            disabled={!editable || save.isPending}
            onClick={() => save.mutate({ enabled: !enabled, language: data.conversation.customer_language })}
            className="shrink-0 rounded-sm px-1 py-1 font-medium underline decoration-dotted underline-offset-4 hover:decoration-2 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-current disabled:cursor-not-allowed disabled:opacity-50"
          >
            {enabled ? 'Pause' : 'Enable'}
          </button>
        </TooltipTrigger>
        <TooltipContent>{actionLabel}</TooltipContent>
      </Tooltip>
    </span>
    {save.isError && <span role="alert" className="basis-full text-destructive">Could not save. Try again.</span>}
  </div>;
}
