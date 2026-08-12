'use client';

import { useMemo, type ReactNode } from 'react';
import { createClient, HelpinProvider, usePageView } from '@helpin-ai/nextjs';

const DEFAULT_WIDGET_KEY = 'b86e7c64e7c93517f0c2f395c7b98701';
const DEFAULT_HELPIN_HOST = 'https://client.helpin.ai';

export function HelpinWidgetProvider({ children }: { children: ReactNode }) {
  const client = useMemo(
    () => createClient({
      widgetKey: process.env.NEXT_PUBLIC_HELPIN_WIDGET_KEY || DEFAULT_WIDGET_KEY,
      host: process.env.NEXT_PUBLIC_HELPIN_HOST || DEFAULT_HELPIN_HOST,
    }),
    [],
  );

  usePageView(client);

  return <HelpinProvider client={client}>{children}</HelpinProvider>;
}
