import { useState } from "react";
import { Button } from "@/components/ui/button";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/design-system/quiet-dropdown-select";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from "@/components/ui/tooltip";
import type { DocsCollection, DocsSpace } from "@/lib/docsTypes";
import type {
  CoverageSuggestionReview,
  SupportCoverageGapDetail,
  SupportCoverageGapMergeSuggestion,
} from "@/lib/supportCoverageTypes";
import { GAP_STATUS_LABELS } from "@/lib/supportCoverageTypes";
import { Cancel01Icon, CheckmarkCircle02Icon, Loading01Icon, MoreHorizontalIcon } from "@/lib/icons";
import { timeAgo } from "@/lib/utils";
import { GapAskAgentDock } from "./GapAskAgentDock";
import { GapProposalReview } from "./GapProposalReview";
import { buildCoverageCollectionOptions } from "./coverageCollectionOptions";
import { QuietStatusBadge } from "@/components/design-system/quiet";

export interface GapDetailPaneProps {
  gap: SupportCoverageGapDetail;
  wsSlug: string;
  loading: boolean;
  canGenerate: boolean;
  canEdit: boolean;
  statusUpdating: boolean;
  externalSpaces: DocsSpace[];
  collections: DocsCollection[] | undefined;
  targetSpaceId: string;
  targetCollectionId: string;
  generating: boolean;
  generateError: string | null;
  applying: boolean;
  mergeSuggestions: SupportCoverageGapMergeSuggestion[];
  canReviewMergeSuggestions: boolean;
  mergeActionSuggestionId: string | null;
  onClose: () => void;
  onTargetSpaceChange: (id: string) => void;
  onTargetCollectionChange: (id: string) => void;
  onSuggestImprovements: () => void;
  onDraftNewArticle: () => void;
  onApplySuggestion: (
    id: string,
    review?: CoverageSuggestionReview,
  ) => Promise<string | null>;
  onDiscardSuggestion: (id: string) => void;
  onApplyMergeSuggestion: (id: string) => void;
  onDismissMergeSuggestion: (id: string) => void;
  onStatusUpdate: (id: string, status: string) => void;
  onRegenerate: (id: string) => void;
  onRefresh: () => void;
}

