
'use client';
import { useState } from 'react';
import { useWorkflowPlayback } from '../../_components/useWorkflowPlayback';
const BEATS = [0, 2600, 5600, 9600];
export function useCRMPlayback(enabled = true) {
  const [paused, setPaused] = useState(false);
  const flow = useWorkflowPlayback({ beats: BEATS, duration: 20000, paused: paused || !enabled });
  return { container: flow.container, active: enabled && flow.playing, phase: enabled ? flow.phase : 3, paused, setPaused };
}
