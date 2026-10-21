import { useEffect, useMemo, useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { useNavigate } from "@tanstack/react-router";
import { toast } from "sonner";
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogDescription,
  DialogFooter,
} from "@/components/ui/dialog";
import { Button } from "@/components/ui/button";
import {
  QuietDropdown,
  QuietSearchInput,
} from "@/components/design-system/quiet";
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from "@/components/ui/tooltip";
import { Avatar, AvatarFallback } from "@/components/ui/avatar";
import { ContactAvatarImage } from "@/components/ui/contact-avatar-image";
import {
  ArrowLeftRightIcon,
  ArrowRight02Icon,
  InformationCircleIcon,
} from "@/lib/icons";
import type { SupportConversation } from "@/lib/pmTypes";
import { supportMergeService } from "@/lib/services/supportMergeService";
import { unwrapRequired } from "@/lib/queryUtils";
import { mergeSourceIds } from "@/lib/supportMerging";
import { useMergeConversation } from "@/hooks/queries/useSupportMerge";
import { useWorkspaceStore } from "@/stores/workspaceStore";
import { useSupportInboxStore } from "@/stores/supportInboxStore";
import { ConversationTranscript } from "./ConversationTranscript";
import { getAvatarColor, getCustomerAvatarSeed, getInitial } from "./helpers";
import { STATUS_LABELS } from "./constants";
import { cn } from "@/lib/utils";

interface Props {
  workspaceId: string;
  conversations: SupportConversation[];
  initialTarget?: SupportConversation;
  open: boolean;
  onOpenChange: (open: boolean) => void;
  onMerged?: (ids: string[]) => void;
}

export function ConversationMergeDialog(props: Props) {
  return (
    <Dialog open={props.open} onOpenChange={props.onOpenChange}>
      {props.open && <MergeDialogBody {...props} />}
    </Dialog>
  );
}

