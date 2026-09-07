import { useLayoutEffect, useRef, type RefObject } from 'react';
import type { SupportMessage } from '@/lib/pmTypes';
import { joinedReplyClientMessageID, supportMessageRenderKey } from '@/lib/supportMessagePages';
import { isNearThreadBottom } from './threadAutoScroll';

interface RowPosition {
  node: HTMLElement;
  top: number;
  bottom: number;
}

interface ThreadPosition {
  conversationId: string | null;
  joinedIDs: Set<string>;
  rows: Map<string, RowPosition>;
  anchorKey?: string;
}

/** Preserve the reading position when a first-reply status joins the timeline. */
export function useJoinedMessagePosition(
  scrollAreaRef: RefObject<HTMLDivElement | null>,
  conversationId: string | null,
  messages: SupportMessage[],
  prependingRef: RefObject<{ scrollHeight: number; scrollTop: number } | null>,
) {
  const previous = useRef<ThreadPosition | null>(null);
  useLayoutEffect(() => {
    const viewport = scrollAreaRef.current?.querySelector<HTMLElement>('[data-slot="scroll-area-viewport"]');
    if (!viewport) {
      previous.current = null;
      return;
    }
    const joinedIDs = new Set(messages.filter((message) => message.system_event_type === 'teammate_joined').map((message) => message.id));
    const capture = (): ThreadPosition => {
      const top = viewport.getBoundingClientRect().top;
      const rows = new Map<string, RowPosition>();
      for (const node of viewport.querySelectorAll<HTMLElement>('[data-support-message-key]')) {
        const rect = node.getBoundingClientRect();
        rows.set(node.dataset.supportMessageKey!, { node, top: rect.top - top, bottom: rect.bottom - top });
      }
      const entries = [...rows];
      const anchor = isNearThreadBottom(viewport) ? entries[entries.length - 1]
        : entries.find(([, row]) => row.bottom > 0 && row.top < viewport.clientHeight);
      return { conversationId, joinedIDs, rows, anchorKey: anchor?.[0] };
    };

    const before = previous.current;
    let after = capture();
    const joinedExistingReply = before && messages.some((message) => {
      const clientID = joinedReplyClientMessageID(message);
      return clientID && !before.joinedIDs.has(message.id) && before.rows.has(`client:${clientID}`);
    });
    const newReply = before && messages.some((message) => message.message_type !== 'system'
      && !before.rows.has(supportMessageRenderKey(message)));
    // Batches that include genuinely new replies retain normal bottom-follow.
    if (!prependingRef.current && before?.conversationId === conversationId && joinedExistingReply && !newReply) {
      const oldAnchor = before.anchorKey ? before.rows.get(before.anchorKey) : undefined;
      const newAnchor = before.anchorKey ? after.rows.get(before.anchorKey) : undefined;
      if (oldAnchor && newAnchor) {
        // The normal bottom-follow effect may already have compensated. Apply
        // only the remaining displacement, including for readers in history.
        viewport.scrollTop += newAnchor.top - oldAnchor.top;
        after = capture();
      }
      const remainingAnchor = before.anchorKey ? after.rows.get(before.anchorKey) : undefined;
      if (oldAnchor && remainingAnchor && Math.abs(oldAnchor.top - remainingAnchor.top) > 0.5
        && !window.matchMedia?.('(prefers-reduced-motion: reduce)').matches) {
        // A short thread cannot scroll enough to absorb added content. Move
        // existing rows smoothly from their old position; never delay the reply.
        for (const [key, row] of after.rows) {
          const oldRow = before.rows.get(key);
          if (!oldRow || oldRow.bottom <= 0 || oldRow.top >= viewport.clientHeight) continue;
          const delta = oldRow.top - row.top;
          if (Math.abs(delta) > 0.5) row.node.animate?.([
            { transform: `translateY(${delta}px)` }, { transform: 'translateY(0)' },
          ], { duration: 160, easing: 'ease-out' });
        }
      }
    }
    previous.current = after;
    const onScroll = () => { previous.current = capture(); };
    viewport.addEventListener('scroll', onScroll, { passive: true });
    return () => viewport.removeEventListener('scroll', onScroll);
  }, [conversationId, messages, prependingRef, scrollAreaRef]);
}
