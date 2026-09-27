export function AskAgentWorkAnimation({ className = 'h-3.5 w-3.5' }: { className?: string }) {
  return (
    <span className={`agent-work-orbit relative inline-block shrink-0 text-quiet-text-secondary ${className}`} aria-hidden="true" data-agent-work-loader />
  );
}