function MergeDialogBody({
  workspaceId,
  conversations,
  initialTarget,
  onOpenChange,
  onMerged,
}: Props) {
  const [selected, setSelected] = useState(conversations);
  const [target, setTarget] = useState<SupportConversation | null>(
    initialTarget ?? null,
  );
  const [search, setSearch] = useState("");
  const [query, setQuery] = useState("");
  const [scope, setScope] = useState("same");
  const [page, setPage] = useState(1);
  const [sourceIndex, setSourceIndex] = useState(0);
  const [mobileTab, setMobileTab] = useState("target");
  const [operation, setOperation] = useState(() => crypto.randomUUID());
  const navigate = useNavigate();
  const slug = useWorkspaceStore((s) => s.currentWorkspace?.slug ?? "");
  const sourceIds = useMemo(
    () =>
      mergeSourceIds(
        selected.map((c) => c.id),
        target?.id ?? "",
      ),
    [selected, target],
  );
  useEffect(() => {
    const timer = setTimeout(() => {
      setQuery(search);
      setPage(1);
    }, 200);
    return () => clearTimeout(timer);
  }, [search]);
  useEffect(() => {
    setOperation(crypto.randomUUID());
    setSourceIndex(0);
  }, [target, selected]);
  const candidates = useQuery({
    queryKey: [
      "support",
      workspaceId,
      "merge-candidates",
      selected[0]?.id,
      query,
      scope,
      page,
    ],
    queryFn: async () =>
      unwrapRequired(
        await supportMergeService.candidates(
          workspaceId,
          selected[0].id,
          query,
          scope === "all",
          page,
        ),
        "Merge candidates",
      ),
    enabled: !target && !!selected[0],
    placeholderData: (previous) => previous,
  });
  const preview = useQuery({
    queryKey: ["support", workspaceId, "merge-preview", target?.id, sourceIds],
    queryFn: async () =>
      unwrapRequired(
        await supportMergeService.preview(workspaceId, target!.id, sourceIds),
        "Merge preview",
      ),
    enabled: !!target && sourceIds.length > 0,
    staleTime: 0,
  });
  const merge = useMergeConversation(workspaceId);
  const review = preview.data;
  const source =
    review?.sources[sourceIndex] ?? selected.find((c) => c.id !== target?.id);
  const sameCustomer = review ? !review.cross_customer : scope === "same";
  const chooseTarget = (c: SupportConversation) => {
    merge.reset();
    setTarget(c);
  };
  const swap = () => {
    const activeSource = selected.find((c) => c.id !== target?.id);
    if (!activeSource || !target) return;
    setSelected([target]);
    setTarget(activeSource);
    merge.reset();
  };
  async function commit() {
    if (!review || !target) return;
    try {
      const result = await merge.mutateAsync({
        target: target.id,
        sources: sourceIds,
        token: review.preview_token,
        operation,
      });
      onOpenChange(false);
      onMerged?.(result.affected_ids);
      useSupportInboxStore
        .getState()
        .selectConversation(result.conversation_id);
      void navigate({
        to: "/w/$slug/support/$conversationId",
        params: { slug, conversationId: result.conversation_id },
      });
      toast.success("Conversations merged");
    } catch {
      /* Keep the review and original selection available for recovery. */
    }
  }
  return (
    <DialogContent
      showCloseButton={!merge.isPending}
      className={cn(
        "flex max-h-[90dvh] flex-col gap-0 overflow-hidden p-0",
        target ? "sm:max-w-[960px]" : "sm:max-w-[600px]",
        merge.isPending && "[&>button:last-child]:pointer-events-none",
      )}
      onEscapeKeyDown={(event) => {
        if (merge.isPending) event.preventDefault();
      }}
      onInteractOutside={(event) => {
        if (merge.isPending) event.preventDefault();
      }}
    >
      <DialogHeader className="px-6 pb-4 pt-6 text-left">
        <DialogTitle className="text-xl">
          {target ? "Merge conversations" : "Merge into"}
        </DialogTitle>
        <DialogDescription
          className={target && sameCustomer ? "sr-only" : "text-sm"}
        >
          {target
            ? "Review the conversations before merging."
            : selected.length === 1
              ? `From #${selected[0].display_id} · ${selected[0].subject}`
              : `${selected.length} selected · Choose the conversation to continue in.`}
        </DialogDescription>
        {target && sameCustomer && (
          <div className="pt-2">
            <MergeCustomer conversation={target} />
          </div>
        )}
      </DialogHeader>
      {!target ? (
        <>
          <div className="space-y-3 px-6 pb-3">
            <QuietSearchInput
              autoFocus
              value={search}
              onChange={(e) => setSearch(e.target.value)}
              placeholder="Search by subject, message, or number"
              aria-label="Search conversations to merge into"
            />
            {candidates.data?.cross_customer_allowed && (
              <QuietDropdown
                options={[
                  { value: "same", label: "Same customer" },
                  { value: "all", label: "All customers" },
                ]}
                selected={[scope]}
                trigger={
                  <Button variant="ghost" size="sm">
                    {scope === "all" ? "All customers" : "Same customer"}
                  </Button>
                }
                onSelect={(v) => {
                  setScope(v);
                  setPage(1);
                }}
                label="Customer scope"
              />
            )}
          </div>
          <div className="min-h-48 overflow-y-auto border-y px-3 py-2">
            {selected.length > 1 && !query && (
              <>
                <p className="px-3 py-2 text-xs font-medium text-muted-foreground">
                  Selected conversations
                </p>
                {selected.map((c) => (
                  <Candidate
                    key={c.id}
                    conversation={c}
                    onSelect={chooseTarget}
                  />
                ))}
                <div className="my-2 border-t" />
              </>
            )}
            {candidates.isPending ? (
              <p className="p-6 text-center text-sm text-muted-foreground">
                Finding conversations…
              </p>
            ) : candidates.isError ? (
              <div className="p-6 text-sm" role="alert">
                Could not load conversations.{" "}
                <Button
                  variant="ghost"
                  size="sm"
                  onClick={() => void candidates.refetch()}
                >
                  Retry
                </Button>
              </div>
            ) : (
              <>
                {!candidates.data?.data.length && (
                  <p className="p-6 text-center text-sm text-muted-foreground">
                    {query
                      ? "No matching conversations."
                      : "No conversations to merge into."}
                  </p>
                )}
                {candidates.data?.data
                  .filter(() => selected.length < 50)
                  .filter(
                    (c) =>
                      selected.length === 1 ||
                      !selected.some((s) => s.id === c.id),
                  )
                  .map((c) => (
                    <Candidate
                      key={c.id}
                      conversation={c}
                      onSelect={chooseTarget}
                    />
                  ))}
              </>
            )}
          </div>
          <DialogFooter className="items-center px-6 py-4">
            <div className="mr-auto flex gap-1">
              {page > 1 && (
                <Button
                  variant="ghost"
                  size="sm"
                  onClick={() => setPage((p) => p - 1)}
                >
                  Previous
                </Button>
              )}
              {candidates.data?.has_more && (
                <Button
                  variant="ghost"
                  size="sm"
                  onClick={() => setPage((p) => p + 1)}
                >
                  Next
                </Button>
              )}
            </div>
            <Button variant="outline" onClick={() => onOpenChange(false)}>
              Cancel
            </Button>
          </DialogFooter>
        </>
      ) : (
        <>
          <div className="mx-6 flex shrink-0 items-center justify-between pb-3 text-xs">
            <span className="text-muted-foreground">
              {review?.sources.length ?? sourceIds.length} conversation
              {(review?.sources.length ?? sourceIds.length) === 1
                ? ""
                : "s"}{" "}
              kept as history
            </span>
            <Tooltip>
              <TooltipTrigger asChild>
                <Button
                  variant="ghost"
                  size="icon-sm"
                  aria-label="What is kept when merging"
                >
                  <InformationCircleIcon className="h-4 w-4" />
                </Button>
              </TooltipTrigger>
              <TooltipContent className="max-w-64">
                The destination keeps its inbox, assignment, tags, and links.
                Original conversations and files stay available. You can unmerge
                later.
              </TooltipContent>
            </Tooltip>
          </div>
          <div
            className="mx-6 flex border-b sm:hidden"
            role="tablist"
            aria-label="Conversation preview"
          >
            <Button
              role="tab"
              aria-selected={mobileTab === "source"}
              variant="ghost"
              onClick={() => setMobileTab("source")}
            >
              Merge from
            </Button>
            <Button
              role="tab"
              aria-selected={mobileTab === "target"}
              variant="ghost"
              onClick={() => setMobileTab("target")}
            >
              Merge into
            </Button>
          </div>
          <div className="relative mx-6 grid min-h-0 flex-1 grid-cols-1 overflow-hidden rounded-lg border sm:grid-cols-2">
            <div
              className={cn(
                "flex min-h-0 flex-col sm:border-r",
                mobileTab !== "source" && "hidden sm:flex",
              )}
            >
              <div className="space-y-2 border-b px-4 py-3">
                <div className="flex h-7 items-center justify-between">
                  <span className="text-xs text-muted-foreground">
                    Merge from
                  </span>
                  {(review?.sources.length ?? 0) > 1 && (
                    <QuietDropdown
                      selected={[String(sourceIndex)]}
                      trigger={
                        <Button variant="ghost" size="sm">
                          {sourceIndex + 1} of {review?.sources.length}
                        </Button>
                      }
                      options={review!.sources.map((c, i) => ({
                        value: String(i),
                        label: `#${c.display_id} · ${c.subject}`,
                      }))}
                      onSelect={(v) => setSourceIndex(Number(v))}
                      label="Source conversation"
                    />
                  )}
                </div>
                {source && (
                  <ConversationPreviewHeading
                    conversation={source}
                    showCustomer={!sameCustomer}
                  />
                )}
              </div>
              <div className="h-[min(40vh,390px)] min-h-[160px]">
                {source && (
                  <ConversationTranscript
                    key={source.id}
                    workspaceId={workspaceId}
                    conversation={source}
                  />
                )}
              </div>
              <p className="border-t px-4 py-2.5 text-xs text-muted-foreground">
                Kept as history
              </p>
            </div>
            <div
              className={cn(
                "flex min-h-0 flex-col",
                mobileTab !== "target" && "hidden sm:flex",
              )}
            >
              <div className="space-y-2 border-b px-4 py-3">
                <div className="flex h-7 items-center justify-between">
                  <span className="text-xs text-muted-foreground">
                    Merge into
                  </span>
                  <Button
                    variant="ghost"
                    size="sm"
                    disabled={merge.isPending}
                    onClick={() => setTarget(null)}
                  >
                    Change
                  </Button>
                </div>
                <ConversationPreviewHeading
                  conversation={target}
                  showCustomer={!sameCustomer}
                />
              </div>
              <div className="h-[min(40vh,390px)] min-h-[160px]">
                <ConversationTranscript
                  key={target.id}
                  workspaceId={workspaceId}
                  conversation={target}
                />
              </div>
              <p className="border-t px-4 py-2.5 text-xs font-medium text-primary">
                Continues here
              </p>
            </div>
            {selected.length === 1 && (
              <Tooltip>
                <TooltipTrigger asChild>
                  <Button
                    variant="outline"
                    size="icon-sm"
                    className="absolute left-1/2 top-2 hidden -translate-x-1/2 bg-background sm:inline-flex"
                    aria-label="Swap merge direction"
                    disabled={merge.isPending}
                    onClick={swap}
                  >
                    <ArrowLeftRightIcon className="h-3.5 w-3.5" />
                  </Button>
                </TooltipTrigger>
                <TooltipContent>Swap direction</TooltipContent>
              </Tooltip>
            )}
          </div>
          <div
            className="space-y-1.5 px-6 pt-4 text-xs text-muted-foreground"
            aria-live="polite"
          >
            {preview.isPending && <p>Checking merge…</p>}
            {review &&
              (review.cross_customer || review.participants.length > 1) && (
                <p className="text-foreground">
                  New replies will be shared with{" "}
                  {[
                    ...new Set([
                      ...review.participants,
                      ...[review.target, ...review.sources]
                        .filter(
                          (c) =>
                            !c.customer_email &&
                            (c.anonymous_id || c.source === "widget"),
                        )
                        .map(
                          (c) => c.customer_name || `Visitor #${c.display_id}`,
                        ),
                    ]),
                  ].join(", ") || "the customers in these conversations"}
                  . Earlier histories stay private.
                </p>
              )}
            {review?.transfers_access && (
              <p>
                Teammates in the destination inbox will have access to both
                histories.
              </p>
            )}
            {review?.stops_ai && (
              <p>
                AI and scheduled follow-ups pause. You can resume AI in the
                active conversation.
              </p>
            )}
            {review?.reopens && (
              <p>
                The destination will reopen for the unresolved conversation.
              </p>
            )}
            {(preview.isError || merge.isError) && (
              <p role="alert" className="text-destructive">
                {(merge.error ?? preview.error)?.message}{" "}
                <Button
                  size="sm"
                  variant="ghost"
                  onClick={() => {
                    merge.reset();
                    setOperation(crypto.randomUUID());
                    void preview.refetch();
                  }}
                >
                  Review again
                </Button>
              </p>
            )}
          </div>
          <DialogFooter className="px-6 py-4">
            <Button
              variant="outline"
              disabled={merge.isPending}
              onClick={() => setTarget(null)}
            >
              Back
            </Button>
            <Button
              disabled={
                !review ||
                preview.isFetching ||
                preview.isError ||
                merge.isPending ||
                merge.isError
              }
              onClick={() => void commit()}
            >
              {merge.isPending
                ? "Merging…"
                : `Merge into #${target.display_id}`}
            </Button>
          </DialogFooter>
        </>
      )}
    </DialogContent>
  );
}

