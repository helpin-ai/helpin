export const MAX_VOICE_SECONDS = 300;
export interface VoiceRecording {
  stop(): Promise<Blob>;
  cancel(): void;
}
export interface VoiceProgress { seconds: number; level: number }

export function appendVoiceTranscript(draft: string, transcript: string): string {
  const text = transcript.trim();
  if (!text) return draft;
  return `${draft}${draft && !/\s$/.test(draft) ? ' ' : ''}${text}`;
}

export function encodeVoiceWav(chunks: Float32Array[], sampleRate: number): Blob {
  const length = Math.min(chunks.reduce((n, c) => n + c.length, 0), sampleRate * MAX_VOICE_SECONDS);
  const samples = new Float32Array(length);
  let offset = 0;
  for (const chunk of chunks) {
    const part = chunk.subarray(0, length - offset);
    samples.set(part, offset); offset += part.length;
    if (offset >= length) break;
  }
  const count = Math.floor(length * 16000 / sampleRate);
  const buffer = new ArrayBuffer(44 + count * 2);
  const view = new DataView(buffer);
  const text = (start: number, value: string) => [...value].forEach((c, i) => view.setUint8(start + i, c.charCodeAt(0)));
  text(0, 'RIFF'); view.setUint32(4, buffer.byteLength - 8, true); text(8, 'WAVE');
  text(12, 'fmt '); view.setUint32(16, 16, true); view.setUint16(20, 1, true);
  view.setUint16(22, 1, true); view.setUint32(24, 16000, true); view.setUint32(28, 32000, true);
  view.setUint16(32, 2, true); view.setUint16(34, 16, true); text(36, 'data'); view.setUint32(40, count * 2, true);
  const ratio = sampleRate / 16000;
  for (let i = 0; i < count; i++) {
    const start = Math.floor(i * ratio);
    const end = Math.max(start + 1, Math.floor((i + 1) * ratio));
    let sum = 0;
    for (let j = start; j < end; j++) sum += samples[j] ?? 0;
    const value = Math.max(-1, Math.min(1, sum / (end - start)));
    view.setInt16(44 + i * 2, value < 0 ? value * 32768 : value * 32767, true);
  }
  return new Blob([buffer], { type: 'audio/wav' });
}

export async function startVoiceRecording(onProgress: (progress: VoiceProgress) => void, signal: AbortSignal): Promise<VoiceRecording> {
  if (!window.isSecureContext) throw new Error('Microphone access requires HTTPS or localhost.');
  if (!navigator.mediaDevices?.getUserMedia || !window.AudioContext || !window.AudioWorkletNode) {
    throw new Error('Voice input is not supported in this browser.');
  }
  const stream = await navigator.mediaDevices.getUserMedia({ audio: { channelCount: 1, echoCancellation: true, noiseSuppression: true } });
  const stopTracks = () => stream.getTracks().forEach(track => track.stop());
  if (signal.aborted) { stopTracks(); throw new DOMException('Cancelled', 'AbortError'); }
  let context: AudioContext | undefined;
  let node: AudioWorkletNode | undefined;
  let closed = false;
  let chunks: Float32Array[] = [];
  let count = 0;
  const cancel = () => {
    if (closed) return;
    closed = true;
    signal.removeEventListener('abort', cancel);
    stopTracks(); node?.disconnect(); node?.port.close();
    if (context && context.state !== 'closed') void context.close().catch(() => {});
  };
  signal.addEventListener('abort', cancel, { once: true });
  try {
    context = new AudioContext({ sampleRate: 16000 });
    await context.audioWorklet.addModule(new URL('./voiceRecorder.worklet.js?no-inline', import.meta.url).href);
    if (signal.aborted) throw new DOMException('Cancelled', 'AbortError');
    node = new AudioWorkletNode(context, 'helpin-voice-recorder');
    const rate = context.sampleRate;
    let stopped: (() => void) | undefined;
    node.port.onmessage = ({ data }: MessageEvent<{ samples?: Float32Array; stopped?: boolean }>) => {
      if (closed) return;
      if (data.stopped) { stopped?.(); return; }
      if (!data.samples) return;
      const samples = data.samples.subarray(0, Math.max(0, rate * MAX_VOICE_SECONDS - count));
      if (!samples.length) return;
      chunks.push(samples); count += samples.length;
      const energy = samples.reduce((sum, sample) => sum + sample * sample, 0) / samples.length;
      onProgress({ seconds: count / rate, level: Math.min(1, Math.sqrt(energy) * 5) });
    };
    context.createMediaStreamSource(stream).connect(node);
    // The processor emits silence; connecting it keeps capture scheduled without playback.
    node.connect(context.destination);
    await context.resume();
    if (signal.aborted) throw new DOMException('Cancelled', 'AbortError');
    return {
      cancel() { cancel(); chunks = []; },
      async stop() {
        if (closed) throw new DOMException('Cancelled', 'AbortError');
        let timer: ReturnType<typeof setTimeout> | undefined;
        await new Promise<void>(resolve => {
          stopped = resolve;
          timer = setTimeout(resolve, 500);
          node?.port.postMessage('stop');
        });
        clearTimeout(timer);
        if (closed) throw new DOMException('Cancelled', 'AbortError');
        cancel();
        if (count < rate / 4) throw new Error('Recording was too short. Try speaking for a little longer.');
        const wav = encodeVoiceWav(chunks, rate); chunks = [];
        return wav;
      },
    };
  } catch (error) { cancel(); throw error; }
}
