import { useMemo } from 'react';
import { useCachedSupportTranslations } from '@/hooks/queries/useSupportTranslation';
import { useWorkspaceMembers } from '@/hooks/queries/useWorkspaces';
import type { SupportConversation, SupportConversationMerge, SupportMessage } from '@/lib/pmTypes';
import { supportMessageRenderKey } from '@/lib/supportMessagePages';
import { resolveTeamMemberAvatarSrc } from '@/lib/teamMemberAvatar';
import { useWorkspaceStore } from '@/stores/workspaceStore';
import { ConversationMergeHistoryEvent } from './ConversationMergeHistory';
import { groupSupportTimeline } from './groupSupportTimeline';
import { getSupportReceiptStatus } from './helpers';
import { foldedSupportAICardIds } from './supportAIActivityModel';
import { isFollowUpStatusNote } from './supportFollowUpState';
import { DaySeparator } from './SupportDaySeparator';
import { TranslatedMessageBubble } from './TranslatedMessageBubble';

/** Messages a support thread shows; join notices and follow-up status notes stay hidden. */
export function isShownInSupportThread(message: SupportMessage): boolean {
  return !(message.message_type === 'system' && message.system_event_type === 'teammate_joined') && !isFollowUpStatusNote(message);
}

/**
 * The messages of a support thread: date lines, merge history and message
 * bubbles. The inbox thread and every read-only preview render this, so a
 * conversation looks the same wherever it is shown.
 */
export function SupportThreadMessages({
  workspaceId,
  conversationId,
  conversation,
  messages,
  merges,
  readOnly,
  canUnmerge,
  onEditNote,
  separatorRef,
}: {
  workspaceId: string;
  conversationId: string;
  conversation?: SupportConversation;
  messages: SupportMessage[];
  merges: SupportConversationMerge[];
  readOnly: boolean;
  canUnmerge: boolean;
  onEditNote?: (message: SupportMessage) => void;
  /** Receives each date line, so the thread can show the floating date. */
  separatorRef?: (index: number, node: HTMLDivElement | null) => void;
}) {
  const workspaceSlug = useWorkspaceStore((s) => s.currentWorkspace?.slug ?? '');
  const { data: members = [] } = useWorkspaceMembers(workspaceId);
  const memberNameByUserId = useMemo(() => new Map(members.map(member => [member.user_id, member.full_name])), [members]);
  const memberAvatarByUserId = useMemo(() => {
    const map = new Map<string, string>();
    for (const member of members) {
      const avatarSrc = resolveTeamMemberAvatarSrc({
        avatarUrl: member.avatar_url,
        avatarStyle: member.avatar_style,
        avatarSeed: member.avatar_seed,
        avatarBackgroundMode: member.avatar_background_mode,
        avatarBackgroundColor: member.avatar_background_color,
        fallbackSeed: member.full_name ?? member.email,
      });
      if (member.user_id && avatarSrc) {
        map.set(member.user_id, avatarSrc);
      }
    }
    return map;
  }, [members]);

  const translationIds = useMemo(() => messages.filter(m => m.message_type === 'reply' && !m.is_internal && !m.pending_send).map(m => m.id), [messages]);
  const cachedTranslations = useCachedSupportTranslations(workspaceId, conversationId, translationIds, Math.max(0, ...messages.filter(m => !m.is_internal).map(m => Date.parse(m.created_at)))).data;
  const translationMap = useMemo(() => new Map((cachedTranslations || []).map(t => [t.purpose === 'outgoing_reply' ? t.sent_message_id : t.source_message_id, t])), [cachedTranslations]);

  // The last outbound reply carries the delivered/read receipt.
  const receiptMessageId = useMemo(() => {
    for (let i = messages.length - 1; i >= 0; i--) {
      const msg = messages[i];
      if (!msg.is_internal && msg.sender_type !== 'customer' && (!msg.message_type || msg.message_type === 'reply')) {
        return msg.id;
      }
    }
    return null;
  }, [messages]);
  const receiptStatus = useMemo(() => {
    if (!receiptMessageId || !conversation) return null;
    const msg = messages.find((m) => m.id === receiptMessageId);
    return msg ? getSupportReceiptStatus(msg, conversation) : null;
  }, [receiptMessageId, conversation, messages]);

  const groupedMessages = useMemo(() => groupSupportTimeline(messages, merges), [messages, merges]);
  const foldedAICardIds = useMemo(() => foldedSupportAICardIds(messages), [messages]);

  return groupedMessages.map((item, idx) => {
    if (item.type === 'merge') return conversation ? <ConversationMergeHistoryEvent key={item.id} workspaceId={workspaceId} conversation={conversation} merges={item.merges} restored={item.restored} canUnmerge={canUnmerge} /> : null;
    if (item.type === 'separator') {
      return (
        <DaySeparator
          key={`sep-${idx}`}
          label={item.label}
          separatorRef={separatorRef ? (node) => separatorRef(idx, node) : undefined}
        />
      );
    }
    return (
      <div
        key={supportMessageRenderKey(item.message)}
        className="support-thread-message w-full min-w-0 max-w-full"
        data-support-message-id={item.message.id}
        data-support-message-key={supportMessageRenderKey(item.message)}
      >
        <TranslatedMessageBubble
          workspaceId={workspaceId}
          readOnly={readOnly}
          onEditNote={onEditNote}
          message={item.message}
          cachedTranslation={translationMap.get(item.message.id)}
          isConsecutive={item.isConsecutive}
          isLastInGroup={item.isLastInGroup}
          source={conversation?.source}
          contactLastSeenAt={conversation?.contact_last_seen_at}
          receiptStatus={item.message.id === receiptMessageId ? receiptStatus : undefined}
          teammateDisplayName={item.message.sender_user_id ? memberNameByUserId.get(item.message.sender_user_id) : undefined}
          customerDisplayName={conversation?.customer_name || conversation?.customer_email}
          customerEmail={conversation?.customer_email}
          workspaceSlug={workspaceSlug}
          linkedTaskId={conversation?.linked_task_id}
          folded={foldedAICardIds.has(item.message.id)}
          fallbackAvatarUrl={
            item.message.sender_user_id ? memberAvatarByUserId.get(item.message.sender_user_id) : undefined
          }
        />
      </div>
    );
  });
}
