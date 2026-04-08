# @helpin-ai/nextjs

Helpin for Next.js. Drop-in analytics, chat widget control, and pageview tracking — with middleware support for server-side events.

## Installation

```bash
npm install @helpin-ai/nextjs @helpin-ai/sdk-js
```

## Quick Start

Create a client component that initializes Helpin and wraps your app:

```tsx
'use client';

import { useMemo } from 'react';
import { createClient, HelpinProvider, usePageView } from '@helpin-ai/nextjs';

export function Providers({ children }: { children: React.ReactNode }) {
  const helpinClient = useMemo(
    () =>
      createClient({
        widgetKey: process.env.NEXT_PUBLIC_HELPIN_WIDGET_KEY!,
        host: process.env.NEXT_PUBLIC_HELPIN_HOST!,
        autoBoot: false,
      }),
    [],
  );

  usePageView(helpinClient);

  return <HelpinProvider client={helpinClient}>{children}</HelpinProvider>;
}
```

> `createClient(...)` is browser-only and returns `null` during SSR. Always call it from a client component.

## `useHelpin()`

The hook provides analytics, user identification, and widget control from any component in the tree:

```tsx
'use client';

import { useEffect } from 'react';
import { useHelpin } from '@helpin-ai/nextjs';

export function BillingCTA() {
  const { id, track, lead, set, show } = useHelpin();

  useEffect(() => {
    void id({
      id: 'user_123',
      email: 'jane@example.com',
      name: 'Jane Doe',
    });
    set({ app: 'web' });
  }, [id, set]);

  return (
    <>
      <button onClick={() => track('billing_cta_clicked', { source: 'hero' })}>
        Talk to Sales
      </button>
      <button onClick={show}>Chat with us</button>
    </>
  );
}
```

### Available methods

**Analytics**

| Method | Signature | Description |
| --- | --- | --- |
| `trackPageView` | `() => void` | Send a pageview event |
| `id` | `(userData, doNotSendEvent?) => Promise<void>` | Identify the current user |
| `track` | `(eventName, payload?) => void` | Track a custom event |
| `lead` | `(payload, directSend?) => void` | Track a lead event |
| `rawTrack` | `(payload) => void` | Send a raw event payload |
| `set` | `(properties, opts?) => void` | Set global or event-scoped properties |
| `unset` | `(propertyName, opts?) => void` | Remove a property added with `set(...)` |

**Widget**

| Method | Signature | Description |
| --- | --- | --- |
| `show` | `() => void` | Boot the widget if needed and open it |
| `hide` | `() => void` | Close the widget |
| `toggle` | `() => void` | Toggle the widget open or closed |
| `showMessages` | `() => void` | Open the messages view |
| `showNewMessage` | `(content?) => void` | Start a new conversation |
| `shutdown` | `() => void` | End the session and unmount the widget |

For the complete client API (`boot`, `group`, `reset`, `setUserId`, `getConfig`, `getLogger`), use the object returned by `createClient(...)` directly.

## `usePageView()`

Tracks client-side route changes automatically. Optionally run setup logic or attach extra data before each pageview fires:

```tsx
'use client';

import { createClient, usePageView } from '@helpin-ai/nextjs';

const helpinClient = createClient({
  widgetKey: process.env.NEXT_PUBLIC_HELPIN_WIDGET_KEY!,
  host: process.env.NEXT_PUBLIC_HELPIN_HOST!,
});

export function PageViewTracker() {
  usePageView(helpinClient, {
    before: (client) => {
      void client.id({ id: 'user_123', email: 'jane@example.com' });
    },
    payload: {
      framework: 'nextjs',
    },
  });

  return null;
}
```

| Option | Type | Description |
| --- | --- | --- |
| `before` | `(helpin) => void` | Runs before each pageview event |
| `typeName` | `string` | Custom event name (default: `pageview`) |
| `payload` | `EventPayload` | Extra fields merged into the payload |

## Server-Side Tracking

For server-side analytics in middleware, route handlers, or server actions, pair the core SDK with `middlewareEnv(...)`. It extracts request metadata and manages the anonymous visitor ID cookie:

```ts
import { NextRequest, NextResponse } from 'next/server';
import { helpinClient } from '@helpin-ai/sdk-js';
import { middlewareEnv } from '@helpin-ai/nextjs';

const client = helpinClient({
  widgetKey: process.env.NEXT_PUBLIC_HELPIN_WIDGET_KEY!,
  host: process.env.NEXT_PUBLIC_HELPIN_HOST!,
});

export function middleware(req: NextRequest) {
  const res = NextResponse.next();
  const env = middlewareEnv(req, res);

  client?.track('pageview', {
    ...env.describeClient(),
    source_ip: env.getSourceIp(),
    anonymous_id: env.getAnonymousId({
      name: 'helpin_aid',
      domain: '.example.com',
    }),
  });

  return res;
}
```

| Method | Description |
| --- | --- |
| `getAnonymousId({ name, domain? })` | Return or create the anonymous visitor ID cookie |
| `getSourceIp()` | Extract the client IP from request headers |
| `describeClient()` | Build a `ClientProperties` object from the request |

## Client vs. Server

| Context | What to use |
| --- | --- |
| Browser (analytics + widget) | `createClient(...)` from `@helpin-ai/nextjs` |
| Server (middleware, route handlers, server actions) | `helpinClient(...)` from `@helpin-ai/sdk-js` + `middlewareEnv(...)` |

The chat widget boots automatically in the browser when `widgetKey` and `host` are set. Pass `autoBoot: false` to keep it dormant until you call `show()` or `showNewMessage()` — useful for custom launchers.

## Exports

| Export | Description |
| --- | --- |
| `createClient` | Browser-only client factory |
| `HelpinProvider` | React context provider |
| `HelpinContext` | Raw React context (for advanced use) |
| `useHelpin` | Analytics and widget hook |
| `usePageView` | Automatic pageview tracking hook |
| `middlewareEnv` | Next.js middleware helper |

## Development

```bash
pnpm --filter @helpin-ai/nextjs build
pnpm --filter @helpin-ai/nextjs test
```
