import { useMemo, useState } from "react";
import { useNavigate } from "@tanstack/react-router";
import { Badge } from "@/components/ui/badge";
import { Checkbox } from "@/components/ui/checkbox";
import { Button } from "@/components/ui/button";
import { QueryBuilderPopover } from "@/components/ui/query-builder/QueryBuilderPopover";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { useCompanies, useSignalWorkspaceFeed } from "@/hooks/queries/useCRM";
import { useWorkspaceMembers } from "@/hooks/queries";
import { Activity01Icon, ArrowRight01Icon, Layers01Icon } from "@/lib/icons";
import type {
  CRMSignal,
  CRMSignalAccountStory,
  CRMSignalFeedFilters,
  CRMSignalSeverity,
  CRMCommercialMotion,
} from "@/lib/crmTypes";
import { cn, timeAgo } from "@/lib/utils";
import { buildCRMSignalQueryFields } from "@/lib/crmSignalQueryBuilder";
import {
  serializeQueryFilterGroup,
  type QueryFilterGroup,
} from "@/lib/queryBuilder";
import { useWorkspaceStore } from "@/stores/workspaceStore";

const domainLabels: Record<string, string> = {
  conversation: "Conversation",
  web_behavior: "Web",
  product_usage: "Product",
  support: "Support",
  delivery: "Delivery",
  relationship: "Relationship",
  market: "Market",
};

const motionLabels: Record<CRMCommercialMotion, string> = {
  prospecting: "Prospecting",
  conversion: "Conversion",
  onboarding: "Onboarding",
  adoption: "Adoption",
  expansion: "Expansion",
  renewal: "Renewal",
  retention: "Retention",
  needs_context: "Needs customer context",
};

function motionLabel(motion: CRMCommercialMotion | string) {
  return (
    motionLabels[motion as CRMCommercialMotion] ??
    motion.replaceAll("_", " ").replace(/^./, (letter) => letter.toUpperCase())
  );
}

const selectClassName =
  "h-8 min-w-[128px] border-border/70 bg-transparent text-xs shadow-none";

function severityClass(severity: CRMSignalSeverity) {
  if (severity === "high")
    return "bg-orange-500/10 text-orange-800 dark:text-orange-300";
  if (severity === "medium")
    return "bg-amber-500/10 text-amber-800 dark:text-amber-300";
  return "bg-muted text-muted-foreground";
}

function impactLabel(story: CRMSignalAccountStory) {
  if (story.polarity === "positive") return "Momentum";
  if (story.polarity === "negative") return "Risk";
  return "Context";
}

function priorityLabel(story: CRMSignalAccountStory) {
  if (story.severity === "high") return "Act now";
  if (story.severity === "medium") return "Review soon";
  return "Monitor";
}

function entityLabel(story: CRMSignalAccountStory) {
  if (story.entity_type === "company") return "company";
  if (story.entity_type === "deal") return "deal";
  if (story.entity_type === "contact") return "contact";
  return "evidence";
}

function recommendedNextStep(story: CRMSignalAccountStory) {
  if (story.needs_judgment) {
    return "Opposing evidence is close. Review both directions before choosing a next step.";
  }
  return (
    story.recommended_action_label ??
    "Review the evidence and choose a concrete follow-up."
  );
}

function storyEvidenceNoun(story: CRMSignalAccountStory) {
  const usesSupportConversations =
    story.signals.length > 0 &&
    story.signals.every(
      (signal) =>
        signal.source_type === "support" &&
        Boolean(signal.source_thread_id || signal.source_id),
    );
  return usesSupportConversations ? "conversation" : "source";
}

