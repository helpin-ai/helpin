import { forwardRef, useEffect, useImperativeHandle, useRef, useState } from 'react';
import { LinkSquare01Icon, Loading01Icon, PlayCircleIcon } from '@/lib/icons';
import { Button } from '@/components/ui/button';
import { Card, CardAction, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { Skeleton } from '@/components/ui/skeleton';
import { crmMeetingService } from '@/lib/services/crmMeetingService';
import type { CRMMeetingRecording } from '@/lib/crmMeetingTypes';

export interface MeetingRecordingPlayerHandle {
  focus: () => void;
  seekTo: (seconds: number) => void;
}

interface MeetingRecordingPlayerProps {
  workspaceId: string;
  meetingId: string;
  onTimeUpdate?: (seconds: number) => void;
}

type RecordingState =
  | { status: 'loading' }
  | { status: 'error'; message: string }
  | { status: 'ready'; recording: CRMMeetingRecording };

export const MeetingRecordingPlayer = forwardRef<MeetingRecordingPlayerHandle, MeetingRecordingPlayerProps>(
  function MeetingRecordingPlayer({ workspaceId, meetingId, onTimeUpdate }, ref) {
    const containerRef = useRef<HTMLDivElement | null>(null);
    const mediaRef = useRef<HTMLMediaElement | null>(null);
    const [reloadKey, setReloadKey] = useState(0);
    const [state, setState] = useState<RecordingState>({ status: 'loading' });

    useEffect(() => {
      let active = true;
      setState({ status: 'loading' });
      void crmMeetingService.getRecording(workspaceId, meetingId).then((response) => {
        if (!active) return;
        if (response.error || !response.data?.url) {
          setState({ status: 'error', message: response.error ?? 'Recording is unavailable' });
          return;
        }
        setState({ status: 'ready', recording: response.data });
      }).catch((error: unknown) => {
        if (!active) return;
        const message = error instanceof Error ? error.message : 'Recording is unavailable';
        setState({ status: 'error', message });
      });
      return () => { active = false; };
    }, [meetingId, reloadKey, workspaceId]);

    useImperativeHandle(ref, () => ({
      focus: () => {
        containerRef.current?.scrollIntoView({ behavior: 'smooth', block: 'start' });
        mediaRef.current?.focus();
      },
      seekTo: (seconds: number) => {
        const media = mediaRef.current;
        if (!media) return;
        media.currentTime = Math.max(0, seconds);
        containerRef.current?.scrollIntoView({ behavior: 'smooth', block: 'start' });
        void media.play().catch(() => undefined);
      },
    }), []);

    const recording = state.status === 'ready' ? state.recording : null;
    const setMediaElement = (element: HTMLMediaElement | null) => { mediaRef.current = element; };
    const handleTimeUpdate = () => onTimeUpdate?.(mediaRef.current?.currentTime ?? 0);

    return (
      <div ref={containerRef} className="scroll-mt-4">
        <Card className="overflow-hidden">
          <CardHeader>
            <CardTitle className="flex items-center gap-2 text-base"><PlayCircleIcon className="h-4 w-4" />Meeting recording</CardTitle>
            <CardDescription>{recording ? (recording.media_type === 'video' ? 'Video and audio captured by Helpin.' : 'Audio captured by Helpin.') : 'Private playback from Helpin storage.'}</CardDescription>
            {recording && <CardAction><Button asChild variant="outline" size="sm" className="h-7 text-xs"><a href={recording.url} target="_blank" rel="noreferrer">Open recording <LinkSquare01Icon className="h-3.5 w-3.5" /></a></Button></CardAction>}
          </CardHeader>
          <CardContent>
            {state.status === 'loading' ? (
              <div className="space-y-3"><Skeleton className="aspect-video w-full rounded-lg" /><div className="flex items-center gap-2 text-xs text-muted-foreground"><Loading01Icon className="h-3.5 w-3.5 animate-spin" />Preparing secure playback…</div></div>
            ) : state.status === 'error' ? (
              <div className="flex min-h-36 flex-col items-center justify-center rounded-lg border border-dashed bg-muted/20 px-6 text-center">
                <p className="text-sm font-medium">Recording could not be loaded</p>
                <p className="mt-1 text-xs text-muted-foreground">{state.message}</p>
                <Button className="mt-3" size="sm" variant="outline" onClick={() => setReloadKey((current) => current + 1)}>Try again</Button>
              </div>
            ) : state.recording.media_type === 'video' ? (
              <video ref={setMediaElement} src={state.recording.url} controls preload="metadata" playsInline onTimeUpdate={handleTimeUpdate} onError={() => setState({ status: 'error', message: 'The recording could not be played. Refresh the secure link and try again.' })} className="aspect-video w-full rounded-lg bg-black object-contain">
                <track kind="captions" />
                Meeting recording playback is not supported by this browser.
              </video>
            ) : state.recording.media_type === 'audio' ? (
              <div className="flex min-h-32 items-center rounded-lg border bg-muted/20 px-5">
                <audio ref={setMediaElement} src={state.recording.url} controls preload="metadata" onTimeUpdate={handleTimeUpdate} onError={() => setState({ status: 'error', message: 'The recording could not be played. Refresh the secure link and try again.' })} className="w-full">Meeting recording playback is not supported by this browser.</audio>
              </div>
            ) : (
              <div className="flex min-h-32 items-center justify-center rounded-lg border bg-muted/20"><Button asChild variant="outline"><a href={state.recording.url} target="_blank" rel="noreferrer">Open recording <LinkSquare01Icon className="h-4 w-4" /></a></Button></div>
            )}
          </CardContent>
        </Card>
      </div>
    );
  },
);
