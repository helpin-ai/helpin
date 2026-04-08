# @helpin-ai/nextjs

Next.js bindings for the Helpin SDK. Adds React context, pageview tracking, and middleware helpers designed for the App Router.

## Installation

```bash
npm install @helpin-ai/nextjs @helpin-ai/sdk-js
```

## Quick Start (App Router)

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

> **Note:** `createClient(...)` is browser-only and returns `null` during SSR. Always initialize it from a client component.

## Tracking with `useHelpin()`

Once the provider is in place, use the `useHelpin()` hook anywhere in your component tree:

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

| Method | Signature | Description |
| --- | --- | --- |
| `trackPageView` | `() => void` | Send a `pageview` event |
| `id` | `(userData, doNotSendEvent?) => Promise<void>` | Identify the current user |
| `track` | `(eventName, payload?) => void` | Track a custom event |
| `lead` | `(payload, directSend?) => void` | Track a lead event |
| `show` | `() => void` | Boot the widget if needed and open it |
| `hide` | `() => void` | Close the widget |
| `toggle` | `() => void` | Toggle the widget open or closed |
| `showMessages` | `() => void` | Open the widget to the messages view |
| `showNewMessage` | `(content?) => void` | Start a new conversation |
| `shutdown` | `() => void` | Revoke the widget session and unmount it |
| `rawTrack` | `(payload) => void` | Send a raw event payload |
| `set` | `(properties, opts?) => void` | Set global or event-scoped properties |
| `unset` | `(propertyName, opts?) => void` | Remove a property added with `set(...)` |

## Pageview Tracking

`usePageView` tracks client-side route changes automatically. You can customize the event or run setup logic before each pageview fires:

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
| `typeName` | `string` | Override the event name (default: `pageview`) |
| `payload` | `EventPayload` | Extra fields merged into the pageview payload |

## Server-Side Tracking with Middleware

For server-side analytics — route handlers, middleware, or server actions — use `middlewareEnv(...)` alongside the core SDK. It collects request metadata and manages the anonymous visitor ID cookie.

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
| `getAnonymousId({ name, domain? })` | Returns or creates the anonymous visitor ID cookie |
| `getSourceIp()` | Extracts the client IP from request headers |
| `describeClient()` | Returns a `ClientProperties`-shaped object for the current request |

## Client vs. Server

| Use case | What to use |
| --- | --- |
| Client-side analytics and widget | `createClient(...)` from `@helpin-ai/nextjs` |
| Server-side tracking (middleware, route handlers, server actions) | `helpinClient(...)` from `@helpin-ai/sdk-js` + `middlewareEnv(...)` |

When `widgetKey` and `host` are provided, the browser client automatically boots the chat widget through `@helpin-ai/sdk-js`. Set `autoBoot: false` to keep the widget dormant until you call `show()` or `showNewMessage()`.

## Full SDK API

`useHelpin()` exposes the most common tracking and widget methods. For the complete client API, hold onto the reference returned by `createClient(...)` — it also supports `boot(...)`, `group(...)`, `reset(...)`, `setUserId(...)`, `getConfig()`, and `getLogger()`.

## Widget Controls

Widget control is available directly from `useHelpin()` and the client returned by `createClient(...)`. For custom launchers, initialize with `autoBoot: false` and call `show()` from your button click handler.

## Exports

| Export | Description |
| --- | --- |
| `createClient` | Browser-only Helpin client factory |
| `HelpinProvider` | React context provider |
| `HelpinContext` | Raw React context |
| `useHelpin` | Tracking hook |
| `usePageView` | Client-side pageview hook |
| `middlewareEnv` | Next.js middleware helper |

## Development

```bash
pnpm --filter @helpin-ai/nextjs build
pnpm --filter @helpin-ai/nextjs test
```
