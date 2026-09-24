export function AskAgentWorkAnimation({ className = 'h-5 w-5' }: { className?: string }) {
  return (
    <span className={`agent-square-snake relative inline-block shrink-0 ${className}`} aria-hidden="true" data-agent-work-loader>
      <span className="agent-square-snake-piece" />
      <span className="agent-square-snake-piece" />
      <span className="agent-square-snake-piece" />
      <span className="agent-square-snake-piece" />
    </span>
  );
}
