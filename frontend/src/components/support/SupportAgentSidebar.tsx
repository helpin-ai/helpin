import { AskAgentsDock } from '@/components/agents/AskAgentsDock';
import type { CommandBarPageContext } from '@/lib/pmTypes';
import { useSupportInboxStore } from '@/stores/supportInboxStore';

export function SupportAgentSidebar({ context, active, onBack }: { context: CommandBarPageContext; active: boolean; onBack: () => void }) {
  const agentDraft = useSupportInboxStore((state) => state.agentDraft);
  const clearAgentDraft = useSupportInboxStore((state) => state.clearAgentDraft);
  const initialDraft = agentDraft?.conversationId === context.entity_id ? agentDraft.text : undefined;
  return (
    <div className="flex h-full w-full min-w-0 flex-col bg-background">
      <AskAgentsDock
        presentation="embedded"
        requiredPageContext={context}
        associatedSupportConversationId={context.entity_id}
        active={active}
        onClose={onBack}
        initialDraft={initialDraft}
        onInitialDraftConsumed={clearAgentDraft}
      />
    </div>
  );
}
