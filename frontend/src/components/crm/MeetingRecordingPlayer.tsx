import { forwardRef, useEffect, useImperativeHandle, useRef, useState } from 'react';
import { LinkSquare01Icon, Loading01Icon, PlayCircleIcon } from '@/lib/icons';
import { QuietEmptyState, QuietSection, QuietTextAction } from '@/components/design-system/quiet';
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
        <QuietSection
          title="Meeting recording"
          icon={PlayCircleIcon}
          action={recording ? (
            <QuietTextAction asChild>
              <a href={recording.url} target="_blank" rel="noreferrer">
                Open recording <LinkSquare01Icon className="h-3.5 w-3.5" />
              </a>
            </QuietTextAction>
          ) : undefined}
          className="px-0 sm:px-0 lg:px-0"
        >
          <p className="mb-4 text-sm leading-[1.6] text-quiet-text-tertiary">
            {recording
              ? recording.media_type === 'video'
                ? 'Private video and audio captured by Helpin.'
                : 'Private audio captured by Helpin.'
              : 'Preparing private playback from Helpin storage.'}
          </p>
          {state.status === 'loading' ? (
            <div className="space-y-3">
              <Skeleton className="aspect-video w-full rounded-none" />
              <div className="flex items-center gap-2 text-[12.5px] text-quiet-text-tertiary">
                <Loading01Icon className="h-3.5 w-3.5 animate-spin" />Preparing secure playback…
              </div>
            </div>
          ) : state.status === 'error' ? (
            <QuietEmptyState
              className="border-b-0"
              title="Recording could not be loaded"
              description={state.message}
              action={(
                <QuietTextAction className="border-b border-quiet-field pb-0.5" onClick={() => setReloadKey((current) => current + 1)}>
                  Try again
                </QuietTextAction>
              )}
            />
          ) : state.recording.media_type === 'video' ? (
            <video ref={setMediaElement} src={state.recording.url} controls preload="metadata" playsInline onTimeUpdate={handleTimeUpdate} onError={() => setState({ status: 'error', message: 'The recording could not be played. Refresh the secure link and try again.' })} className="aspect-video w-full bg-black object-contain">
              <track kind="captions" />
              Meeting recording playback is not supported by this browser.
            </video>
          ) : state.recording.media_type === 'audio' ? (
            <div className="flex min-h-28 items-center border-y border-quiet-divider-light py-5">
              <audio ref={setMediaElement} src={state.recording.url} controls preload="metadata" onTimeUpdate={handleTimeUpdate} onError={() => setState({ status: 'error', message: 'The recording could not be played. Refresh the secure link and try again.' })} className="w-full">Meeting recording playback is not supported by this browser.</audio>
            </div>
          ) : (
            <div className="border-y border-quiet-divider-light py-5">
              <QuietTextAction asChild className="border-b border-quiet-field pb-0.5">
                <a href={state.recording.url} target="_blank" rel="noreferrer">Open recording <LinkSquare01Icon className="h-4 w-4" /></a>
              </QuietTextAction>
            </div>
          )}
        </QuietSection>
      </div>
    );
  },
);
