import type { StateType } from '@/lib/pmTypes';
import { StateSelectContent } from '@/components/design-system/state-select-content';
interface TaskStateSelectContentProps {
  stateType: StateType;
  label: string;
  color?: string | null;
  autoRunEnabled?: boolean;
}
export function TaskStateSelectContent({ label, color, autoRunEnabled }: TaskStateSelectContentProps) {
  return <StateSelectContent label={label} color={color} autoRunEnabled={autoRunEnabled} />;
}
