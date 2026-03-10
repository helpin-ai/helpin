import { Circle, CircleCheck, CircleX } from 'lucide-react';
import type { PipelineStageType } from './crmTypes';

export const STAGE_TYPE_CONFIG: Record<
  PipelineStageType,
  { icon: React.ElementType; color: string; label: string }
> = {
  open: { icon: Circle, color: 'text-blue-500', label: 'Open' },
  won: { icon: CircleCheck, color: 'text-green-500', label: 'Won' },
  lost: { icon: CircleX, color: 'text-red-500', label: 'Lost' },
};

export function StageTypeIcon({
  stageType,
  className = 'h-4 w-4',
}: {
  stageType: PipelineStageType;
  className?: string;
}) {
  const config = STAGE_TYPE_CONFIG[stageType];
  const Icon = config.icon;
  return <Icon className={`${className} ${config.color}`} />;
}
