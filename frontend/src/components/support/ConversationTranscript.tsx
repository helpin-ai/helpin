import { useEffect, useMemo, useRef } from "react";
import { useConversationMessages } from "@/hooks/queries/useSupport";
import { useSharedConversationHistory } from "@/hooks/queries/useSupportMerge";
import { flattenSupportMessagePages } from "@/lib/supportMessagePages";
import type { SupportConversation } from "@/lib/pmTypes";
import { Button } from "@/components/ui/button";
import { useConversationMerges } from "@/hooks/queries/useSupportMerge";
import { isShownInSupportThread, SupportThreadMessages } from "./SupportThreadMessages";

// Renders the live thread's own message list, without the thread's header or
// composer, a presence broadcaster, or marking customer messages read.
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
        isShownInSupportThread,
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
      className="h-full min-h-0 overflow-y-auto overscroll-contain bg-muted/20 px-4 pb-6 pt-2 dark:bg-sidebar"
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
            <SupportThreadMessages
              workspaceId={workspaceId}
              conversationId={conversation.id}
              conversation={conversation}
              messages={messages}
              merges={merges}
              readOnly
              canUnmerge={false}
            />
          )}
        </>
      )}
    </div>
  );
}
