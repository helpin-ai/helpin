import { cn } from '@/lib/utils';

export const PRESET_COLORS = [
  '#3b82f6', // blue
  '#16a34a', // green
  '#ec4899', // pink
  '#64748b', // slate
  '#ef4444', // red
  '#f97316', // orange
  '#eab308', // yellow
  '#14b8a6', // teal
  '#8b5cf6', // violet
  '#6366f1', // indigo
  '#06b6d4', // cyan
  '#d946ef', // fuchsia
  '#84cc16', // lime
  '#f43f5e', // rose
  '#0ea5e9', // sky
  '#a855f7', // purple
];

export function ColorPicker({
  value,
  onChange,
}: {
  value: string;
  onChange: (color: string) => void;
}) {
  return (
    <div className="flex flex-wrap gap-1.5">
      {PRESET_COLORS.map((c) => (
        <button
          key={c}
          type="button"
          className={cn(
            'h-6 w-6 rounded-full border-2 transition-all cursor-pointer',
            value === c
              ? 'border-foreground scale-110'
              : 'border-transparent hover:border-muted-foreground/40',
          )}
          style={{ backgroundColor: c }}
          onClick={() => onChange(c)}
        />
      ))}
    </div>
  );
}
