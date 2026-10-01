import { useMemo } from "react";
import { AskAgentsDock } from "@/components/agents/AskAgentsDock";
import type { SupportCoverageGapDetail } from "@/lib/supportCoverageTypes";
import {
  coverageFindings,
  coverageStarterSuggestions,
} from "./coverageFindings";

export function GapAskAgentDock({
  gap,
  canEdit,
  onChanged,
}: {
  gap: SupportCoverageGapDetail;
  canEdit: boolean;
  onChanged: () => void;
}) {
  const context = useMemo(
    () => ({
      entity_type: "support_coverage_gap" as const,
      entity_id: gap.id,
      display_title: gap.title,
    }),
    [gap.id, gap.title],
  );
  const findings = useMemo(() => coverageFindings(gap), [gap]);
  const closed = gap.status !== "open";
  return (
    <section
      aria-label="Ask Agent about this gap"
      data-coverage-conversation
      className="flex h-[calc(100dvh-3.5rem)] min-h-[360px] flex-col overflow-hidden border-b border-border/50"
    >
      <AskAgentsDock
        presentation="embedded"
        showEmbeddedHeader={false}
        requiredPageContext={context}
        associatedCoverageGapId={gap.id}
        contextMessage={findings}
        starterSuggestions={coverageStarterSuggestions(gap)}
        readOnly={!canEdit || closed}
        readOnlyReason={
          closed
            ? "Reopen this gap to continue the conversation."
            : !canEdit
              ? "You can review this gap. Support edit access is needed to send messages."
              : undefined
        }
        onWorkCompleted={onChanged}
      />
    </section>
  );
}
