import type { StateType } from '@/lib/pmTypes';

interface StoryStateSelectContentProps {
  stateType: StateType;
  label: string;
  color?: string | null;
}

export function StoryStateSelectContent({
  stateType: _stateType,
  label,
  color,
}: StoryStateSelectContentProps) {
  return (
    <span className="inline-flex min-w-0 items-center gap-1.5">
      <span
        data-testid="state-color-dot"
        className="h-3 w-3 shrink-0 rounded-full border border-border/50"
        style={color ? { backgroundColor: color } : undefined}
      />
      <span className="truncate">{label}</span>
    </span>
  );
}
