import { useEffect, useMemo, useRef } from "react";
import { useConversationMessages } from "@/hooks/queries/useSupport";
import { useSharedConversationHistory } from "@/hooks/queries/useSupportMerge";
import { useWorkspaceMembers } from "@/hooks/queries/useWorkspaces";
import { useCachedSupportTranslations } from "@/hooks/queries/useSupportTranslation";
import { flattenSupportMessagePages } from "@/lib/supportMessagePages";
import type {
  SupportConversation,
  SupportConversationMerge,
  SupportMessage,
} from "@/lib/pmTypes";
import { useWorkspaceStore } from "@/stores/workspaceStore";
import { resolveTeamMemberAvatarSrc } from "@/lib/teamMemberAvatar";
import { Button } from "@/components/ui/button";
import { TranslatedMessageBubble } from "./TranslatedMessageBubble";
import { groupSupportTimeline } from "./groupSupportTimeline";
import { ConversationMergeHistoryEvent } from "./ConversationMergeHistory";
import { useConversationMerges } from "@/hooks/queries/useSupportMerge";
import { getSupportReceiptStatus } from "./helpers";

// Uses exactly the message renderer and grouping of the live thread, without
// mounting a composer, presence broadcaster, or marking customer messages read.
export function ConversationTranscript({
  workspaceId,
  conversation,
  mergeId,
}: {
  workspaceId: string;
  conversation: SupportConversation;
  mergeId?: string;
}) {
  const { data: merges = [] } = useConversationMerges(
    workspaceId,
    mergeId ? null : conversation.id,
  );
  const original = useConversationMessages(
    workspaceId,
    mergeId ? null : conversation.id,
  );
  const shared = useSharedConversationHistory(
    workspaceId,
    conversation.id,
    mergeId,
  );
  const query = mergeId ? shared : original;
  const messages = useMemo(
    () =>
      flattenSupportMessagePages(query.data).filter(
        (m) => m.system_event_type !== "teammate_joined",
      ),
    [query.data],
  );
  const scroll = useRef<HTMLDivElement>(null);
  const initialScroll = useRef(false);
  useEffect(() => {
    initialScroll.current = false;
  }, [conversation.id, mergeId]);
  useEffect(() => {
    if (!query.isPending && !initialScroll.current && scroll.current) {
      scroll.current.scrollTop = scroll.current.scrollHeight;
      initialScroll.current = true;
    }
  }, [query.isPending, messages]);
  return (
    <div
      ref={scroll}
      className="h-full min-h-0 overflow-y-auto overscroll-contain bg-muted/20 px-4 py-3 dark:bg-sidebar"
      aria-label={`Conversation #${conversation.display_id} messages`}
    >
      {query.isPending ? (
        <p className="py-8 text-center text-sm text-muted-foreground">
          Loading messages…
        </p>
      ) : query.isError ? (
        <div role="alert" className="py-8 text-center">
          <p className="text-sm">Could not load this conversation.</p>
          <Button
            variant="ghost"
            size="sm"
            onClick={() => void query.refetch()}
          >
            Retry
          </Button>
        </div>
      ) : (
        <>
          {query.hasNextPage && (
            <div className="mb-4 text-center">
              <Button
                variant="ghost"
                size="sm"
                disabled={query.isFetchingNextPage}
                onClick={async () => {
                  const height = scroll.current?.scrollHeight ?? 0;
                  await query.fetchNextPage();
                  requestAnimationFrame(() => {
                    if (scroll.current)
                      scroll.current.scrollTop =
                        scroll.current.scrollHeight - height;
                  });
                }}
              >
                Load earlier messages
              </Button>
            </div>
          )}
          {messages.length === 0 ? (
            <p className="py-8 text-center text-sm text-muted-foreground">
              No messages yet.
            </p>
          ) : (
            <ConversationTranscriptMessages
              workspaceId={workspaceId}
              conversation={conversation}
              messages={messages}
              merges={merges}
            />
          )}
        </>
      )}
    </div>
  );
}

export function ConversationTranscriptMessages({
  workspaceId,
  conversation,
  messages,
  merges = [],
}: {
  workspaceId: string;
  conversation: SupportConversation;
  messages: SupportMessage[];
  merges?: SupportConversationMerge[];
}) {
  const slug = useWorkspaceStore((s) => s.currentWorkspace?.slug ?? "");
  const { data: members = [] } = useWorkspaceMembers(workspaceId);
  const ids = useMemo(
    () =>
      messages
        .filter((m) => m.message_type === "reply" && !m.is_internal)
        .map((m) => m.id),
    [messages],
  );
  const { data: translations = [] } = useCachedSupportTranslations(
    workspaceId,
    conversation.id,
    ids,
    Math.max(0, ...messages.map((m) => Date.parse(m.created_at))),
  );
  const byMessage = new Map(
    translations.map((t) => [
      t.purpose === "outgoing_reply" ? t.sent_message_id : t.source_message_id,
      t,
    ]),
  );
  const receipt = messages.findLast(
    (m) =>
      !m.is_internal &&
      m.message_type === "reply" &&
      m.sender_type !== "customer",
  );
  return groupSupportTimeline(messages, merges).map((item, index) => {
    if (item.type === "merge")
      return (
        <ConversationMergeHistoryEvent
          key={item.id}
          workspaceId={workspaceId}
          conversation={conversation}
          merges={item.merges}
          restored={item.restored}
          canUnmerge={false}
        />
      );
    if (item.type === "separator")
      return (
        <div
          key={`day-${index}`}
          className="py-4 text-center text-[11px] text-muted-foreground"
        >
          {item.label}
        </div>
      );
    const message = item.message;
    const member = members.find((m) => m.user_id === message.sender_user_id);
    const avatar = member
      ? resolveTeamMemberAvatarSrc({
          avatarUrl: member.avatar_url,
          avatarStyle: member.avatar_style,
          avatarSeed: member.avatar_seed,
          avatarBackgroundMode: member.avatar_background_mode,
          avatarBackgroundColor: member.avatar_background_color,
          fallbackSeed: member.full_name ?? member.email,
        })
      : undefined;
    return (
      <TranslatedMessageBubble
        readOnly
        key={message.id}
        workspaceId={workspaceId}
        message={message}
        cachedTranslation={byMessage.get(message.id)}
        isConsecutive={item.isConsecutive}
        isLastInGroup={item.isLastInGroup}
        source={conversation.source}
        customerDisplayName={
          conversation.customer_name || conversation.customer_email
        }
        customerEmail={conversation.customer_email}
        teammateDisplayName={member?.full_name}
        fallbackAvatarUrl={avatar}
        workspaceSlug={slug}
        linkedTaskId={conversation.linked_task_id}
        contactLastSeenAt={conversation.contact_last_seen_at}
        receiptStatus={
          receipt?.id === message.id
            ? getSupportReceiptStatus(message, conversation)
            : undefined
        }
      />
    );
  });
}
