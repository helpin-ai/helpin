import { describe, expect, it } from 'vitest';
import { encodeVoiceWav, appendVoiceTranscript } from '../voiceRecording';

describe('voice recording', () => {
  it('writes bounded mono 16 kHz PCM with a consistent header', async () => {
    const blob = encodeVoiceWav([new Float32Array(48000).fill(0.5)], 48000);
    const buffer = await blob.arrayBuffer();
    const view = new DataView(buffer);
    expect(blob.type).toBe('audio/wav');
    expect(buffer.byteLength).toBe(32044);
    expect(view.getUint32(24, true)).toBe(16000);
    expect(view.getUint32(40, true)).toBe(32000);
    expect(view.getInt16(44, true)).toBe(16383);
  });
  it('preserves the existing draft and separates new speech', () => {
    expect(appendVoiceTranscript('Please investigate', 'the error.')).toBe('Please investigate the error.');
    expect(appendVoiceTranscript('Notes:\n', 'Hello')).toBe('Notes:\nHello');
    expect(appendVoiceTranscript('', ' Hello ')).toBe('Hello');
    expect(appendVoiceTranscript('draft', '  ')).toBe('draft');
  });
});

it('allows five minutes and bounds longer input to the upload limit', async () => {
  const audio = encodeVoiceWav([new Float32Array(16000 * 301)], 16000);
  expect(audio.size).toBe(44 + 300 * 16000 * 2);
});
