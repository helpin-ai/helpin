interface ApprovalAttentionBannerProps {
  onReview: () => void;
}

/** Keeps an off-screen approval visible without forcing the transcript to jump. */
export function ApprovalAttentionBanner({ onReview }: ApprovalAttentionBannerProps) {
  return (
    <button
      type="button"
      onClick={onReview}
      className="mx-3 mb-2 flex shrink-0 items-center gap-2 rounded-lg border border-amber-300/70 bg-amber-50 px-3 py-2 text-left text-xs text-amber-950 shadow-sm transition hover:bg-amber-100 dark:border-amber-800/70 dark:bg-amber-950/40 dark:text-amber-100 dark:hover:bg-amber-950/60"
    >
      <span className="h-2 w-2 shrink-0 rounded-full bg-amber-500" aria-hidden />
      <span className="min-w-0 flex-1 font-semibold">Agent needs your approval</span>
      <span className="shrink-0 font-medium underline-offset-2 hover:underline">Review</span>
    </button>
  );
}
