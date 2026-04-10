# @helpin-ai/sdk-js

Analytics and live chat for your website. Install as an npm module or add a script tag — both give you event tracking, user identification, and full control over the Helpin chat widget.

## Installation

```bash
npm install @helpin-ai/sdk-js
```

## Quick Start (Module)

```ts
import { helpinClient } from '@helpin-ai/sdk-js';

const client = helpinClient({
  widgetKey: 'your-widget-key',
  host: 'https://client.helpin.ai',
  namespace: 'helpin',
  autoBoot: false,
});

if (!client) {
  throw new Error('Helpin client failed to initialize');
}

await client.id({
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

client.track('button_click', { cta: 'pricing' });
client.lead({
  email: 'lead@example.com',
  first_name: 'New',
  last_name: 'Lead',
  company: {
    id: 'company_456',
    name: 'Acme Inc',
    created_at: '2024-01-15T00:00:00Z',
  },
});
client.pageview();
client.open();
```

By default, the widget boots automatically in browser environments when `widgetKey` and `host` are set. Pass `autoBoot: false` to keep the widget dormant until you explicitly call `boot()`, `show()`, `open()`, `openMessages()`, or `openNewMessage()`.

## Quick Start (Script Tag)

```html
<script>
  window.helpinQ = window.helpinQ || [];
  window.helpin = function () {
    window.helpinQ.push(arguments);
  };

  helpin('onLoad', function () {
    helpin('track', 'pageview');
    helpin('open');
  });
</script>

<script
  defer
  src="https://cdn.helpin.ai/lib.js"
  data-widget-key="your-widget-key"
  data-host="https://client.helpin.ai"
  data-namespace="helpin"
></script>
```

The snippet queues commands until the SDK loads, so you can call `helpin(...)` immediately. Once ready, the global API handles both analytics and widget control.

## Exports

| Export | Description |
| --- | --- |
| `helpinClient` | Factory — returns a `HelpinClient` or `null` |
| `HelpinClient` | The client class (analytics + widget) |
| `HelpinOptions` | Configuration type |
| `UserProps` | User identity payload |
| `EventPayload` | Event data payload |
| `ClientProperties` | Browser/request environment shape |
| `LogLevel` | Logger verbosity enum |

## Configuration

Configure via the `HelpinOptions` object passed to `helpinClient(...)`, or with `data-*` attributes on the script tag.

| Option | Description |
| --- | --- |
| `widgetKey` | **Required.** Your public widget key |
| `host` | Helpin host URL, with or without protocol |
| `autoBoot` | Boot the widget on initialization (default: `true`) |
| `namespace` | Global name for the script-tag build (default: `helpin`) |
| `autoPageview` | Track a pageview automatically on load |
| `useBeaconApi` | Prefer the Beacon API for event transport |
| `forceUseFetch` | Prefer `fetch` over `XMLHttpRequest` |
| `cookieDomain` / `cookieName` | Customize the anonymous visitor ID cookie |
| `crossDomainLinking` / `domains` | Share the visitor ID across specified domains |
| `propertyBlacklist` | Omit specific fields from outgoing payloads |
| `logLevel` | Internal logging verbosity |

**Script tag equivalents:** `data-widget-key`, `data-host`, `data-auto-boot`, `data-namespace`, `data-auto-pageview`, `data-log-level`.

## Client API

Every method below is available on the object returned by `helpinClient(...)`.

### Analytics

| Method | Signature | Description |
| --- | --- | --- |
| `init` | `(config: HelpinOptions) => void` | Re-initialize with new options |
| `id` | `(userData: UserProps, doNotSendEvent?: boolean) => Promise<void>` | Identify a user (optionally suppress the `user_identify` event) |
| `track` | `(eventName: string, payload?: EventPayload, directSend?: boolean) => void` | Track a custom event |
| `lead` | `(payload: EventPayload, directSend?: boolean) => void` | Track a validated lead event |
| `rawTrack` | `(payload: unknown) => void` | Send a raw payload as event type `raw` |
| `group` | `(company: { id: string; name: string; created_at: string; ... }, doNotSendEvent?: boolean) => Promise<void>` | Associate the user with a company or group |
| `pageview` | `() => void` | Send a pageview event |
| `set` | `(properties: Record<string, unknown>, opts?: { eventType?: string; persist?: boolean }) => void` | Attach global or event-scoped properties |
| `unset` | `(propertyName: string, opts?: { eventType?: string; persist?: boolean }) => void` | Remove a property added with `set(...)` |
| `setUserId` | `(userId: string) => void` | Update the stored user ID without a full identify call |
| `reset` | `(resetAnonId?: boolean) => Promise<void>` | Clear all persisted user, company, and global state |

### Widget

| Method | Signature | Description |
| --- | --- | --- |
| `boot` | `(settings?: { widgetKey?, key?, host?, user? }) => void` | Boot or re-boot the widget |
| `show` | `() => void` | Make the launcher/widget visible without opening the panel |
| `hide` | `() => void` | Hide the launcher and close the panel |
| `open` | `() => void` | Open the chat panel and ensure the widget is visible |
| `close` | `() => void` | Close the chat panel while keeping the launcher visible |
| `toggle` | `() => void` | Toggle the widget open or closed |
| `openMessages` | `() => void` | Open the widget to the messages list |
| `openNewMessage` | `(content?: string) => void` | Start a new conversation |
| `openConversation` | `(conversationId: string) => void` | Open a specific conversation |
| `openArticle` | `(articleId: string, options?: { collectionId?: string; spaceId?: string }) => void` | Display a help-center article |
| `shutdown` | `() => void` | End the widget session and remove it from the page |

### Event listeners

| Method | Signature | Description |
| --- | --- | --- |
| `onOpen` | `(callback) => void` | Fired when the visitor opens the chat from the widget UI |
| `onClose` | `(callback) => void` | Fired when the visitor closes the chat from the widget UI |
| `onUnreadCountChange` | `(callback) => void` | Unread count changed |
| `onUserEmailSupplied` | `(callback) => void` | Visitor submitted their email |
| `onConversationStarted` | `(callback) => void` | New conversation created |
| `onMessageReceived` | `(callback) => void` | Incoming message received |

### Getters

| Method | Signature | Description |
| --- | --- | --- |
| `getVisitorId` | `() => string` | Current anonymous visitor ID |
| `isWidgetReady` | `() => boolean` | Whether the widget has finished loading |
| `getConfig` | `() => HelpinOptions` | Merged runtime configuration |
| `getLogger` | `() => logger` | Internal logger instance |
| `getCookie` | `(name: string) => string \| null` | Read a browser cookie by name |

## Script Tag Command Reference

When using the script tag, every method above is available as `helpin('methodName', ...args)`. A few additional commands are specific to the global API:

| Command | Arguments | Description |
| --- | --- | --- |
| `onLoad` | `(callback)` | Run a callback once the SDK has finished loading |

## Module vs. Script Tag

Both builds provide the same analytics and widget capabilities. The module build returns a typed `HelpinClient` object; the script-tag build exposes the same methods through the `window.helpin(...)` command API. Choose whichever fits your stack.

## Development

```bash
pnpm --filter @helpin-ai/sdk-js build
pnpm --filter @helpin-ai/sdk-js test
```

Widget E2E tests:

```bash
pnpm --filter @helpin-ai/sdk-js test:e2e:widget
```