export function GapDetailPane(props: GapDetailPaneProps) {
  const {
    gap,
    wsSlug,
    canGenerate,
    canEdit,
    statusUpdating,
    generating,
    applying,
  } = props;
  const [quickDraftOpen, setQuickDraftOpen] = useState(false);
  const [regenerating, setRegenerating] = useState(false);
  const draft = gap.suggestions.find(
    (suggestion) =>
      suggestion.status === "draft" &&
      suggestion.is_active !== false &&
      !suggestion.superseded_at,
  );
  const applied = gap.suggestions.find(
    (suggestion) => suggestion.status === "applied",
  );
  const savedDraft =
    gap.status === "open" && !gap.recurrence_reopened ? applied : undefined;
  const contentGap = !["data", "action", "policy"].includes(gap.gap_kind);
  const canQuickDraft =
    canGenerate && gap.status === "open" && contentGap && !draft;
  const quickUpdate =
    gap.related_articles.length > 0 &&
    ["weak_article", "outdated_or_conflicting_article"].includes(
      gap.v1_gap_type,
    );
  const pendingMerges = props.mergeSuggestions.filter(
    (suggestion) => suggestion.status === "pending",
  );
  const collectionOptions = buildCoverageCollectionOptions(
    props.targetSpaceId,
    props.collections ?? [],
  );
  const regenerate = () => {
    setRegenerating(true);
    props.onRegenerate(gap.id);
    window.setTimeout(() => setRegenerating(false), 30_000);
  };
  return (
    <div className="min-h-full min-w-0 bg-card text-foreground">
      <header className="sticky top-0 z-10 flex h-14 items-center justify-between gap-3 border-b border-border/50 bg-card px-5 sm:px-7">
        <div className="flex min-w-0 flex-1 items-center gap-2">
          <Tooltip>
            <TooltipTrigger asChild>
              <h3 tabIndex={0} className="min-w-0 truncate text-sm font-semibold focus-visible:outline-2 focus-visible:outline-ring">
                {gap.title}
              </h3>
            </TooltipTrigger>
            <TooltipContent className="max-w-xs break-words">{gap.title}</TooltipContent>
          </Tooltip>
          <QuietStatusBadge tone={gap.status === "done" ? "positive" : "neutral"} className="shrink-0">
            {GAP_STATUS_LABELS[gap.status] ?? gap.status}
          </QuietStatusBadge>
        </div>
        <div className="flex shrink-0 items-center gap-1">
          {canEdit && (gap.status === "open" ? (
            <Tooltip>
              <TooltipTrigger asChild>
                <Button variant="ghost" size="sm" aria-label="Mark resolved"
                  className="text-muted-foreground"
                  disabled={statusUpdating || generating || applying}
                  onClick={() => props.onStatusUpdate(gap.id, "done")}>
                  <CheckmarkCircle02Icon className="h-4 w-4" />
                  <span className="hidden sm:inline">Mark resolved</span>
                </Button>
              </TooltipTrigger>
              <TooltipContent>Mark resolved once the fix is available to the AI and answers the customer need.</TooltipContent>
            </Tooltip>
          ) : (
            <Button variant="ghost" size="sm" disabled={statusUpdating}
              onClick={() => props.onStatusUpdate(gap.id, "open")}>Reopen gap</Button>
          ))}
          {props.loading && (
            <Loading01Icon
              aria-label="Refreshing gap"
              className="mr-2 h-4 w-4 animate-spin text-muted-foreground"
            />
          )}
          <DropdownMenu>
            <DropdownMenuTrigger asChild>
              <Button
                variant="ghost"
                size="icon-sm"
                aria-label="Gap options"
                className="text-muted-foreground"
              >
                <MoreHorizontalIcon />
              </Button>
            </DropdownMenuTrigger>
            <DropdownMenuContent align="end">
              <DropdownMenuItem onClick={props.onRefresh}>
                Refresh gap
              </DropdownMenuItem>
              {canQuickDraft && (
                <DropdownMenuItem
                  disabled={generating}
                  onClick={() => setQuickDraftOpen(true)}
                >
                  Generate a quick draft
                </DropdownMenuItem>
              )}
              {canEdit && gap.status === "open" && (
                <DropdownMenuItem disabled={regenerating} onClick={regenerate}>
                  Refresh analysis{regenerating ? " (queued)" : ""}
                </DropdownMenuItem>
              )}
              {canEdit && (
                <>
                  <DropdownMenuSeparator />
                  {gap.status === "open" && (
                    <DropdownMenuItem
                      disabled={statusUpdating || generating || applying}
                      onClick={() => props.onStatusUpdate(gap.id, "rejected")}
                    >
                      Dismiss gap
                    </DropdownMenuItem>
                  )}
                </>
              )}
            </DropdownMenuContent>
          </DropdownMenu>
          <Button
            variant="ghost"
            size="icon-sm"
            aria-label="Close gap details"
            className="text-muted-foreground"
            onClick={props.onClose}
          >
            <Cancel01Icon />
          </Button>
        </div>
      </header>

      {(gap.split_review_needed || gap.recurrence_reopened || props.generateError || generating || (savedDraft && !draft) || (gap.status !== "open" && gap.status_changed_at)) && <div className="space-y-4 px-5 py-4 sm:px-7">
        {gap.split_review_needed && (
          <p className="text-xs text-amber-700 dark:text-amber-400">
            These sources may describe separate needs. Review them before
            combining the fix.
          </p>
        )}
        {gap.recurrence_reopened && (
          <p className="text-xs text-muted-foreground">
            New evidence arrived after the previous fix, so this gap was
            reopened.
          </p>
        )}
        {props.generateError && (
          <p role="alert" className="text-sm text-destructive">
            {props.generateError}
          </p>
        )}
        {generating && (
          <p
            role="status"
            className="flex items-center gap-2 text-sm text-muted-foreground"
          >
            <Loading01Icon className="h-4 w-4 animate-spin" />
            Preparing your draft…
          </p>
        )}
        {savedDraft && !draft && (
          <div className="space-y-2 border-l-2 border-border pl-3">
            <p className="text-sm font-medium">Draft saved for review</p>
            <p className="text-xs leading-5 text-muted-foreground">
              Review and publish the guidance, then check that the AI can use
              the fix before marking this gap resolved.
            </p>
            {savedDraft.result_document_id && (
              <a
                className="inline-block text-sm underline underline-offset-4"
                href={`/w/${wsSlug}/docs/documents/${savedDraft.result_document_id}`}
                target="_blank"
                rel="noopener noreferrer"
              >
                Review saved draft
              </a>
            )}
          </div>
        )}
        {gap.status !== "open" && gap.status_changed_at && (
          <p className="text-xs text-muted-foreground">
            {GAP_STATUS_LABELS[gap.status]}
            {gap.status_changed_by_name
              ? ` by ${gap.status_changed_by_name}`
              : ""}{" "}
            {timeAgo(gap.status_changed_at)}.
          </p>
        )}
      </div>}

      {draft && (
        <GapProposalReview
          key={draft.id}
          gap={gap}
          suggestion={draft}
          wsSlug={wsSlug}
          spaces={props.externalSpaces}
          collections={props.collections}
          targetSpaceId={props.targetSpaceId}
          targetCollectionId={props.targetCollectionId}
          canEdit={canGenerate && gap.status === "open"}
          applying={applying}
          onSpaceChange={props.onTargetSpaceChange}
          onCollectionChange={props.onTargetCollectionChange}
          onApply={props.onApplySuggestion}
          onDiscard={props.onDiscardSuggestion}
        />
      )}

      <GapAskAgentDock gap={gap} canEdit={canEdit} onChanged={props.onRefresh} />

      {quickDraftOpen && canQuickDraft && (
        <section
          aria-label="Quick draft"
          className="space-y-3 border-t border-border/50 px-5 py-4 sm:px-7"
        >
          <div className="flex items-center justify-between">
            <h4 className="text-sm font-semibold">Quick draft</h4>
            <Button
              variant="ghost"
              size="sm"
              onClick={() => setQuickDraftOpen(false)}
            >
              Cancel
            </Button>
          </div>
          <p className="text-xs leading-5 text-muted-foreground">
            Uses the current sources without an assistant investigation.
            {quickUpdate
              ? " Only new sections are added; use Quill to correct existing guidance."
              : ""}
          </p>
          {!quickUpdate && (
            <div className="flex flex-col gap-2 sm:flex-row">
              <Select
                value={props.targetSpaceId}
                onValueChange={props.onTargetSpaceChange}
              >
                <SelectTrigger
                  aria-label="Quick draft docs space"
                  className="w-full"
                >
                  <SelectValue placeholder="Choose docs space" />
                </SelectTrigger>
                <SelectContent>
                  {props.externalSpaces.map((space) => (
                    <SelectItem key={space.id} value={space.id}>
                      {space.name}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
              <Select
                value={props.targetCollectionId || "_root"}
                onValueChange={(value) =>
                  props.onTargetCollectionChange(value === "_root" ? "" : value)
                }
                disabled={!props.targetSpaceId}
              >
                <SelectTrigger
                  aria-label="Quick draft collection"
                  className="w-full"
                >
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="_root">Space root</SelectItem>
                  {collectionOptions.map((collection) => (
                    <SelectItem key={collection.id} value={collection.id}>
                      {collection.label}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>
          )}
          <Button
            variant="outline"
            size="sm"
            disabled={generating || (!quickUpdate && !props.targetSpaceId)}
            onClick={
              quickUpdate
                ? props.onSuggestImprovements
                : props.onDraftNewArticle
            }
          >
            Generate draft
          </Button>
        </section>
      )}

      {pendingMerges.length > 0 && (
        <details className="border-t border-border/50 px-5 py-4 sm:px-7">
          <summary className="cursor-pointer text-sm font-medium focus-visible:outline-2 focus-visible:outline-ring">
            Similar gaps ({pendingMerges.length})
          </summary>
          <div className="mt-4 divide-y divide-border/40">
            {pendingMerges.map((suggestion) => {
              const other =
                suggestion.source_gap_id === gap.id
                  ? suggestion.target_gap
                  : suggestion.source_gap;
              const busy = props.mergeActionSuggestionId === suggestion.id;
              return (
                <div key={suggestion.id} className="space-y-2 py-3">
                  <p className="text-sm font-medium">
                    {other?.title || "Related gap"}
                  </p>
                  <p className="text-xs text-muted-foreground">
                    {Math.round(suggestion.similarity_score * 100)}% match ·{" "}
                    {suggestion.combined_evidence_count} evidence records after
                    merge
                  </p>
                  {suggestion.reason && (
                    <p className="text-xs leading-5 text-muted-foreground">
                      {suggestion.reason}
                    </p>
                  )}
                  {props.canReviewMergeSuggestions && (
                    <div className="flex gap-2">
                      <Button
                        variant="outline"
                        size="sm"
                        disabled={busy}
                        onClick={() =>
                          props.onApplyMergeSuggestion(suggestion.id)
                        }
                      >
                        Merge
                      </Button>
                      <Button
                        variant="ghost"
                        size="sm"
                        disabled={busy}
                        onClick={() =>
                          props.onDismissMergeSuggestion(suggestion.id)
                        }
                      >
                        Keep separate
                      </Button>
                    </div>
                  )}
                </div>
              );
            })}
          </div>
        </details>
      )}

    </div>
  );
}
