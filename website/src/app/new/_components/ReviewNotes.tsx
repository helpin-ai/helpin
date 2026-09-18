'use client';

import { createContext, useContext, useState } from 'react';

// Review notes render in development, or in any build with NEXT_PUBLIC_REVIEW_NOTES=1.
// Production exports without that flag never include them.
const NOTES_ENABLED =
  process.env.NODE_ENV !== 'production' || process.env.NEXT_PUBLIC_REVIEW_NOTES === '1';

const Ctx = createContext<{ shown: boolean; toggle: () => void }>({ shown: false, toggle: () => {} });

export function ReviewNotesProvider({ children }: { children: React.ReactNode }) {
  const [shown, setShown] = useState(true);
  return <Ctx.Provider value={{ shown: NOTES_ENABLED && shown, toggle: () => setShown((v) => !v) }}>{children}</Ctx.Provider>;
}

export function ReviewNote({ tag, children }: { tag: string; children: React.ReactNode }) {
  const { shown } = useContext(Ctx);
  if (!shown) return null;
  return (
    <div className="rn">
      <span className="tag">{tag}</span>
      {children}
    </div>
  );
}

export function Flag({ children }: { children: React.ReactNode }) {
  return <span className="tag">{children}</span>;
}

export function ReviewBanner() {
  const { shown } = useContext(Ctx);
  if (!shown) return null;
  return (
    <div className="banner">
      Homepage preview, September 18, 2026. Amber notes explain each section and flag what still needs a decision or a
      check. Frames are placeholders until screenshots from the seeded workspace replace them.
    </div>
  );
}

export function ReviewToggle() {
  const { shown, toggle } = useContext(Ctx);
  if (!NOTES_ENABLED) return null;
  return (
    <button type="button" className="toggle" onClick={toggle} aria-pressed={shown}>
      {shown ? 'Hide review notes' : 'Show review notes'}
    </button>
  );
}
