# @helpin-ai/react

React bindings for the Helpin SDK. Provides context, hooks, and automatic pageview tracking for any React SPA.

## Installation

```bash
npm install @helpin-ai/react @helpin-ai/sdk-js
```

## Quick Start

Wrap your app with `HelpinProvider` to make the client available throughout your component tree:

```tsx
import React from 'react';
import ReactDOM from 'react-dom/client';
import { createClient, HelpinProvider } from '@helpin-ai/react';

const helpinClient = createClient({
  widgetKey: 'your-widget-key',
  host: 'https://client.helpin.ai',
  autoBoot: false,
});

ReactDOM.createRoot(document.getElementById('root')!).render(
  <React.StrictMode>
    <HelpinProvider client={helpinClient}>
      <App />
    </HelpinProvider>
  </React.StrictMode>,
);
```

When running in the browser with `widgetKey` and `host` provided, the chat widget boots automatically through `@helpin-ai/sdk-js`. Set `autoBoot: false` to delay widget boot until you call a widget method like `show()` or `showNewMessage()`.

## Tracking with `useHelpin()`

Use the `useHelpin()` hook to identify users, track events, and control the widget:

```tsx
import { useEffect } from 'react';
import { useHelpin } from '@helpin-ai/react';

function App() {
  const { id, track, lead, trackPageView, set, show } = useHelpin();

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

`usePageView()` automatically tracks route changes by observing `pushState`, `replaceState`, and `popstate`. You can run setup logic before each pageview or attach extra payload data:

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
| `typeName` | `string` | Override the event name (default: `pageview`) |
| `payload` | `EventPayload` | Extra fields merged into the pageview payload |

## `HelpinProvider`

```tsx
<HelpinProvider client={helpinClient}>
  <App />
</HelpinProvider>
```

| Prop | Type | Description |
| --- | --- | --- |
| `client` | `HelpinClient \| null` | The client returned by `createClient(...)` |

## Full SDK API

`useHelpin()` exposes the most common tracking and widget methods. For the complete client API, hold onto the reference returned by `createClient(...)` — it also supports `boot(...)`, `group(...)`, `reset(...)`, `setUserId(...)`, `getConfig()`, and `getLogger()`.

## Widget Controls

Widget control is available directly from `useHelpin()` and the client returned by `createClient(...)`. If you want a custom launcher without rendering the widget immediately, initialize with `autoBoot: false` and call `show()` when the user clicks your CTA.

## Exports

| Export | Description |
| --- | --- |
| `createClient` | Create the underlying Helpin client |
| `HelpinProvider` | React context provider |
| `HelpinContext` | Raw React context |
| `useHelpin` | Tracking hook |
| `usePageView` | SPA pageview hook |

## Development

```bash
pnpm --filter @helpin-ai/react build
pnpm --filter @helpin-ai/react test
```
