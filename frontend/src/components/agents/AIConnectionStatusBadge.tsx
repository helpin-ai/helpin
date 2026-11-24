import { Badge } from "@/components/ui/badge";
import { connectionStatusInfo, type ConnectionStatusTone } from "@/lib/aiProviders";
import { cn } from "@/lib/utils";

const toneClasses: Record<ConnectionStatusTone, string> = {
  positive:
    "border-emerald-300/70 bg-emerald-50 text-emerald-700 dark:border-emerald-900 dark:bg-emerald-950/30 dark:text-emerald-300",
  attention:
    "border-amber-300/70 bg-amber-50 text-amber-700 dark:border-amber-900 dark:bg-amber-950/30 dark:text-amber-300",
  neutral: "border-border/70 bg-muted/30 text-muted-foreground",
};

export function AIConnectionStatusBadge({
  status,
  verificationError,
  className,
}: {
  status: string;
  verificationError?: string;
  className?: string;
}) {
  const info = connectionStatusInfo(status, verificationError);
  return (
    <Badge title={status === "connected" ? verificationError || "Credentials configured. Model availability depends on the provider." : undefined} variant="outline" className={cn("shrink-0 text-[10px]", toneClasses[info.tone], className)}>
      <span aria-hidden="true" className="size-1.5 rounded-full bg-current" />
      {info.label}
    </Badge>
  );
}
