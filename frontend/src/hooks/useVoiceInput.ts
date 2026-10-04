import { useCallback, useLayoutEffect, useRef, useState } from 'react';
import { MAX_VOICE_SECONDS, startVoiceRecording, type VoiceRecording } from '@/lib/voiceRecording';
import { transcribeVoice } from '@/lib/services/voiceInputService';

export type VoicePhase = 'idle' | 'requesting' | 'recording' | 'transcribing';
interface Options {
  workspaceId?: string;
  identity: string;
  enabled: boolean;
  onTranscript: (text: string) => void;
}

export function useVoiceInput({ workspaceId, identity, enabled, onTranscript }: Options) {
  const [phase, setPhase] = useState<VoicePhase>('idle');
  const [seconds, setSeconds] = useState(0);
  const [levels, setLevels] = useState<number[]>(Array(24).fill(0));
  const [error, setError] = useState<string | null>(null);
  const generation = useRef(0);
  const controller = useRef<AbortController | null>(null);
  const recording = useRef<VoiceRecording | null>(null);
  const limitTimer = useRef<ReturnType<typeof setTimeout> | undefined>(undefined);
  const callback = useRef(onTranscript);
  useLayoutEffect(() => { callback.current = onTranscript; }, [onTranscript]);
  const stopRef = useRef<() => Promise<void>>(async () => {});

  const cancel = useCallback(() => {
    generation.current++;
    clearTimeout(limitTimer.current);
    controller.current?.abort(); controller.current = null;
    recording.current?.cancel(); recording.current = null;
    setPhase('idle'); setError(null);
  }, []);

  useLayoutEffect(() => {
    const onVisibility = () => { if (document.hidden) cancel(); };
    const onPageHide = () => cancel();
    document.addEventListener('visibilitychange', onVisibility);
    window.addEventListener('pagehide', onPageHide);
    return () => {
      cancel();
      document.removeEventListener('visibilitychange', onVisibility);
      window.removeEventListener('pagehide', onPageHide);
    };
  }, [workspaceId, identity, enabled, cancel]);

  const stop = useCallback(async () => {
    const capture = recording.current;
    const request = controller.current;
    if (!capture || !request || !workspaceId) return;
    recording.current = null;
    clearTimeout(limitTimer.current);
    const id = generation.current;
    setPhase('transcribing');
    try {
      const audio = await capture.stop();
      if (id !== generation.current) return;
      const text = await transcribeVoice(workspaceId, audio, request.signal);
      if (id !== generation.current) return;
      if (text.trim()) callback.current(text);
      else setError('No speech detected. Try again closer to your microphone.');
    } catch (cause) {
      if (id !== generation.current) return;
      setError(cause instanceof Error ? cause.message : 'Could not transcribe the recording. Please try again.');
    } finally {
      capture.cancel();
      if (id === generation.current) { controller.current = null; setPhase('idle'); }
    }
  }, [workspaceId]);
  useLayoutEffect(() => { stopRef.current = stop; }, [stop]);

  const start = useCallback(async () => {
    if (!enabled || !workspaceId || controller.current) return;
    const id = ++generation.current;
    const request = new AbortController(); controller.current = request;
    setError(null); setSeconds(0); setLevels(Array(24).fill(0)); setPhase('requesting');
    try {
      const capture = await startVoiceRecording(progress => {
        if (id !== generation.current) return;
        setSeconds(progress.seconds);
        setLevels(values => [...values.slice(1), progress.level]);
        if (progress.seconds >= MAX_VOICE_SECONDS) void stopRef.current();
      }, request.signal);
      if (id !== generation.current) { capture.cancel(); return; }
      recording.current = capture;
      setPhase('recording');
      limitTimer.current = setTimeout(() => { void stopRef.current(); }, MAX_VOICE_SECONDS * 1000);
    } catch (cause) {
      if (id !== generation.current) return;
      request.abort(); controller.current = null; setPhase('idle');
      const name = cause && typeof cause === 'object' && 'name' in cause ? String(cause.name) : '';
      setError(name === 'NotAllowedError' ? 'Allow microphone access in your browser, then try again.'
        : name === 'NotFoundError' ? 'No microphone found. Connect one and try again.'
        : name === 'NotReadableError' ? 'Your microphone is unavailable. Check whether another app is using it.'
        : cause instanceof Error ? cause.message : 'Unable to start recording. Please try again.');
    }
  }, [enabled, workspaceId]);

  return { phase, seconds, levels, error, busy: phase !== 'idle', start, stop, cancel, dismissError: () => setError(null) };
}
