# @helpin-ai/nextjs

Next.js helpers for `@helpin-ai/sdk-js`.

This package provides:

- `createClient(...)` for client-side Helpin initialization
- `HelpinProvider` and `useHelpin()` for React context access
- `usePageView(...)` for client-side route tracking
- `middlewareEnv(...)` for Next.js middleware request metadata

## Install

```bash
npm install @helpin-ai/nextjs @helpin-ai/sdk-js
```

## Client-Side Setup

`createClient(...)` in `@helpin-ai/nextjs` is browser-only. It returns `null` during SSR, so initialize it from a client component or client-only module.

### App Router Example

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
      }),
    [],
  );

  usePageView(helpinClient);

  return <HelpinProvider client={helpinClient}>{children}</HelpinProvider>;
}
```

### Using `useHelpin()`

```tsx
'use client';

import { useEffect } from 'react';
import { useHelpin } from '@helpin-ai/nextjs';

export function BillingCTA() {
  const { id, track, lead, set } = useHelpin();

  useEffect(() => {
    void id({
      id: 'user_123',
      email: 'jane@example.com',
      name: 'Jane Doe',
    });
    set({ app: 'web' });
  }, [id, set]);

  return (
    <button onClick={() => track('billing_cta_clicked', { source: 'hero' })}>
      Talk to Sales
    </button>
  );
}
```

Typed hook methods:

| Method | Signature | Description |
| --- | --- | --- |
| `trackPageView` | `() => void` | Sends a `pageview` event |
| `id` | `(userData, doNotSendEvent?) => Promise<void>` | Identify the current user |
| `track` | `(eventName, payload?) => void` | Track a custom event |
| `lead` | `(payload, directSend?) => void` | Track a lead event |
| `rawTrack` | `(payload) => void` | Send a raw event payload |
| `set` | `(properties, opts?) => void` | Set global or event-scoped properties |
| `unset` | `(propertyName, opts?) => void` | Remove a property set via `set(...)` |

## `usePageView(helpin, opts?)`

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

Options:

| Option | Type | Description |
| --- | --- | --- |
| `before` | `(helpin) => void` | Runs before the pageview event is sent |
| `typeName` | `string` | Override the event name, default `pageview` |
| `payload` | `EventPayload` | Extra fields merged into the pageview payload |

## `middlewareEnv(req, res, opts?)`

`middlewareEnv(...)` is the server-safe helper in this package. It does not create a client. It collects request metadata and manages the anonymous-id cookie in middleware.

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

Helper methods:

| Method | Description |
| --- | --- |
| `getAnonymousId({ name, domain? })` | Returns or creates the visitor id cookie |
| `getSourceIp()` | Extracts the client IP from request headers |
| `describeClient()` | Returns a `ClientProperties`-shaped object for the current request |

## Important Distinction

- `createClient(...)` from `@helpin-ai/nextjs` is client-only.
- For server-side tracking in middleware, route handlers, or server actions, use `helpinClient(...)` from `@helpin-ai/sdk-js`.
- The browser client auto-boots the widget through `@helpin-ai/sdk-js` when `widgetKey` and `host` are present.

## Accessing The Full SDK API

Like the React wrapper, `useHelpin()` focuses on the common tracking helpers. If you need the full client API, keep a reference to the object returned by `createClient(...)`.

That underlying client also supports:

- `group(...)`
- `reset(...)`
- `setUserId(...)`
- `getConfig()`
- `getLogger()`

## Widget Controls

`@helpin-ai/nextjs` does not currently provide a typed wrapper for widget control commands such as `show()`, `hide()`, or `toggle()`. Document those commands against the global/script Helpin API from `@helpin-ai/sdk-js`.

## Exports

| Export | Description |
| --- | --- |
| `createClient` | Client-side Helpin factory |
| `HelpinProvider` | React context provider |
| `HelpinContext` | Raw React context |
| `useHelpin` | Tracking hook |
| `usePageView` | Client-side pageview hook |
| `middlewareEnv` | Next middleware helper |

## Development

```bash
pnpm --filter @helpin-ai/nextjs build
pnpm --filter @helpin-ai/nextjs test
```
