export function createDebouncedBoardFetchScheduler<TArgs>(
  delayMs: number,
  run: (args: TArgs) => void | Promise<void>,
) {
  let timeoutId: ReturnType<typeof setTimeout> | null = null;

  return {
    schedule(args: TArgs) {
      if (timeoutId) {
        clearTimeout(timeoutId);
      }

      timeoutId = setTimeout(() => {
        timeoutId = null;
        void run(args);
      }, delayMs);
    },
    cancel() {
      if (!timeoutId) {
        return;
      }

      clearTimeout(timeoutId);
      timeoutId = null;
    },
  };
}
