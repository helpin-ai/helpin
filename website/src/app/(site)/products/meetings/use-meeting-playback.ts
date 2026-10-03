
'use client';
import { useState } from 'react';
import { useWorkflowPlayback } from '../../_components/useWorkflowPlayback';
const BEATS = [0, 2200, 4800, 8500];
export function useMeetingPlayback(enabled = true) {
  const [paused, setPaused] = useState(false);
  const flow = useWorkflowPlayback({ beats: BEATS, duration: 18000, paused: paused || !enabled });
  return { container: flow.container, active: enabled && flow.playing, phase: enabled ? flow.phase : 3, paused, setPaused };
}
