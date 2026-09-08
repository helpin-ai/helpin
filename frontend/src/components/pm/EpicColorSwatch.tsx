import { resolveEpicColor } from './epicColor';

export function EpicColorSwatch({ color }: { color?: string | null }) {
  return (
    <span
      aria-hidden="true"
      className="block h-4 w-4 shrink-0 rounded-[4px] border border-foreground/10"
      style={{ backgroundColor: resolveEpicColor(color) }}
    />
  );
}
