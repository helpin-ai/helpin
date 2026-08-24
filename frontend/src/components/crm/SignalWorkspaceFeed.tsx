import { useMemo, useState } from "react";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { useCompanies, useSignalWorkspaceFeed } from "@/hooks/queries/useCRM";
import { useWorkspaceMembers } from "@/hooks/queries";
import {
  Activity01Icon,
  FilterHorizontalIcon,
  Layers01Icon,
} from "@/lib/icons";
import type {
  CRMSignalAccountStory,
  CRMSignalDomain,
  CRMSignalFeedFilters,
  CRMSignalPolarity,
  CRMSignalSeverity,
} from "@/lib/crmTypes";
import { cn, timeAgo } from "@/lib/utils";

const domainLabels: Record<string, string> = {
  conversation: "Conversation",
  web_behavior: "Web",
  product_usage: "Product",
  support: "Support",
  delivery: "Delivery",
  relationship: "Relationship",
  market: "Market",
};

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

function StoryRow({
  story,
  ownerName,
}: {
  story: CRMSignalAccountStory;
  ownerName?: string;
}) {
  const confidence = story.signals.length
    ? Math.round(
        (story.signals.reduce((sum, signal) => sum + signal.confidence, 0) /
          story.signals.length) *
          100,
      )
    : 0;
  return (
    <article className="border-t border-border/55 first:border-t-0">
      <div className="grid gap-4 px-1 py-5 md:grid-cols-[minmax(0,1fr)_150px]">
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
              {story.severity}
            </Badge>
          </div>
          <p className="mt-1 text-xs text-muted-foreground">
            {story.change_summary}
            {ownerName ? ` · ${ownerName}` : ""}
          </p>
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
          <div>
            <p className="text-[10px] font-semibold uppercase tracking-[0.09em] text-muted-foreground">
              Business priority
            </p>
            <p className="mt-0.5 text-2xl font-semibold tabular-nums tracking-[-0.04em]">
              {Math.round(story.priority)}
            </p>
          </div>
          <div className="md:mt-2">
            <p className="text-[10px] font-semibold uppercase tracking-[0.09em] text-muted-foreground">
              Evidence certainty
            </p>
            <p className="mt-0.5 text-sm font-medium tabular-nums">
              {confidence}%
            </p>
          </div>
        </div>
      </div>

      <details className="group pb-4">
        <summary className="flex cursor-pointer list-none items-center gap-2 px-1 py-1 text-xs font-medium text-foreground/75 hover:text-foreground">
          <Layers01Icon className="h-3.5 w-3.5" />
          Inspect {story.signals.length} source
          {story.signals.length === 1 ? "" : "s"}
          <span className="text-muted-foreground">
            · {impactLabel(story)} · score model v{story.score_version}
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
                <div className="col-span-2 mt-1 hidden justify-between md:flex">
                  <dt className="text-muted-foreground">Weight × decay</dt>
                  <dd className="tabular-nums">
                    {String(signal.score_factors?.business_weight ?? "—")} ×{" "}
                    {typeof signal.score_factors?.recency_factor === "number"
                      ? signal.score_factors.recency_factor.toFixed(2)
                      : "—"}
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
  const [filters, setFilters] = useState<CRMSignalFeedFilters>({
    status: "active",
    max_age_days: 90,
  });
  const feed = useSignalWorkspaceFeed(workspaceId, filters);
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
  const setFilter = <K extends keyof CRMSignalFeedFilters>(
    key: K,
    value: CRMSignalFeedFilters[K] | "all",
  ) => {
    setFilters((current) => ({
      ...current,
      [key]: value === "all" ? undefined : value,
    }));
  };
  const stories = feed.data?.data ?? [];

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
              Account stories
            </h2>
          </div>
          <p className="mt-1 text-xs text-muted-foreground">
            Corroborated evidence, ranked by business importance—not model
            certainty.
          </p>
        </div>
        {feed.data ? (
          <p className="text-[11px] text-muted-foreground">
            Scoring v{feed.data.score_version}
            {feed.data.heuristic ? " · heuristic, pending calibration" : ""}
          </p>
        ) : null}
      </div>

      <div className="flex flex-wrap items-center gap-2 border-b border-border/50 py-3">
        <FilterHorizontalIcon className="mr-1 h-3.5 w-3.5 text-muted-foreground" />
        <Select
          value={filters.owner_member_id ?? "all"}
          onValueChange={(value) => setFilter("owner_member_id", value)}
        >
          <SelectTrigger className={selectClassName}>
            <SelectValue placeholder="Owner" />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="all">All owners</SelectItem>
            {(members.data ?? []).map((member) => (
              <SelectItem key={member.id} value={member.id}>
                {member.full_name || member.email || "Unnamed member"}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
        <Select
          value={filters.account_id ?? "all"}
          onValueChange={(value) => setFilter("account_id", value)}
        >
          <SelectTrigger className={selectClassName}>
            <SelectValue placeholder="Account" />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="all">All accounts</SelectItem>
            {(companies.data?.data ?? []).map((company) => (
              <SelectItem key={company.id} value={company.id}>
                {company.name}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
        <Select
          value={filters.domain ?? "all"}
          onValueChange={(value) =>
            setFilter("domain", value as CRMSignalDomain | "all")
          }
        >
          <SelectTrigger className={selectClassName}>
            <SelectValue placeholder="Domain" />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="all">All domains</SelectItem>
            {Object.entries(domainLabels).map(([value, label]) => (
              <SelectItem key={value} value={value}>
                {label}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
        <Select
          value={filters.polarity ?? "all"}
          onValueChange={(value) =>
            setFilter("polarity", value as CRMSignalPolarity | "all")
          }
        >
          <SelectTrigger className={selectClassName}>
            <SelectValue placeholder="Polarity" />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="all">Any polarity</SelectItem>
            <SelectItem value="positive">Momentum</SelectItem>
            <SelectItem value="negative">Risk</SelectItem>
            <SelectItem value="neutral">Neutral</SelectItem>
          </SelectContent>
        </Select>
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
          value={filters.trust ?? "all"}
          onValueChange={(value) => setFilter("trust", value)}
        >
          <SelectTrigger className={selectClassName}>
            <SelectValue placeholder="Trust" />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="all">Any trust</SelectItem>
            <SelectItem value="verified">Verified</SelectItem>
            <SelectItem value="probabilistic">Probabilistic</SelectItem>
            <SelectItem value="untrusted">Untrusted</SelectItem>
          </SelectContent>
        </Select>
        <Select
          value={filters.status ?? "all"}
          onValueChange={(value) =>
            setFilters((current) => ({
              ...current,
              status: value as "active" | "dismissed" | "all",
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
        {Object.values(filters).some((value) => value !== undefined) ? (
          <Button
            variant="ghost"
            size="sm"
            className="h-8 px-2 text-xs"
            onClick={() => setFilters({ status: "all" })}
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
          <p className="text-sm font-medium">No stories match these filters</p>
          <p className="mt-1 text-xs text-muted-foreground">
            Broaden the time window or include lower-trust context.
          </p>
        </div>
      ) : (
        <div>
          {stories.map((story) => (
            <StoryRow
              key={story.id}
              story={story}
              ownerName={
                story.owner_member_id
                  ? ownerNames.get(story.owner_member_id)
                  : undefined
              }
            />
          ))}
        </div>
      )}
    </section>
  );
}