function Candidate({
  conversation: c,
  onSelect,
}: {
  conversation: SupportConversation;
  onSelect: (c: SupportConversation) => void;
}) {
  return (
    <button
      type="button"
      onClick={() => onSelect(c)}
      className="flex w-full items-center gap-3 rounded-md px-3 py-3 text-left hover:bg-accent focus-visible:outline-2 focus-visible:outline-ring"
    >
      <div className="min-w-0 flex-1">
        <p className="truncate text-sm font-medium">
          {c.subject}{" "}
          <span className="font-normal text-muted-foreground">
            #{c.display_id}
          </span>
        </p>
        <p className="mt-1 truncate text-xs text-muted-foreground">
          {c.customer_name || c.customer_email || "Visitor"} ·{" "}
          {c.source === "email" ? "Email" : "Chat"} · {STATUS_LABELS[c.status]}
          {c.mailbox_name ? ` · ${c.mailbox_name}` : ""}
        </p>
      </div>
      <ArrowRight02Icon className="h-4 w-4 shrink-0 text-muted-foreground" />
    </button>
  );
}
export function ConversationPreviewHeading({
  conversation: c,
  showCustomer = false,
}: {
  conversation: SupportConversation;
  showCustomer?: boolean;
}) {
  return (
    <>
      <p className="truncate text-sm font-semibold" title={c.subject}>
        {c.subject}{" "}
        <span className="font-normal text-muted-foreground">
          #{c.display_id}
        </span>
      </p>
      <p className="truncate text-xs text-muted-foreground">
        {showCustomer &&
          `${c.customer_name || c.customer_email || "Visitor"} · `}
        {c.source === "email" ? "Email" : "Chat"} · {STATUS_LABELS[c.status]} ·{" "}
        {c.mailbox_name || "Shared Inbox"}
      </p>
    </>
  );
}
function MergeCustomer({
  conversation: c,
}: {
  conversation: SupportConversation;
}) {
  return (
    <div className="flex items-center gap-2.5">
      <Avatar className="h-8 w-8">
        <ContactAvatarImage email={c.customer_email} alt="" />
        <AvatarFallback
          className={cn(
            "text-xs",
            getAvatarColor(
              getCustomerAvatarSeed(c.customer_email, c.customer_name, c.id),
            ),
          )}
        >
          {getInitial(c.customer_name || c.customer_email || "Visitor")}
        </AvatarFallback>
      </Avatar>
      <div className="min-w-0">
        <p className="truncate text-sm font-medium">
          {c.customer_name || c.customer_email || "Visitor"}
        </p>
        {c.customer_name && c.customer_email && (
          <p className="truncate text-xs text-muted-foreground">
            {c.customer_email}
          </p>
        )}
      </div>
    </div>
  );
}
