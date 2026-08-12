import { AskAgentsDock } from '@/components/agents/AskAgentsDock';
import type { CommandBarPageContext } from '@/lib/pmTypes';

export function SupportAgentSidebar({ context, onBack }: { context: CommandBarPageContext; onBack: () => void }) {
  return (
    <div className="flex h-full w-[420px] min-w-0 animate-in flex-col border-l bg-background fade-in slide-in-from-right-2 duration-200 ease-out motion-reduce:animate-none">
      <AskAgentsDock presentation="embedded" requiredPageContext={context} onClose={onBack} />
    </div>
  );
}
