import { useLayoutEffect, useRef, useState } from 'react';

const WORD_DELAY_MS = 110;
const WORD_DURATION_MS = 150;

/** Reveals a changed chat title word-by-word without changing its layout. */
export function AnimatedDockChatTitle({ title }: { title: string }) {
  const previousTitleRef = useRef(title);
  const [animatingTitle, setAnimatingTitle] = useState<string | null>(null);

  useLayoutEffect(() => {
    if (previousTitleRef.current === title) return;
    previousTitleRef.current = title;
    setAnimatingTitle(title);
    const wordCount = Math.max(1, title.trim().split(/\s+/).length);
    const timer = window.setTimeout(
      () => setAnimatingTitle(null),
      (wordCount - 1) * WORD_DELAY_MS + WORD_DURATION_MS,
    );
    return () => window.clearTimeout(timer);
  }, [title]);

  if (animatingTitle !== title) return title;
  const words = title.trim().split(/\s+/);
  return (
    <>
      <span className="sr-only">{title}</span>
      <span aria-hidden>
        {words.map((word, index) => (
          <span key={`${word}-${index}`}>
            <span
              className="agent-dock-title-word inline-block"
              style={{ animationDelay: `${index * WORD_DELAY_MS}ms` }}
            >
              {word}
            </span>
            {index < words.length - 1 ? ' ' : null}
          </span>
        ))}
      </span>
    </>
  );
}
