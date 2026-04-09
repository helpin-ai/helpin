'use client';

import { createClient, HelpinProvider } from '@helpin-ai/nextjs';
import { ReactNode, useMemo } from 'react';

const HELPIN_CONFIG = {
  widgetKey: 'YOUR_WIDGET_KEY',
  host: 'https://your-helpin-host.com',
};

export default function HelpinClientProvider({
  children,
}: {
  children: ReactNode;
}) {
  const client = useMemo(() => createClient(HELPIN_CONFIG), []);

  return <HelpinProvider client={client}>{children}</HelpinProvider>;
}
