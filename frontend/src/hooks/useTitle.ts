import { useEffect } from 'react';

const BASE = 'Helpin';

export function useTitle(title?: string) {
  useEffect(() => {
    if (title === undefined) return;
    const prev = document.title;
    document.title = `${title} · ${BASE}`;
    return () => { document.title = prev; };
  }, [title]);
}
