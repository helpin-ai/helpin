import { useInboxScopes } from "@/hooks/queries/useSupport";
import { QuietDropdown, QuietStatusBadge } from "@/components/design-system/quiet";
import { useState, type ComponentProps, type ReactNode } from "react";
import { useQuery } from "@tanstack/react-query";
import { Link } from "@tanstack/react-router";
import { toast } from "sonner";
import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogDescription,
  DialogFooter,
} from "@/components/ui/dialog";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { MoreHorizontalIcon } from "@/lib/icons";
import { useWorkspaceStore } from "@/stores/workspaceStore";
import { useWorkspaceAccess } from "@/hooks/queries/useSession";
import {
  useConversationMerges,
  useUnmergeConversation,
} from "@/hooks/queries/useSupportMerge";
import { supportMergeService } from "@/lib/services/supportMergeService";
import { unwrapRequired } from "@/lib/queryUtils";
import type {
  ConversationStatus,
  SupportConversation,
  SupportConversationMerge,
} from "@/lib/pmTypes";
import { ConversationTranscript } from "./ConversationTranscript";
import { ConversationPreviewHeading } from "./ConversationMergeDialog";
import { STATUS_LABELS } from "./constants";
import { formatMessageTime, getDayLabel } from "./helpers";

export function ConversationMergeHistory({
  workspaceId,
  conversation,
}: {
  workspaceId: string;
  conversation: SupportConversation;
}) {
  const { data: merges = [] } = useConversationMerges(
    workspaceId,
    conversation.id,
  );
  const { data: access } = useWorkspaceAccess(workspaceId);
  if (!merges.length) return null;
  const operations = new Map<string, SupportConversationMerge[]>();
  for (const merge of merges) {
    const key = merge.operation_id;
    operations.set(key, [...(operations.get(key) ?? []), merge]);
  }
  return (
    <div className="my-4 space-y-3">
      {[...operations.values()].map((rows) => (
        <ConversationMergeHistoryEvent
          key={rows[0].operation_id}
          workspaceId={workspaceId}
          conversation={conversation}
          merges={rows}
          canUnmerge={!!access?.permissions.includes("support.merge")}
        />
      ))}
    </div>
  );
}

export function ConversationMergeHistoryEvent({
  workspaceId,
  conversation,
  merges,
  canUnmerge,
  restored = false,
}: {
  workspaceId: string;
  conversation: SupportConversation;
  merges: SupportConversationMerge[];
  canUnmerge: boolean;
  restored?: boolean;
}) {
  const [expanded, setExpanded] = useState(false);
  if (restored)
    return (
      <p className="my-4 text-center text-xs text-muted-foreground">
        Conversation #{merges[0].source?.display_id ?? "—"} was unmerged ·{" "}
        {formatMessageTime(merges[0].ended_at!)}
      </p>
    );
  const incoming = merges.filter((m) => m.target_id === conversation.id);
  const outgoing = merges.filter((m) => m.source_id === conversation.id);
  return (
    <div className="my-4 space-y-2">
      {incoming.length > 1 && (
        <p className="mx-auto max-w-xl text-xs text-muted-foreground">
          {incoming[0].actor_name?.split(" ")[0] || "A teammate"} merged{" "}
          {incoming.length} conversations ·{" "}
          {formatMessageTime(incoming[0].created_at)}
        </p>
      )}
      {(expanded ? incoming : incoming.slice(0, 3)).map((merge) => (
        <MergeHistoryEntry
          key={merge.id}
          workspaceId={workspaceId}
          merge={merge}
          canUnmerge={canUnmerge}
          showActor={incoming.length === 1}
        />
      ))}
      {incoming.length > 3 && (
        <Button
          variant="ghost"
          size="sm"
          className="ml-3 text-xs"
          onClick={() => setExpanded(!expanded)}
        >
          {expanded
            ? "Show less"
            : `Show ${incoming.length - 3} more conversations`}
        </Button>
      )}
      {outgoing.map((merge) => (
        <SharedHistoryEntry
          key={merge.id}
          workspaceId={workspaceId}
          conversation={conversation}
          merge={merge}
          canUnmerge={canUnmerge}
        />
      ))}
    </div>
  );
}