function StoryRow({
  story,
  ownerName,
  onOpen,
  onOpenSignal,
  canOpenSignal,
}: {
  story: CRMSignalAccountStory;
  ownerName?: string;
  onOpen?: () => void;
  onOpenSignal?: (signal: CRMSignal) => void;
  canOpenSignal?: (signal: CRMSignal) => boolean;
}) {
  const verifiedSources = story.signals.filter(
    (signal) => signal.evidence_identity_trust === "verified",
  ).length;
  const evidenceNoun = storyEvidenceNoun(story);
  return (
    <article className="border-t border-border/55 first:border-t-0">
      <div className="grid gap-4 px-1 py-5 md:grid-cols-[minmax(0,1fr)_180px]">
        <div className="min-w-0">
          <div className="flex flex-wrap items-center gap-2">
            <h3 className="truncate text-[15px] font-semibold tracking-[-0.012em]">
              {story.account_name}
            </h3>
            {story.account_domain ? (
              <span className="text-xs text-muted-foreground">
                {story.account_domain}
              </span>
            ) : null}
            <Badge
              variant="secondary"
              className={cn(
                "rounded-sm border-0 text-[10px] font-semibold uppercase tracking-[0.06em]",
                severityClass(story.severity),
              )}
            >
              {priorityLabel(story)}
            </Badge>
            <Badge variant="outline" className="rounded-sm text-[10px]">
              {motionLabel(story.commercial_motion)}
            </Badge>
            {story.signals.some((signal) => signal.metadata?.needs_customer_context) &&
            story.commercial_motion !== "needs_context" ? (
              <Badge variant="outline" className="rounded-sm text-[10px]">
                Confirm customer relationship
              </Badge>
            ) : null}
            {story.other_active_motions?.map((motion) => (
              <Badge
                key={motion}
                variant="outline"
                className="rounded-sm text-[10px] text-muted-foreground"
              >
                Also {motionLabel(motion).toLowerCase()}
              </Badge>
            ))}
          </div>
          <p className="mt-1 text-xs text-muted-foreground">
            {story.change_summary} · {impactLabel(story)}
            {ownerName ? ` · ${ownerName}` : ""}
          </p>
          <div className="mt-3 max-w-2xl">
            <p className="text-[10px] font-semibold uppercase tracking-[0.08em] text-muted-foreground">
              Why now
            </p>
            <p className="mt-1 text-[13px] leading-5 text-foreground/90">
              {story.signals[0]?.metadata?.commercial_consequence ||
                story.signals[0]?.summary ||
                "New customer evidence needs review."}
            </p>
          </div>
          <div className="mt-3 rounded-md border border-border/60 bg-muted/20 px-3 py-2.5">
            <p className="text-[10px] font-semibold uppercase tracking-[0.08em] text-muted-foreground">
              Recommended next step
            </p>
            <p className="mt-1 text-xs leading-5 text-foreground/80">
              {recommendedNextStep(story)}
            </p>
            {story.direction_changed_by_supersession ? (
              <p className="mt-1 text-[11px] text-muted-foreground">
                Direction changed because older evidence was superseded, not
                because new evidence arrived.
              </p>
            ) : null}
          </div>
          <div className="mt-3 flex flex-wrap gap-1.5">
            {story.domains.map((domain) => (
              <span
                key={domain}
                className="border border-border/60 px-2 py-0.5 text-[11px] text-muted-foreground"
              >
                {domainLabels[domain] ?? domain}
              </span>
            ))}
          </div>
        </div>
        <div className="flex items-start justify-between gap-5 md:block md:text-right">
          <div className="text-xs text-muted-foreground">
            <p>
              {story.evidence_source_count} {evidenceNoun}
              {story.evidence_source_count === 1 ? "" : "s"}
            </p>
            <p className="mt-1">
              {story.signals.length} signal
              {story.signals.length === 1 ? "" : "s"} · {verifiedSources}{" "}
              identity verified
            </p>
          </div>
          {onOpen ? (
            <Button size="sm" className="mt-3 h-8 gap-1.5" onClick={onOpen}>
              Open {entityLabel(story)}
              <ArrowRight01Icon className="h-3.5 w-3.5" />
            </Button>
          ) : null}
        </div>
      </div>

      <details className="group pb-4">
        <summary className="flex cursor-pointer list-none items-center gap-2 px-1 py-1 text-xs font-medium text-foreground/75 hover:text-foreground">
          <Layers01Icon className="h-3.5 w-3.5" />
          Inspect {story.signals.length} signal
          {story.signals.length === 1 ? "" : "s"} from{" "}
          {story.evidence_source_count} {evidenceNoun}
          {story.evidence_source_count === 1 ? "" : "s"}
          <span className="text-muted-foreground">
            · evidence and scoring details
          </span>
        </summary>
        <div className="mt-2 divide-y divide-border/50 border-y border-border/50">
          {story.signals.map((signal) => (
            <div
              key={signal.id}
              className="grid gap-2 py-3 md:grid-cols-[110px_minmax(0,1fr)_160px] md:gap-4"
            >
              <div>
                <p className="text-[11px] font-semibold uppercase tracking-[0.05em] text-foreground/70">
                  {domainLabels[signal.signal_domain ?? "conversation"] ??
                    signal.signal_domain}
                </p>
                <p className="mt-0.5 text-[11px] text-muted-foreground">
                  {timeAgo(signal.detected_at)}
                </p>
              </div>
              <div className="min-w-0">
                <p className="text-[13px] leading-5 text-foreground/90">
                  {signal.summary}
                </p>
                {signal.evidence_excerpt ? (
                  <p className="mt-1 line-clamp-2 text-xs leading-5 text-muted-foreground">
                    “{signal.evidence_excerpt}”
                  </p>
                ) : null}
                <p className="mt-1 text-[11px] text-muted-foreground">
                  {signal.source_type.replace(/_/g, " ")} ·{" "}
                  {signal.evidence_identity_method?.replace(/_/g, " ") ||
                    "unknown identity"}{" "}
                  · {signal.evidence_identity_trust || "unknown"} trust
                </p>
                {onOpenSignal && canOpenSignal?.(signal) ? (
                  <button
                    type="button"
                    className="mt-1.5 text-[11px] font-medium text-orange-700 hover:text-orange-800 dark:text-orange-400"
                    onClick={() => onOpenSignal(signal)}
                  >
                    Open exact source
                  </button>
                ) : null}
              </div>
              <dl className="grid grid-cols-2 gap-x-3 text-right text-[11px] md:block">
                <div className="md:flex md:justify-between">
                  <dt className="text-muted-foreground">Priority</dt>
                  <dd className="font-semibold tabular-nums">
                    {(signal.business_priority ?? 0).toFixed(1)}
                  </dd>
                </div>
                <div className="md:mt-1 md:flex md:justify-between">
                  <dt className="text-muted-foreground">Confidence</dt>
                  <dd className="font-semibold tabular-nums">
                    {Math.round(signal.confidence * 100)}%
                  </dd>
                </div>
              </dl>
            </div>
          ))}
        </div>
      </details>
    </article>
  );
}

