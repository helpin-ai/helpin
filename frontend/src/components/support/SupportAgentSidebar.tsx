import { AskAgentsDock } from '@/components/agents/AskAgentsDock';
import type { CommandBarPageContext } from '@/lib/pmTypes';

export function SupportAgentSidebar({ context, active, onBack }: { context: CommandBarPageContext; active: boolean; onBack: () => void }) {
  return (
    <div className="flex h-full w-full min-w-0 flex-col bg-background">
      <AskAgentsDock
        presentation="embedded"
        requiredPageContext={context}
        associatedSupportConversationId={context.entity_id}
        active={active}
        onClose={onBack}
      />
    </div>
  );
}
