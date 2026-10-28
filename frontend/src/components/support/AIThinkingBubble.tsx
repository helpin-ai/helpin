import { AskAgentAvatar } from '@/components/agents/AskAgentAvatar';
import { useSupportPresenceStore } from '@/stores/supportPresenceStore';
import { HELPIN_AI_DISPLAY_NAME } from './helpers';

/**
 * AIThinkingBubble tells teammates the AI is working on this conversation,
 * in the place and style of a teammate's typing bubble, so nobody replies
 * over it. It stays hidden while a running code-check card says the same.
 */
export function AIThinkingBubble({ conversationId, hidden = false }: { conversationId: string | null; hidden?: boolean }) {
  const thinking = useSupportPresenceStore((s) => (conversationId ? Boolean(s.aiThinking[conversationId]) : false));
  if (!thinking || hidden) return null;
  return (
    <div role="status" className="mt-2 flex justify-end animate-in fade-in slide-in-from-right-2 duration-200">
      <div className="max-w-[70%]">
        <div className="mb-1 pr-1 text-right">
          <span className="text-[11px] font-medium text-blue-600/70 dark:text-blue-400/70">
            {HELPIN_AI_DISPLAY_NAME} is thinking
            <span className="ml-0.5 inline-flex" aria-hidden="true">
              <span className="animate-bounce [animation-delay:0ms] [animation-duration:1s] motion-reduce:animate-none">.</span>
              <span className="animate-bounce [animation-delay:200ms] [animation-duration:1s] motion-reduce:animate-none">.</span>
              <span className="animate-bounce [animation-delay:400ms] [animation-duration:1s] motion-reduce:animate-none">.</span>
            </span>
          </span>
        </div>
        <div className="flex justify-end">
          <div aria-hidden="true" className="rounded-2xl rounded-br-sm bg-blue-100/60 px-3.5 py-2 text-sm leading-relaxed text-blue-600/70 dark:bg-blue-900/20 dark:text-blue-300/70">
            <span className="flex gap-0.5 opacity-50">
              <span className="animate-bounce [animation-delay:0ms] motion-reduce:animate-none">·</span>
              <span className="animate-bounce [animation-delay:150ms] motion-reduce:animate-none">·</span>
              <span className="animate-bounce [animation-delay:300ms] motion-reduce:animate-none">·</span>
            </span>
          </div>
        </div>
      </div>
      <div className="ml-2 flex w-7 shrink-0 flex-col justify-end">
        <AskAgentAvatar plateStyle="solid" radius={50} className="h-7 w-7" />
      </div>
    </div>
  );
}
