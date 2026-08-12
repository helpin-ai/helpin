import { AskAgentsDock } from '@/components/agents/AskAgentsDock';
import type { CommandBarPageContext } from '@/lib/pmTypes';

export function SupportAgentSidebar({ context, onBack }: { context: CommandBarPageContext; onBack: () => void }) {
  return (
    <div className="flex h-full w-full min-w-0 flex-col bg-background">
      <AskAgentsDock presentation="embedded" requiredPageContext={context} onClose={onBack} />
    </div>
  );
}
