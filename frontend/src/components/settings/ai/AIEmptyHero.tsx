import type { ComponentType, ReactNode } from "react";

/** Dashed hero used for the empty states on the AI settings pages. */
export function AIEmptyHero({
  icon: Icon,
  title,
  description,
  action,
}: {
  icon: ComponentType<{ className?: string }>;
  title: string;
  description: ReactNode;
  action?: ReactNode;
}) {
  return (
    <div className="rounded-xl border border-dashed border-border/80 bg-card px-6 py-12 text-center">
      <span className="mx-auto flex h-11 w-11 items-center justify-center rounded-full bg-muted/50">
        <Icon className="h-5 w-5 text-muted-foreground" />
      </span>
      <p className="mt-3 text-sm font-medium text-foreground">{title}</p>
      <p className="mx-auto mt-1.5 max-w-sm text-[13px] leading-snug text-muted-foreground">
        {description}
      </p>
      {action && <div className="mt-4 flex justify-center">{action}</div>}
    </div>
  );
}
