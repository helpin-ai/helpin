import type { RunSteeringCommand } from '@/lib/services/dockChatService';

export function RunSteeringReceipt({ command }: { command: RunSteeringCommand | null }) {
  if (!command) return null;
  return <div className="px-5 py-2 text-sm text-muted-foreground">
    <p role="status">{command.state === 'accepted' ? 'Instruction queued for the next safe boundary.' : command.state === 'applied' ? 'Instruction applied to this run.' : 'Instruction was not applied. Review the run before retrying.'}</p>
    <details><summary>View instruction</summary><p className="whitespace-pre-wrap">{command.content}</p></details>
  </div>;
}
