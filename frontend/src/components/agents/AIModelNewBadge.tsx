import { Badge } from "@/components/ui/badge";
import { QuickTooltip } from "@/components/ui/quick-tooltip";

export function AIModelNewBadge() {
  return (
    <QuickTooltip label="Discovered for this connection in the last 7 days. This may differ from the provider’s release date.">
      <Badge variant="secondary" tabIndex={0}>New</Badge>
    </QuickTooltip>
  );
}
