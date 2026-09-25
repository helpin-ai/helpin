import { createContext, useCallback, useContext, useEffect, useRef, useState, type ReactNode } from 'react';

const InteractionContext = createContext<((id: string, prompt: ReactNode) => void) | null>(null);

/** Let delegated runs present their questions in the same dock overlay. */
export function DockInteractionPrompt({ id, children }: { id: string; children: ReactNode }) {
  const register = useContext(InteractionContext);
  useEffect(() => register?.(id, children), [register, id, children]);
  useEffect(() => () => register?.(id, null), [register, id]);
  return register ? null : children;
}

/** A dock-scoped prompt: the conversation keeps its layout and scroll position. */
export function DockInteractionLayer({ children, prompt, interactionId, active = true }: {
  children: ReactNode;
  prompt?: ReactNode;
  interactionId?: string;
  active?: boolean;
}) {
  const promptRef = useRef<HTMLDivElement>(null);
  const [delegated, setDelegated] = useState<Map<string, ReactNode>>(() => new Map());
  const register = useCallback((id: string, content: ReactNode) => {
    setDelegated(current => {
      if (current.get(id) === content || (!content && !current.has(id))) return current;
      const next = new Map(current);
      if (content) next.set(id, content);
      else next.delete(id);
      return next;
    });
  }, []);
  const nextDelegated = delegated.entries().next().value;
  const displayedPrompt = prompt || nextDelegated?.[1];
  const displayedId = prompt ? interactionId : nextDelegated?.[0];
  const open = !!displayedPrompt;

  useEffect(() => {
    if (!open || !active) return;
    const previous = document.activeElement;
    const panel = promptRef.current;
    panel?.focus({ preventScroll: true });
    return () => {
      if (previous instanceof HTMLElement && previous.isConnected
        && (document.activeElement === document.body || panel?.contains(document.activeElement))) {
        previous.focus({ preventScroll: true });
      }
    };
  }, [open, displayedId, active]);

  return (
    <InteractionContext.Provider value={register}>
      <div className="relative isolate flex min-h-0 flex-1 flex-col">
        <div className="flex min-h-0 flex-1 flex-col" inert={open} aria-hidden={open || undefined} data-dock-interaction-background>
          {children}
        </div>
        {open && (
          <div className="absolute inset-0 z-20 flex flex-col overflow-y-auto overscroll-contain bg-background/65 px-3 py-3 sm:px-5" data-dock-interaction-overlay>
            <div
              key={displayedId}
              ref={promptRef}
              role="dialog"
              aria-label="Agent needs your response"
              tabIndex={-1}
              className="mt-auto min-w-0 shrink-0 rounded-[14px] border border-border bg-popover p-4 text-popover-foreground shadow-sm outline-none"
            >
              {displayedPrompt}
            </div>
          </div>
        )}
      </div>
    </InteractionContext.Provider>
  );
}
