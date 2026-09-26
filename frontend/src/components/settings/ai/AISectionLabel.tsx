import type { ReactNode } from "react";

/**
 * Skill Catalog group label: uppercase name, a count, and an optional action
 * aligned to the right of the row.
 */
export function AISectionLabel({
  label,
  count,
  action,
}: {
  label: string;
  count?: number;
  action?: ReactNode;
}) {
  return (
    <div className="mb-2.5 flex items-baseline gap-1.5 px-0.5 text-[10.5px] font-medium uppercase tracking-[0.08em] text-muted-foreground/80">
      <span>{label}</span>
      {typeof count === "number" && (
        <>
          <span aria-hidden className="text-muted-foreground/40">
            ·
          </span>
          <span className="tabular-nums text-muted-foreground/60">{count}</span>
        </>
      )}
      {action && <span className="ml-auto normal-case tracking-normal">{action}</span>}
    </div>
  );
}