function MergeHistoryEntry({
  workspaceId,
  merge,
  canUnmerge,
  showActor,
}: {
  workspaceId: string;
  merge: SupportConversationMerge;
  canUnmerge: boolean;
  showActor: boolean;
}) {
  const [historyOpen, setHistoryOpen] = useState(false);
  const [unmergeOpen, setUnmergeOpen] = useState(false);
  const source = merge.source;
  return (
    <div className="mx-auto max-w-xl border-l-2 border-border py-1 pl-3 text-xs">
      <div className="flex items-center gap-2 text-muted-foreground">
        <span className="min-w-0 flex-1">
          {showActor
            ? `${merge.actor_name?.split(" ")[0] || "A teammate"} merged a conversation`
            : "Merged history"}
        </span>
        <span
          className="shrink-0"
          title={`${getDayLabel(merge.created_at)} · ${formatMessageTime(merge.created_at)}`}
        >
          {getDayLabel(merge.created_at)} ·{" "}
          {formatMessageTime(merge.created_at)}
        </span>
        {canUnmerge && !merge.ended_at && source && (
          <DropdownMenu>
            <DropdownMenuTrigger asChild>
              <Button
                size="icon-sm"
                variant="ghost"
                aria-label={`Actions for merged conversation #${source.display_id}`}
              >
                <MoreHorizontalIcon className="h-4 w-4" />
              </Button>
            </DropdownMenuTrigger>
            <DropdownMenuContent align="end">
              <DropdownMenuItem onSelect={() => setUnmergeOpen(true)}>
                Unmerge…
              </DropdownMenuItem>
            </DropdownMenuContent>
          </DropdownMenu>
        )}
      </div>
      {source ? (
        <>
          <p className="mt-1 truncate font-medium">
            #{source.display_id} · {source.subject}
          </p>
          <div className="mt-1 flex items-center justify-between gap-2">
            <span className="text-muted-foreground">
              {merge.ended_at
                ? "Unmerged"
                : source.customer_name ||
                  source.customer_email ||
                  "Original conversation"}
            </span>
            <Button
              variant="link"
              size="sm"
              className="h-auto p-0 text-xs"
              onClick={() => setHistoryOpen(true)}
            >
              View conversation
            </Button>
          </div>
        </>
      ) : (
        <p className="mt-1 text-muted-foreground">
          This history is no longer available to you.
        </p>
      )}
      {source && (
        <ConversationHistoryDialog
          workspaceId={workspaceId}
          conversation={source}
          open={historyOpen}
          onOpenChange={setHistoryOpen}
        />
      )}
      {unmergeOpen && (
        <UnmergeDialog
          workspaceId={workspaceId}
          merge={merge}
          open={unmergeOpen}
          onOpenChange={setUnmergeOpen}
        />
      )}
    </div>
  );
}

function SharedHistoryEntry({
  workspaceId,
  conversation,
  merge,
  canUnmerge,
}: {
  workspaceId: string;
  conversation: SupportConversation;
  merge: SupportConversationMerge;
  canUnmerge: boolean;
}) {
  const [open, setOpen] = useState(false);
  const [unmergeOpen, setUnmergeOpen] = useState(false);
  return (
    <div className="text-center">
      <Button
        variant="ghost"
        size="sm"
        disabled={!merge.target}
        onClick={() => setOpen(true)}
      >
        Shared while merged
        {merge.target
          ? ` with #${merge.target.display_id}`
          : " · restricted history"}
      </Button>
      {canUnmerge && merge.target && !merge.ended_at && (
        <Button variant="ghost" size="sm" onClick={() => setUnmergeOpen(true)}>
          Unmerge…
        </Button>
      )}
      {unmergeOpen && (
        <UnmergeDialog
          workspaceId={workspaceId}
          merge={merge}
          open={unmergeOpen}
          onOpenChange={setUnmergeOpen}
        />
      )}
      <ConversationHistoryDialog
        workspaceId={workspaceId}
        conversation={conversation}
        mergeId={merge.id}
        open={open}
        onOpenChange={setOpen}
      />
    </div>
  );
}

const STATUS_BADGE_TONES: Record<ConversationStatus, ComponentProps<typeof QuietStatusBadge>["tone"]> = {
  open: "current",
  waiting_on_customer: "lifecycle",
  resolved: "positive",
  spam: "blocker",
};

export function ConversationHistoryDialog({
  workspaceId,
  conversation,
  mergeId,
  actions,
  open,
  onOpenChange,
}: {
  workspaceId: string;
  conversation: SupportConversation;
  mergeId?: string;
  /**
   * Buttons under the transcript, such as opening or merging the
   * conversation. The header then shows "#ID - Subject" like the thread.
   */
  actions?: ReactNode;
  open: boolean;
  onOpenChange: (v: boolean) => void;
}) {
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="flex max-h-[90dvh] flex-col gap-0 overflow-hidden p-0 sm:max-w-[720px]">
        {actions ? (
          <DialogHeader className="border-b py-4 pl-6 pr-14 text-left">
            <div className="flex min-w-0 items-center gap-2">
              <DialogTitle className="min-w-0 truncate text-sm font-medium text-foreground">
                <span className="font-semibold text-muted-foreground">#{conversation.display_id}</span>
                <span className="mx-1 text-muted-foreground">-</span>
                <span>{conversation.subject}</span>
              </DialogTitle>
              <QuietStatusBadge tone={STATUS_BADGE_TONES[conversation.status]} className="shrink-0">
                {STATUS_LABELS[conversation.status]}
              </QuietStatusBadge>
            </div>
            <DialogDescription className="truncate text-xs text-muted-foreground">
              {conversation.customer_name || conversation.customer_email || "Visitor"} ·{" "}
              {conversation.mailbox_name || "Shared Inbox"}
            </DialogDescription>
          </DialogHeader>
        ) : (
          <DialogHeader className="border-b px-6 py-5 text-left">
            <DialogTitle>
              {mergeId ? "Shared while merged" : "Original conversation"}
            </DialogTitle>
            <DialogDescription className="sr-only">
              Read-only conversation history
            </DialogDescription>
            <div className="space-y-1 pt-2">
              <ConversationPreviewHeading
                conversation={conversation}
                showCustomer
              />
            </div>
          </DialogHeader>
        )}
        <div className="h-[min(65dvh,650px)] min-h-0">
          {open && (
            <>
              <ConversationTranscript
                workspaceId={workspaceId}
                conversation={conversation}
                mergeId={mergeId}
              />
            </>
          )}
        </div>
        {actions && (
          <DialogFooter className="border-t px-6 py-4">{actions}</DialogFooter>
        )}
      </DialogContent>
    </Dialog>
  );
}

