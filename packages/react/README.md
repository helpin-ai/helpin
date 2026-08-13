# @helpin-ai/react

Helpin for React. Analytics, user identification, pageview tracking, and chat widget control — all through a single hook.

This package is a thin React wrapper. The chat widget UI is loaded from the hosted Helpin runtime at `https://cdn.helpin.ai/lib.js`, so future widget UI and CSS updates go live without requiring a React app redeploy after customers upgrade to this wrapper architecture once.

## Installation

```bash
npm install @helpin-ai/react @helpin-ai/sdk-js
```

## Quick Start

Wrap your app with `HelpinProvider` to make the client available throughout the component tree:

```tsx
import React from 'react';
import ReactDOM from 'react-dom/client';
import { createClient, HelpinProvider } from '@helpin-ai/react';

const helpinClient = createClient({
  widgetKey: 'your-widget-key',
  host: 'https://client.helpin.ai',
  autoBoot: false,
  // Optional: use a staging or pinned runtime.
  // widgetRuntimeUrl: 'https://cdn.helpin.ai/lib.js',
});

ReactDOM.createRoot(document.getElementById('root')!).render(
  <React.StrictMode>
    <HelpinProvider client={helpinClient}>
      <App />
    </HelpinProvider>
  </React.StrictMode>,
);
```

The chat widget boots automatically in the browser when `widgetKey` and `host` are set. Pass `autoBoot: false` to keep it dormant until you call `show()`, `open()`, or `openNewMessage()` — useful for custom launchers.

## `useHelpin()`

The hook provides analytics, user identification, and widget control from any component:

```tsx
import { useEffect } from 'react';
import { useHelpin } from '@helpin-ai/react';

function App() {
  const { id, track, lead, trackPageView, set, open } = useHelpin();

  useEffect(() => {
    void id({
      id: 'user_123',
      email: 'jane@example.com',
      name: 'Jane Doe',
    });
    trackPageView();
    set({ workspace: 'marketing-site' });
  }, [id, set, trackPageView]);

  return (
    <>
      <button onClick={() => track('cta_clicked', { cta: 'pricing' })}>
        Open Pricing
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

Pass the final segment from the Helpin article URL:

```tsx
import { useHelpin } from '@helpin-ai/react';

function LearnMore() {
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

Tracks route changes automatically by observing `pushState`, `replaceState`, and `popstate`. Optionally run setup logic or attach extra data before each pageview fires:

```tsx
import { usePageView } from '@helpin-ai/react';

function AppShell() {
  usePageView({
    before: (helpin) => {
      void helpin.id({ id: 'user_123', email: 'jane@example.com' });
    },
    payload: {
      app_section: 'dashboard',
    },
  });

  return <AppRoutes />;
}
```

| Option | Type | Description |
| --- | --- | --- |
| `before` | `(helpin) => void` | Runs before each pageview event |
| `typeName` | `string` | Custom event name (default: `pageview`) |
| `payload` | `EventPayload` | Extra fields merged into the payload |

## `HelpinProvider`

```tsx
<HelpinProvider client={helpinClient}>
  <App />
</HelpinProvider>
```

| Prop | Type | Description |
| --- | --- | --- |
| `client` | `HelpinClient \| null` | The client returned by `createClient(...)` |

## Exports

| Export | Description |
| --- | --- |
| `createClient` | Client factory |
| `HelpinProvider` | React context provider |
| `HelpinContext` | Raw React context (for advanced use) |
| `useHelpin` | Analytics and widget hook |
| `usePageView` | Automatic pageview tracking hook |

## Configuration notes

- `widgetKey` must be the public key for the intended in-app widget. It can differ from the key embedded in a public help center.
- `host` is the Helpin application/API origin, for example `https://client.helpin.ai`.
- Set `autoBoot: false` when a custom launcher should decide when the widget loads.
- Widget UI is loaded from `https://cdn.helpin.ai/lib.js` by default. Override `widgetRuntimeUrl` only for a custom, staging, or pinned runtime.

See the [JavaScript SDK reference](../sdk-js/README.md#client-api) for configuration, widget events, and the complete client API.

## Development

```bash
pnpm --filter @helpin-ai/react build
pnpm --filter @helpin-ai/react test
```
