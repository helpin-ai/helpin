import { AskAgentAvatar } from '@/components/agents/AskAgentAvatar';

export function SidebarRunsButton() {
  const title = 'Ask Agent';

  const onClick = () => {
    window.dispatchEvent(
      new CustomEvent('helpin:ask-agents', { detail: { mode: 'runs' } }),
    );
  };

  return (
    <button
      type="button"
      aria-label={title}
      title={title}
      onClick={onClick}
      className="relative flex size-8 items-center justify-center rounded-md text-muted-foreground transition-colors hover:bg-muted/80 hover:text-foreground"
    >
      <AskAgentAvatar plateStyle="feather" className="h-6 w-6" />
    </button>
  );
}
