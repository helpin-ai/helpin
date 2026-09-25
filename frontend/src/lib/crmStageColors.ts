import type { PipelineStageType } from './crmTypes';
export function defaultStageColor(
  type: PipelineStageType = 'open',
  position = 0,
) {
  if (type === 'won') return '#45a557';
  if (type === 'lost') return '#e2564a';
  return ['#788596', '#4e8fea', '#c7a53d'][Math.max(0, position) % 3];
}
