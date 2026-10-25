import { Link } from "@tanstack/react-router";
import { QuietTextAction } from "@/components/design-system/quiet";
import type { SupportCoverageGapDetail } from "@/lib/supportCoverageTypes";
import { coverageDiagnosis } from "./coverageUi";

/**
 * Data gaps mean the AI could not see something it needed. The fix is a
 * connected tool; dismissing the gap records that the team won't add one.
 */
export function GapDataSourceNotice({ gap, wsSlug }: { gap: SupportCoverageGapDetail; wsSlug: string }) {
  if (gap.gap_kind !== "data" || gap.status !== "open") return null;
  return (
    <div className="flex flex-wrap items-center gap-x-3 gap-y-1 border-b border-border/50 px-5 py-3 text-sm sm:px-7">
      <span className="text-muted-foreground">{coverageDiagnosis(gap)}</span>
      <QuietTextAction asChild className="underline underline-offset-4">
        <Link to="/w/$slug/settings/external-mcp" params={{ slug: wsSlug }}>Connect a tool</Link>
      </QuietTextAction>
    </div>
  );
}
