import { UpgradeRequiredDialog } from '@edition';
import { getUpgradeRequiredReason } from '@edition/errors';
import { Button } from '@/components/ui/button';
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/tooltip';
import { Cancel01Icon, Loading01Icon, Mic01Icon, StopIcon } from '@/lib/icons';
import type { useVoiceInput } from '@/hooks/useVoiceInput';

export function VoiceMicrophone({ enabled, onStart }: { enabled: boolean; onStart: () => void }) {
  return <Tooltip>
    <TooltipTrigger asChild>
      <Button type="button" variant="ghost" size="icon" className="h-7 w-7 shrink-0 rounded-full text-muted-foreground"
        aria-label="Dictate message" disabled={!enabled} onClick={onStart}>
        <Mic01Icon className="h-4 w-4" />
      </Button>
    </TooltipTrigger>
    <TooltipContent>Dictate up to 5 minutes · review before sending</TooltipContent>
  </Tooltip>;
}

export function VoiceFeedback({ voice }: { voice: ReturnType<typeof useVoiceInput> }) {
  const upgradeReason = getUpgradeRequiredReason(voice.error);
  const time = `${Math.floor(voice.seconds / 60)}:${String(Math.floor(voice.seconds % 60)).padStart(2, '0')}`;
  return <>
    {voice.busy ? <div className="flex min-h-10 w-full items-center gap-2 py-1" aria-label="Voice input" data-voice-input>
      <Button type="button" variant="ghost" size="icon" className="h-8 w-8 shrink-0 rounded-full text-muted-foreground"
        aria-label="Cancel voice input" onClick={voice.cancel}>
        <Cancel01Icon className="h-4 w-4" />
      </Button>
      {voice.phase === 'recording' ? <>
        <span className="sr-only" role="status">Recording. Stop to transcribe.</span>
        <div className="flex h-6 min-w-0 flex-1 items-center justify-center gap-[3px] overflow-hidden text-primary" aria-hidden="true">
          {voice.levels.map((level, i) => <span key={i} className="w-[3px] shrink-0 rounded-full bg-current" style={{ height: `${3 + level * 21}px` }} />)}
        </div>
        <span className="shrink-0 text-xs tabular-nums text-muted-foreground" aria-label={`${Math.floor(voice.seconds)} seconds recorded`}>{time}</span>
        <Button type="button" variant="secondary" size="sm" className="h-8 shrink-0 gap-1.5 rounded-full px-3" onClick={() => void voice.stop()}>
          <StopIcon className="h-3 w-3" />Stop
        </Button>
      </> : <span className="flex min-w-0 flex-1 items-center gap-2 text-sm text-muted-foreground" role="status">
        <Loading01Icon className="h-4 w-4 shrink-0 animate-spin" />
        {voice.phase === 'requesting' ? 'Waiting for microphone…' : 'Transcribing…'}
      </span>}
    </div> : null}
    {voice.error ? <div className="flex w-full items-start gap-2 py-1 text-xs text-destructive" role="alert">
      <span className="min-w-0 flex-1">{voice.error}</span>
      <button type="button" className="shrink-0 rounded p-0.5 focus-visible:outline focus-visible:outline-ring" aria-label="Dismiss voice input error" onClick={voice.dismissError}>
        <Cancel01Icon className="h-3.5 w-3.5" />
      </button>
    </div> : null}
    {upgradeReason ? <UpgradeRequiredDialog open onOpenChange={open => { if (!open) voice.dismissError(); }} reason={upgradeReason} /> : null}
  </>;
}
