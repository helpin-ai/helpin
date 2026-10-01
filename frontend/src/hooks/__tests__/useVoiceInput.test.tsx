// @vitest-environment jsdom
import { act } from 'react';
import { createRoot, type Root } from 'react-dom/client';
import { afterEach, beforeEach, expect, it, vi } from 'vitest';
import { useVoiceInput } from '../useVoiceInput';
import { startVoiceRecording } from '@/lib/voiceRecording';
import { transcribeVoice } from '@/lib/services/voiceInputService';

vi.mock('@/lib/voiceRecording', () => ({ MAX_VOICE_SECONDS: 300, startVoiceRecording: vi.fn() }));
vi.mock('@/lib/services/voiceInputService', () => ({ transcribeVoice: vi.fn() }));
let root: Root;
let container: HTMLDivElement;
let voice: ReturnType<typeof useVoiceInput>;
const onTranscript = vi.fn();
function Probe({ identity = 'chat1', enabled = true }: {identity?: string; enabled?: boolean}) {
  voice = useVoiceInput({ workspaceId: 'ws', identity, enabled, onTranscript });
  return null;
}
beforeEach(() => {
  vi.clearAllMocks();
  (globalThis as {IS_REACT_ACT_ENVIRONMENT?: boolean}).IS_REACT_ACT_ENVIRONMENT = true;
  container = document.createElement('div'); document.body.append(container); root = createRoot(container);
});
afterEach(() => { act(() => root.unmount()); container.remove(); });
it('returns editable text only after stop, with no automatic submission', async () => {
  const recording = { cancel: vi.fn(), stop: vi.fn().mockResolvedValue(new Blob(['audio'])) };
  vi.mocked(startVoiceRecording).mockResolvedValue(recording);
  vi.mocked(transcribeVoice).mockResolvedValue('Hello');
  act(() => root.render(<Probe />));
  await act(async () => { await voice.start(); });
  expect(voice.phase).toBe('recording'); expect(onTranscript).not.toHaveBeenCalled();
  await act(async () => { await voice.stop(); });
  expect(onTranscript).toHaveBeenCalledTimes(1); expect(onTranscript).toHaveBeenCalledWith('Hello'); expect(voice.phase).toBe('idle');
});
it('discards a late microphone permission result after cancellation', async () => {
  let resolve!: (r: Awaited<ReturnType<typeof startVoiceRecording>>) => void;
  vi.mocked(startVoiceRecording).mockImplementation(() => new Promise(r => { resolve = r; }));
  const recording = { cancel: vi.fn(), stop: vi.fn() };
  act(() => root.render(<Probe />));
  let pending!: Promise<void>;
  act(() => { pending = voice.start(); });
  act(() => voice.cancel());
  await act(async () => { resolve(recording); await pending; });
  expect(recording.cancel).toHaveBeenCalled(); expect(transcribeVoice).not.toHaveBeenCalled();
});
it('discards an in-flight transcript after changing conversations', async () => {
  const recording = { cancel: vi.fn(), stop: vi.fn().mockResolvedValue(new Blob(['audio'])) };
  vi.mocked(startVoiceRecording).mockResolvedValue(recording);
  let resolve!: (text: string) => void;
  vi.mocked(transcribeVoice).mockImplementation(() => new Promise(r => { resolve = r; }));
  act(() => root.render(<Probe />));
  await act(async () => { await voice.start(); });
  let pending!: Promise<void>;
  await act(async () => { pending = voice.stop(); });
  act(() => root.render(<Probe identity="chat2" />));
  await act(async () => { resolve('Wrong chat'); await pending; });
  expect(onTranscript).not.toHaveBeenCalled(); expect(voice.phase).toBe('idle');
});
it('stops recording when the composer becomes inactive', async () => {
  const recording = { cancel: vi.fn(), stop: vi.fn() };
  vi.mocked(startVoiceRecording).mockResolvedValue(recording);
  act(() => root.render(<Probe />));
  await act(async () => { await voice.start(); });
  act(() => root.render(<Probe enabled={false} />));
  expect(recording.cancel).toHaveBeenCalled(); expect(voice.phase).toBe('idle');
});
it('reports microphone denial without uploading', async () => {
  vi.mocked(startVoiceRecording).mockRejectedValue(new DOMException('Permission denied', 'NotAllowedError'));
  act(() => root.render(<Probe />));
  await act(async () => { await voice.start(); });
  expect(voice.phase).toBe('idle'); expect(voice.error).toMatch(/Allow microphone access/);
  expect(transcribeVoice).not.toHaveBeenCalled();
});
it('handles an empty transcript without changing the draft', async () => {
  vi.mocked(startVoiceRecording).mockResolvedValue({ cancel: vi.fn(), stop: vi.fn().mockResolvedValue(new Blob(['audio'])) });
  vi.mocked(transcribeVoice).mockResolvedValue(' ');
  act(() => root.render(<Probe />));
  await act(async () => { await voice.start(); await voice.stop(); });
  expect(onTranscript).not.toHaveBeenCalled(); expect(voice.error).toMatch(/No speech detected/);
});
it('aborts transcription on cancel and ignores a late result', async () => {
  vi.mocked(startVoiceRecording).mockResolvedValue({ cancel: vi.fn(), stop: vi.fn().mockResolvedValue(new Blob(['audio'])) });
  let resolve!: (text: string) => void;
  vi.mocked(transcribeVoice).mockImplementation(() => new Promise(r => { resolve = r; }));
  act(() => root.render(<Probe />));
  await act(async () => { await voice.start(); });
  let pending!: Promise<void>;
  await act(async () => { pending = voice.stop(); });
  const signal = vi.mocked(transcribeVoice).mock.calls[0][2];
  act(() => voice.cancel());
  expect(signal.aborted).toBe(true);
  await act(async () => { resolve('Cancelled'); await pending; });
  expect(onTranscript).not.toHaveBeenCalled();
});
it('automatically stops at the recording limit', async () => {
  const capture = { cancel: vi.fn(), stop: vi.fn().mockResolvedValue(new Blob(['audio'])) };
  vi.mocked(startVoiceRecording).mockResolvedValue(capture);
  vi.mocked(transcribeVoice).mockResolvedValue('Five minutes');
  act(() => root.render(<Probe />));
  await act(async () => { await voice.start(); });
  await act(async () => { vi.mocked(startVoiceRecording).mock.calls[0][0]({ seconds: 120, level: 0.4 }); });
  expect(capture.stop).not.toHaveBeenCalled();
  await act(async () => { vi.mocked(startVoiceRecording).mock.calls[0][0]({ seconds: 300, level: 0.4 }); });
  expect(capture.stop).toHaveBeenCalledTimes(1);
  expect(onTranscript).toHaveBeenCalledWith('Five minutes');
});
