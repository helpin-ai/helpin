# @helpin-ai/react

React bindings for `@helpin-ai/sdk-js`.

This package gives you:

- `createClient(...)` to create the underlying Helpin client
- `HelpinProvider` to place that client in React context
- `useHelpin()` for the typed tracking helpers used inside components
- `usePageView()` for automatic SPA pageview tracking

## Install

```bash
npm install @helpin-ai/react @helpin-ai/sdk-js
```

## Quick Start

```tsx
import React from 'react';
import ReactDOM from 'react-dom/client';
import { createClient, HelpinProvider } from '@helpin-ai/react';

const helpinClient = createClient({
  widgetKey: 'your-widget-key',
  host: 'https://client.helpin.ai',
});

ReactDOM.createRoot(document.getElementById('root')!).render(
  <React.StrictMode>
    <HelpinProvider client={helpinClient}>
      <App />
    </HelpinProvider>
  </React.StrictMode>,
);
```

In browser builds, the underlying `@helpin-ai/sdk-js` client also auto-boots the widget when `widgetKey` and `host` are present.

## `useHelpin()`

```tsx
import { useEffect } from 'react';
import { useHelpin } from '@helpin-ai/react';

function App() {
  const { id, track, lead, trackPageView, set, unset } = useHelpin();

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
    <button onClick={() => track('cta_clicked', { cta: 'pricing' })}>
      Open Pricing
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

## `usePageView()`

`usePageView()` tracks URL changes by observing `pushState`, `replaceState`, and `popstate`.

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

Options:

| Option | Type | Description |
| --- | --- | --- |
| `before` | `(helpin) => void` | Runs before the pageview event is sent |
| `typeName` | `string` | Override the event name, default `pageview` |
| `payload` | `EventPayload` | Extra fields merged into the pageview payload |

## `HelpinProvider`

```tsx
<HelpinProvider client={helpinClient}>
  <App />
</HelpinProvider>
```

Props:

| Prop | Type | Description |
| --- | --- | --- |
| `client` | `HelpinClient | null` | Client returned by `createClient(...)` |

## Accessing The Full SDK API

`useHelpin()` intentionally documents the common tracking helpers above. If you need the broader `@helpin-ai/sdk-js` client API, keep a reference to the object returned by `createClient(...)`.

That underlying client also supports methods such as:

- `group(...)`
- `reset(...)`
- `setUserId(...)`
- `getConfig()`
- `getLogger()`

## Widget Controls

`@helpin-ai/react` does not currently add a typed React hook for widget control commands such as `show()`, `hide()`, or `toggle()`.

What it does do:

- creating the client in the browser auto-boots the widget through `@helpin-ai/sdk-js`
- gives you React-friendly tracking hooks and pageview handling

If you need explicit widget control methods today, document them against the global/script Helpin API from `@helpin-ai/sdk-js`, not this React wrapper.

## Exports

| Export | Description |
| --- | --- |
| `createClient` | Creates the underlying Helpin client |
| `HelpinProvider` | React context provider |
| `HelpinContext` | Raw React context |
| `useHelpin` | Tracking hook |
| `usePageView` | SPA pageview hook |

## Development

```bash
pnpm --filter @helpin-ai/react build
pnpm --filter @helpin-ai/react test
```
