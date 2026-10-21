import { useEffect } from 'react';
import { useHelpin } from '@helpin-ai/react';
import { useLocation } from '@tanstack/react-router';

// eslint-disable-next-line react-refresh/only-export-components
export function isSupportModulePath(pathname: string) {
  return /^\/w\/[^/]+\/support(?:\/|$)/.test(pathname) || /^\/portal(?:\/|$)/.test(pathname);
}

export function HelpinWidgetVisibility() {
  const { pathname } = useLocation();
  const { hide, show } = useHelpin();

  useEffect(() => {
    if (isSupportModulePath(pathname)) hide();
    else show();
  }, [hide, pathname, show]);

  return null;
}