export function SignalWorkspaceFeed({ workspaceId }: { workspaceId: string }) {
  const navigate = useNavigate();
  const workspaceSlug = useWorkspaceStore(
    (state) => state.currentWorkspace?.slug ?? "",
  );
  const [filters, setFilters] = useState<CRMSignalFeedFilters>({
    status: "active",
    max_age_days: 90,
    page: 1,
    per_page: 20,
  });
  const [queryFilters, setQueryFilters] = useState<QueryFilterGroup>();
  const [lanePages, setLanePages] = useState<
    Partial<Record<CRMCommercialMotion, number>>
  >({});
  const feed = useSignalWorkspaceFeed(workspaceId, {
    ...filters,
    filters: serializeQueryFilterGroup(queryFilters),
    lane_pages: lanePages,
  });
  const companies = useCompanies(workspaceId, { per_page: 100 });
  const members = useWorkspaceMembers(workspaceId);
  const ownerNames = useMemo(
    () =>
      new Map(
        (members.data ?? []).map((member) => [
          member.id,
          member.full_name || member.email || "Unknown owner",
        ]),
      ),
    [members.data],
  );
  const queryFields = useMemo(
    () =>
      buildCRMSignalQueryFields(
        (members.data ?? []).map((member) => ({
          id: member.id,
          label: member.full_name || member.email || "Unnamed member",
        })),
        (companies.data?.data ?? []).map((company) => ({
          id: company.id,
          label: company.name,
        })),
      ),
    [companies.data?.data, members.data],
  );
  const setFilter = <K extends keyof CRMSignalFeedFilters>(
    key: K,
    value: CRMSignalFeedFilters[K] | "all",
  ) => {
    setFilters((current) => ({
      ...current,
      [key]: value === "all" ? undefined : value,
      page: 1,
    }));
    setLanePages({});
  };
  const lanes = feed.data?.lanes ?? [];
  const stories = lanes.flatMap((lane) => lane.data);

  const openStory = (story: CRMSignalAccountStory) => {
    if (!workspaceSlug || !story.entity_id) return;
    if (story.entity_type === "company")
      void navigate({
        to: "/w/$slug/crm/companies/$companyId",
        params: { slug: workspaceSlug, companyId: story.entity_id },
      } as never);
    if (story.entity_type === "contact")
      void navigate({
        to: "/w/$slug/crm/contacts/$contactId",
        params: { slug: workspaceSlug, contactId: story.entity_id },
      } as never);
    if (story.entity_type === "deal")
      void navigate({
        to: "/w/$slug/crm/deals/$dealId",
        params: { slug: workspaceSlug, dealId: story.entity_id },
      } as never);
  };
  const canOpenSignal = (signal: CRMSignal) => {
    if (signal.source_type === "email")
      return (
        !!signal.source_thread_id &&
        (!!signal.contact_id || !!signal.company_id)
      );
    if (signal.source_type === "meeting") return !!signal.source_id;
    if (signal.source_type === "support") return !!signal.source_thread_id;
    if (signal.source_type === "note" || signal.source_type === "call")
      return !!signal.contact_id || !!signal.company_id;
    return false;
  };
  const openSignal = (signal: CRMSignal) => {
    if (!workspaceSlug || !canOpenSignal(signal)) return;
    if (signal.source_type === "meeting" && signal.source_id)
      void navigate({
        to: "/w/$slug/crm/meetings/$meetingId",
        params: { slug: workspaceSlug, meetingId: signal.source_id },
      } as never);
    else if (signal.source_type === "support" && signal.source_thread_id)
      void navigate({
        to: "/w/$slug/support/$conversationId",
        params: {
          slug: workspaceSlug,
          conversationId: signal.source_thread_id,
        },
      } as never);
    else if (signal.contact_id)
      void navigate({
        to: "/w/$slug/crm/contacts/$contactId",
        params: { slug: workspaceSlug, contactId: signal.contact_id },
        search:
          signal.source_type === "email"
            ? { tab: "emails", thread: signal.source_thread_id }
            : { tab: signal.source_type === "note" ? "notes" : "calls" },
      } as never);
    else if (signal.company_id)
      void navigate({
        to: "/w/$slug/crm/companies/$companyId",
        params: { slug: workspaceSlug, companyId: signal.company_id },
        search:
          signal.source_type === "email"
            ? { tab: "emails", thread: signal.source_thread_id }
            : { tab: signal.source_type === "note" ? "notes" : "calls" },
      } as never);
  };

  return (
    <section aria-labelledby="account-stories-title">
      <div className="flex flex-wrap items-end justify-between gap-3 border-b border-border/60 pb-4">
        <div>
          <div className="flex items-center gap-2">
            <Activity01Icon className="h-4 w-4 text-muted-foreground" />
            <h2
              id="account-stories-title"
              className="text-base font-semibold tracking-[-0.01em]"
            >
              Signal inbox
            </h2>
          </div>
          <p className="mt-1 text-xs text-muted-foreground">
            {filters.include_context
              ? "Includes older signals and operational context that have not passed commercial qualification."
              : "Purchases, upgrades, renewals, and revenue risks with supporting evidence."}
          </p>
        </div>
        {feed.data ? (
          <p className="text-[11px] text-muted-foreground">
            {feed.data.total} ranked stor{feed.data.total === 1 ? "y" : "ies"}
          </p>
        ) : null}
      </div>

      <div className="flex flex-wrap items-center gap-2 border-b border-border/50 py-3">
        <QueryBuilderPopover
          fields={queryFields}
          value={queryFilters}
          onApply={(value) => {
            setQueryFilters(value);
            setFilters((current) => ({ ...current, page: 1 }));
            setLanePages({});
          }}
          triggerLabel="Filter signals"
        />
        <Select
          value={filters.severity ?? "all"}
          onValueChange={(value) =>
            setFilter("severity", value as CRMSignalSeverity | "all")
          }
        >
          <SelectTrigger className={selectClassName}>
            <SelectValue placeholder="Severity" />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="all">Any severity</SelectItem>
            <SelectItem value="high">High</SelectItem>
            <SelectItem value="medium">Medium</SelectItem>
            <SelectItem value="low">Low</SelectItem>
          </SelectContent>
        </Select>
        <Select
          value={filters.status ?? "all"}
          onValueChange={(value) =>
            setFilters((current) => ({
              ...current,
              status: value as "active" | "dismissed" | "superseded" | "all",
              page: 1,
            }))
          }
        >
          <SelectTrigger className={selectClassName}>
            <SelectValue placeholder="Status" />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="all">Any status</SelectItem>
            <SelectItem value="active">Active</SelectItem>
            <SelectItem value="dismissed">Dismissed</SelectItem>
            <SelectItem value="superseded">Superseded</SelectItem>
          </SelectContent>
        </Select>
        <Select
          value={filters.motion ?? "all"}
          onValueChange={(value) =>
            setFilter("motion", value as CRMCommercialMotion | "all")
          }
        >
          <SelectTrigger className={selectClassName}>
            <SelectValue placeholder="Motion" />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="all">Any motion</SelectItem>
            {(
              Object.entries(motionLabels) as [CRMCommercialMotion, string][]
            ).map(([value, label]) => (
              <SelectItem key={value} value={value}>
                {label}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
        <Select
          value={String(filters.max_age_days ?? "all")}
          onValueChange={(value) =>
            setFilter("max_age_days", value === "all" ? "all" : Number(value))
          }
        >
          <SelectTrigger className={selectClassName}>
            <SelectValue placeholder="Age" />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="all">Any age</SelectItem>
            <SelectItem value="7">Last 7 days</SelectItem>
            <SelectItem value="30">Last 30 days</SelectItem>
            <SelectItem value="90">Last 90 days</SelectItem>
            <SelectItem value="180">Last 180 days</SelectItem>
          </SelectContent>
        </Select>
        <label className="flex cursor-pointer items-center gap-2 text-xs text-muted-foreground">
          <Checkbox
            checked={filters.include_context ?? false}
            onCheckedChange={(checked) => {
              setFilters((current) => ({
                ...current,
                include_context: checked === true,
                page: 1,
              }));
              setLanePages({});
            }}
          />
          Include unqualified context
        </label>
        {Object.values(filters).some((value) => value !== undefined) ||
        queryFilters ? (
          <Button
            variant="ghost"
            size="sm"
            className="h-8 px-2 text-xs"
            onClick={() => {
              setFilters({
                status: "active",
                max_age_days: 90,
                page: 1,
                per_page: 20,
              });
              setQueryFilters(undefined);
              setLanePages({});
            }}
          >
            Clear
          </Button>
        ) : null}
      </div>

      {feed.isLoading ? (
        <div className="py-12 text-sm text-muted-foreground">
          Composing account stories…
        </div>
      ) : feed.isError ? (
        <div className="py-12 text-sm text-destructive">
          The signal workspace could not be loaded.
        </div>
      ) : stories.length === 0 ? (
        <div className="py-12">
          <p className="text-sm font-medium">
            {feed.data?.rollout_mode === "shadow"
              ? "Signal inbox is still in shadow evaluation"
              : "No stories match these filters"}
          </p>
          <p className="mt-1 text-xs text-muted-foreground">
            {feed.data?.rollout_mode === "shadow"
              ? "Admins can inspect the separate shadow preview and take the workspace live after its quality gate passes."
              : filters.include_context
                ? "Broaden the time window or adjust your filters to see more context."
                : "No qualified commercial evidence matches. You can include unqualified context above. Describe your offerings, buyers, and exclusions in Settings → Knowledge → Company/Product Context."}
          </p>
        </div>
      ) : (
        <div className="space-y-7">
          {lanes.map((lane) => {
            const motion = lane.commercial_motion;
            const pageCount = Math.max(1, Math.ceil(lane.total / lane.per_page));
            return (
              <section key={motion} aria-labelledby={`signal-motion-${motion}`}>
                <div className="flex items-end justify-between gap-3 border-b border-border/60 pb-2">
                  <div>
                    <h3
                      id={`signal-motion-${motion}`}
                      className="text-sm font-semibold"
                    >
                      {motionLabel(motion)}
                    </h3>
                    <p className="text-xs text-muted-foreground">
                      {lane.total} evidence-gated account
                      {lane.total === 1 ? "" : "s"}
                    </p>
                  </div>
                  {pageCount > 1 ? (
                    <div className="flex items-center gap-2 text-xs text-muted-foreground">
                      <span>{lane.page} / {pageCount}</span>
                      <Button
                        variant="outline"
                        size="sm"
                        className="h-7 px-2"
                        disabled={lane.page <= 1 || feed.isFetching}
                        onClick={() => setLanePages((current) => ({ ...current, [motion]: lane.page - 1 }))}
                      >
                        Previous
                      </Button>
                      <Button
                        variant="outline"
                        size="sm"
                        className="h-7 px-2"
                        disabled={lane.page >= pageCount || feed.isFetching}
                        onClick={() => setLanePages((current) => ({ ...current, [motion]: lane.page + 1 }))}
                      >
                        Next
                      </Button>
                    </div>
                  ) : null}
                </div>
                {lane.data.map((story) => (
                  <StoryRow
                    key={story.id}
                    story={story}
                    ownerName={
                      story.owner_member_id
                        ? ownerNames.get(story.owner_member_id)
                        : undefined
                    }
                    onOpen={
                      story.entity_type === "unresolved"
                        ? undefined
                        : () => openStory(story)
                    }
                    onOpenSignal={
                      story.signals.some(canOpenSignal) ? openSignal : undefined
                    }
                    canOpenSignal={canOpenSignal}
                  />
                ))}
              </section>
            );
          })}
        </div>
      )}
    </section>
  );
}
