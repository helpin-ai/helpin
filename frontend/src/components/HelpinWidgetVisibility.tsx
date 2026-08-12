import { useEffect } from 'react';
import { useHelpin } from '@helpin-ai/react';
import { useLocation } from '@tanstack/react-router';

export function isSupportModulePath(pathname: string) {
  return /^\/w\/[^/]+\/support(?:\/|$)/.test(pathname);
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
