// @vitest-environment jsdom
import { act, createRef } from 'react';
import { createRoot, type Root } from 'react-dom/client';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import { MeetingRecordingPlayer, type MeetingRecordingPlayerHandle } from '../MeetingRecordingPlayer';
import { crmMeetingService } from '@/lib/services/crmMeetingService';

(globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true;

let container: HTMLDivElement;
let root: Root;

beforeEach(() => {
  container = document.createElement('div');
  document.body.appendChild(container);
  root = createRoot(container);
  Element.prototype.scrollIntoView = vi.fn();
});

afterEach(() => {
  act(() => root.unmount());
  container.remove();
  vi.restoreAllMocks();
});

describe('MeetingRecordingPlayer', () => {
  it('renders mixed video and seeks through its imperative playback handle', async () => {
    vi.spyOn(crmMeetingService, 'getRecording').mockResolvedValue({
      data: { url: 'https://signed.example/meeting.mp4', content_type: 'video/mp4', media_type: 'video' },
      error: null,
    });
    const play = vi.spyOn(HTMLMediaElement.prototype, 'play').mockResolvedValue();
    const playerRef = createRef<MeetingRecordingPlayerHandle>();

    await act(async () => {
      root.render(<MeetingRecordingPlayer ref={playerRef} workspaceId="workspace-1" meetingId="meeting-1" />);
    });

    const video = container.querySelector<HTMLVideoElement>('video');
    expect(video?.getAttribute('src')).toBe('https://signed.example/meeting.mp4');
    expect(video?.controls).toBe(true);
    expect(video?.preload).toBe('metadata');

    act(() => playerRef.current?.seekTo(42.5));
    expect(video?.currentTime).toBe(42.5);
    expect(play).toHaveBeenCalled();
  });

  it('renders legacy audio recordings with native controls', async () => {
    vi.spyOn(crmMeetingService, 'getRecording').mockResolvedValue({
      data: { url: 'https://signed.example/meeting.mp3', content_type: 'audio/mpeg', media_type: 'audio' },
      error: null,
    });

    await act(async () => {
      root.render(<MeetingRecordingPlayer workspaceId="workspace-1" meetingId="meeting-2" />);
    });

    const audio = container.querySelector<HTMLAudioElement>('audio');
    expect(audio?.getAttribute('src')).toBe('https://signed.example/meeting.mp3');
    expect(audio?.controls).toBe(true);
  });
});
