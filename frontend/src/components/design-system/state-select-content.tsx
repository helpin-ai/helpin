/** Shared state/stage option content used by tasks and CRM. */
export function StateSelectContent({
  label,
  color,
  autoRunEnabled = false,
}: {
  label: string;
  color?: string | null;
  autoRunEnabled?: boolean;
}) {
  return (
    <span className="inline-flex min-w-0 items-center gap-1.5">
      <span
        data-testid="state-color-dot"
        className="h-3 w-3 shrink-0 rounded-full border border-border/50"
        style={color ? { backgroundColor: color } : undefined}
      />
      <span className="truncate">{label}</span>
      {autoRunEnabled && (
        <span className="shrink-0 rounded-full border border-primary/20 bg-primary/10 px-1.5 py-0.5 text-[10px] font-medium leading-none text-primary">
          Auto-run
        </span>
      )}
    </span>
  );
}
