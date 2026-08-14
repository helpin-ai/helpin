import { useEffect } from 'react';

const BASE = 'Helpin';

export function useTitle(title?: string) {
  useEffect(() => {
    if (title === undefined) return;
    document.title = `${title} · ${BASE}`;
  }, [title]);
}
