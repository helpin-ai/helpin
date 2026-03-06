import { useEffect } from 'react';

const BASE = 'Helpin';

export function useTitle(title?: string) {
  useEffect(() => {
    document.title = title ? `${title} · ${BASE}` : BASE;
    return () => { document.title = BASE; };
  }, [title]);
}
