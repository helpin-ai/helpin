# @helpin-ai/nextjs

Helpin for Next.js. Drop-in analytics, chat widget control, and pageview tracking — with middleware support for server-side events.

This package is a thin Next.js wrapper. Browser clients load the chat widget UI from the hosted Helpin runtime at `https://cdn.helpin.ai/lib.js`, so future widget UI and CSS updates go live without requiring a Next.js app redeploy after customers upgrade to this wrapper architecture once.

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
        autoPageview: false, // usePageView owns route tracking.
        // Optional: use a staging or pinned runtime.
        // widgetRuntimeUrl: 'https://cdn.helpin.ai/lib.js',
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
  const { id, track, lead, set, open } = useHelpin();

  useEffect(() => {
    void id({
      id: 'user_123',
      email: 'jane@example.com',
      first_name: 'Jane',
      last_name: 'Doe',
      company: {
        id: 'company_123',
        name: 'Acme Inc',
        created_at: '2024-01-15T00:00:00Z',
      },
    });
    set({ app: 'web' });
  }, [id, set]);

  return (
    <>
      <button onClick={() => track('billing_cta_clicked', { source: 'hero' })}>
        Talk to Sales
      </button>
      <button onClick={open}>Chat with us</button>
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
| `show` | `() => void` | Make the widget visible without opening chat |
| `hide` | `() => void` | Hide the widget entirely |
| `open` | `() => void` | Open the chat panel |
| `close` | `() => void` | Close the chat panel while keeping the launcher visible |
| `toggle` | `() => void` | Toggle the widget open or closed |
| `openMessages` | `() => void` | Open the messages view |
| `openNewMessage` | `(content?) => void` | Start a new conversation |
| `openConversation` | `(conversationId) => void` | Open an existing conversation |
| `openArticle` | `(articleKey, options?) => void` | Open a Helpin article inside the widget |
| `shutdown` | `() => void` | End the session and unmount the widget |

For the complete client API (`boot`, `group`, `reset`, `setUserId`, `getConfig`, `getLogger`), use the object returned by `createClient(...)` directly.

### Open a help article

Call the hook from a client component and pass the final segment from the Helpin article URL:

```tsx
'use client';

import { useHelpin } from '@helpin-ai/nextjs';

export function LearnMore() {
  const { openArticle } = useHelpin();

  return (
    <button onClick={() => openArticle('how-to-add-first-comment-2906b16e')}>
      Learn more
    </button>
  );
}
```

For migrations from another help-center provider, map each legacy article ID to its Helpin article key. Use a normal external link only when no reliable mapping exists.

## `usePageView()`

With `autoPageview: false` on the client, this hook tracks client-side route changes. Optionally run setup logic or attach extra data before each pageview fires:

```tsx
'use client';

import { createClient, usePageView } from '@helpin-ai/nextjs';

const helpinClient = createClient({
  widgetKey: process.env.NEXT_PUBLIC_HELPIN_WIDGET_KEY!,
  host: process.env.NEXT_PUBLIC_HELPIN_HOST!,
  autoPageview: false, // usePageView owns route tracking.
  // widgetRuntimeUrl: 'https://cdn.helpin.ai/lib.js',
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

Mount one pageview hook per application. Disable the SDK’s `autoPageview` tracker
when using the hook to avoid duplicate route-change events. The `before` callback
is synchronous; it does not wait for an asynchronous `id()` call to finish.

| Option | Type | Description |
| --- | --- | --- |
| `before` | `(helpin) => void` | Runs before each pageview event |
| `typeName` | `string` | Custom event name (default: `pageview`) |
| `payload` | `EventPayload` | Extra fields merged into the payload |

## Server-Side Tracking

For server-side analytics where you have a `NextRequest` and a writable `NextResponse`, pair the core SDK with `middlewareEnv(req, res)`. The helper reads request metadata and writes the anonymous visitor cookie to that response. It is not a standalone Server Action helper:

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

Pass `{ disableCookies: true }` as the third argument to `middlewareEnv` to skip
anonymous-cookie creation. `getSourceIp()` reads forwarded headers; your proxy
configuration determines whether those values are trustworthy.

## Client vs. Server

| Context | What to use |
| --- | --- |
| Browser (analytics + widget) | `createClient(...)` from `@helpin-ai/nextjs` |
| Server (middleware or route handler with `NextRequest` / `NextResponse`) | `helpinClient(...)` from `@helpin-ai/sdk-js` + `middlewareEnv(...)` |

The chat widget boots automatically in the browser when `widgetKey` and `host` are set. Pass `autoBoot: false` to keep it dormant until you call `show()`, `open()`, or `openNewMessage()` — useful for custom launchers. Widget UI comes from the hosted runtime by default; analytics and server helpers remain in the npm package.

## Exports

| Export | Description |
| --- | --- |
| `createClient` | Browser-only client factory |
| `HelpinProvider` | React context provider |
| `HelpinContext` | Raw React context (for advanced use) |
| `useHelpin` | Analytics and widget hook |
| `usePageView` | Automatic pageview tracking hook |
| `middlewareEnv` | Next.js middleware helper |

## Configuration notes

- `widgetKey` must be the public key for the intended in-app widget. It can differ from the key embedded in a public help center.
- `host` is the Helpin application/API origin, for example `https://client.helpin.ai`.
- Browser initialization belongs in a Client Component; `createClient()` returns `null` during SSR.
- Set `autoBoot: false` when a custom launcher should decide when the widget loads.
- Widget UI is loaded from `https://cdn.helpin.ai/lib.js` by default. Override `widgetRuntimeUrl` only for a custom, staging, or pinned runtime.

See the [JavaScript SDK reference](../sdk-js/README.md#client-api) for configuration, widget events, and the complete client API.

## Development

```bash
pnpm --filter @helpin-ai/nextjs build
pnpm --filter @helpin-ai/nextjs test
```
