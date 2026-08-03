export function SidebarRunsButton() {
  const title = 'Chats';

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
      <svg
        xmlns="http://www.w3.org/2000/svg"
        viewBox="0 0 24 24"
        fill="none"
        stroke="currentColor"
        strokeWidth={2}
        strokeLinecap="round"
        strokeLinejoin="round"
        aria-hidden
        className="h-3.5 w-3.5"
      >
        <path d="M3 12h3.28a1 1 0 0 1 .948.684l2.298 7.934a.5.5 0 0 0 .96-.044L13.82 4.771A1 1 0 0 1 14.792 4H21" />
      </svg>
    </button>
  );
}