function UnmergeDialog({
  workspaceId,
  merge,
  open,
  onOpenChange,
}: {
  workspaceId: string;
  merge: SupportConversationMerge;
  open: boolean;
  onOpenChange: (v: boolean) => void;
}) {
  const [restoreMailbox, setRestoreMailbox] = useState<string>();
  const { data: scopes } = useInboxScopes(workspaceId);
  const preview = useQuery({
    queryKey: [
      "support",
      workspaceId,
      "unmerge-preview",
      merge.id,
      restoreMailbox,
    ],
    queryFn: async () =>
      unwrapRequired(
        await supportMergeService.unmergePreview(
          workspaceId,
          merge.id,
          restoreMailbox,
        ),
        "Unmerge preview",
      ),
    staleTime: 0,
  });
  const mutation = useUnmergeConversation(workspaceId);
  const [operation, setOperation] = useState(() => crypto.randomUUID());
  const source = preview.data?.sources[0] ?? merge.source;
  async function restore() {
    if (!preview.data) return;
    try {
      await mutation.mutateAsync({
        id: merge.id,
        token: preview.data.preview_token,
        operation,
        restoreMailbox,
      });
      onOpenChange(false);
      toast.success("Conversation restored");
    } catch {
      /* Inline retry keeps the restore decision visible. */
    }
  }
  return (
    <Dialog
      open={open}
      onOpenChange={(value) => {
        if (!mutation.isPending) onOpenChange(value);
      }}
    >
      <DialogContent>
        <DialogHeader>
          <DialogTitle>Unmerge conversation?</DialogTitle>
          <DialogDescription>
            Replies sent while merged stay in both histories. AI stays paused.
          </DialogDescription>
        </DialogHeader>
        {source && (
          <div className="space-y-3 py-2">
            <ConversationPreviewHeading conversation={source} showCustomer />
            <p className="text-sm text-muted-foreground">
              Restore to{" "}
              <span className="text-foreground">
                {source.mailbox_name || "Shared Inbox"}
              </span>
            </p>
          </div>
        )}
        {(preview.data?.restore_inbox_required || restoreMailbox) && (
          <QuietDropdown
            label="Restore to inbox"
            selected={restoreMailbox ? [restoreMailbox] : []}
            options={[
              { value: "shared", label: "Shared Inbox" },
              ...(scopes?.mailboxes ?? []).map((m) => ({
                value: m.id,
                label: m.name,
              })),
            ]}
            onSelect={setRestoreMailbox}
            trigger={
              <Button variant="outline">
                {source?.mailbox_name ||
                  (restoreMailbox === "shared"
                    ? "Shared Inbox"
                    : "Choose an inbox")}
              </Button>
            }
          />
        )}
        {(preview.error || mutation.error) && (
          <div role="alert" className="text-sm text-destructive">
            {(mutation.error ?? preview.error)?.message}
            <Button
              variant="ghost"
              size="sm"
              onClick={() => {
                mutation.reset();
                setOperation(crypto.randomUUID());
                void preview.refetch();
              }}
            >
              Review again
            </Button>
          </div>
        )}
        <DialogFooter>
          <Button
            variant="outline"
            disabled={mutation.isPending}
            onClick={() => onOpenChange(false)}
          >
            Cancel
          </Button>
          <Button
            disabled={
              !preview.data?.preview_token ||
              preview.data.restore_inbox_required ||
              preview.isFetching ||
              preview.isError ||
              mutation.isPending ||
              mutation.isError
            }
            onClick={() => void restore()}
          >
            {mutation.isPending ? "Restoring…" : "Unmerge"}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}

export function MergedConversationContinuation({
  conversation,
}: {
  conversation: SupportConversation;
}) {
  const slug = useWorkspaceStore((s) => s.currentWorkspace?.slug ?? "");
  return (
    <div className="flex items-center justify-between gap-3 border-t px-4 py-3 text-sm">
      <span className="text-muted-foreground">
        This conversation was merged.
      </span>
      {conversation.merge_continuation_id ? (
        <Button variant="outline" size="sm" asChild>
          <Link
            to="/w/$slug/support/$conversationId"
            params={{
              slug,
              conversationId: conversation.merge_continuation_id,
            }}
          >
            Open active conversation
          </Link>
        </Button>
      ) : (
        <span className="text-xs text-muted-foreground">
          The active inbox is restricted.
        </span>
      )}
    </div>
  );
}
