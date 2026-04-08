# @helpin-ai/sdk-js

The core Helpin SDK for analytics and the embeddable chat widget. Works in any browser environment — use it as an npm module or drop in a script tag.

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
});

if (!client) {
  throw new Error('Helpin client failed to initialize');
}

await client.id({
  id: 'user_123',
  email: 'jane@example.com',
  name: 'Jane Doe',
});

client.track('button_click', { cta: 'pricing' });
client.lead({ email: 'lead@example.com', name: 'New Lead' });
client.pageview();
```

When running in the browser with `widgetKey` and `host` provided, the module build automatically boots the chat widget alongside the analytics client.

## Quick Start (Script Tag)

```html
<script>
  window.helpinQ = window.helpinQ || [];
  window.helpin = function () {
    window.helpinQ.push(arguments);
  };

  helpin('onLoad', function () {
    helpin('track', 'pageview');
    helpin('show');
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

The script tag exposes a command-style API on `window.helpin(...)` that covers both analytics and widget control.

## Exports

| Export | Description |
| --- | --- |
| `helpinClient` | Factory that returns `HelpinClient \| null` |
| `HelpinClient` | Core analytics client class |
| `HelpinOptions` | SDK configuration type |
| `UserProps` | User identity payload |
| `EventPayload` | Generic event payload |
| `ClientProperties` | Request/browser environment shape |
| `LogLevel` | Logger level enum |

## Configuration

Pass an `HelpinOptions` object to `helpinClient(...)` or use matching `data-*` attributes on the script tag.

| Option | Description |
| --- | --- |
| `widgetKey` | **Required.** Your public widget key |
| `host` | Helpin host URL (with or without protocol) |
| `namespace` | Global namespace for the script-tag build (default: `helpin`) |
| `autoPageview` | Automatically track pageviews on load |
| `useBeaconApi` | Use the Beacon API for transport when available |
| `forceUseFetch` | Prefer `fetch` over `XMLHttpRequest` |
| `cookieDomain` / `cookieName` | Configure the anonymous visitor ID cookie |
| `crossDomainLinking` / `domains` | Carry the visitor ID across specified domains |
| `propertyBlacklist` | Strip specific fields from outgoing event payloads |
| `logLevel` | Control internal logging verbosity |

**Script tag equivalents:** `data-widget-key`, `data-host`, `data-namespace`, `data-auto-pageview`, `data-log-level`.

## Client API

All methods are available on the object returned by `helpinClient(...)`.

| Method | Signature | Description |
| --- | --- | --- |
| `init` | `(config: HelpinOptions) => void` | Re-initialize the client with new options |
| `id` | `(userData: UserProps, doNotSendEvent?: boolean) => Promise<void>` | Identify a user (optionally suppressing the `user_identify` event) |
| `track` | `(eventName: string, payload?: EventPayload, directSend?: boolean) => void` | Track a custom event |
| `lead` | `(payload: EventPayload, directSend?: boolean) => void` | Track a validated lead event |
| `rawTrack` | `(payload: unknown) => void` | Send a raw event payload (event type `raw`) |
| `group` | `(company: { id: string; name: string; created_at: string; ... }, doNotSendEvent?: boolean) => Promise<void>` | Associate the user with a company or group |
| `pageview` | `() => void` | Send a pageview event |
| `set` | `(properties: Record<string, unknown>, opts?: { eventType?: string; persist?: boolean }) => void` | Attach global or event-scoped properties |
| `unset` | `(propertyName: string, opts?: { eventType?: string; persist?: boolean }) => void` | Remove a property previously added with `set(...)` |
| `setUserId` | `(userId: string) => void` | Update the stored user ID without a full identify call |
| `reset` | `(resetAnonId?: boolean) => Promise<void>` | Clear all persisted user, company, and global state |
| `getConfig` | `() => HelpinOptions` | Return the merged runtime configuration |
| `getLogger` | `() => logger` | Return the internal logger instance |
| `getCookie` | `(name: string) => string \| null` | Read a browser cookie by name |

## Global Analytics Commands

When using the script tag, call these via `helpin('command', ...)`.

| Command | Arguments | Description |
| --- | --- | --- |
| `init` | `(config)` | Initialize analytics manually (skips script-tag auto-init) |
| `track` | `(eventName, payload?)` | Track a custom event |
| `id` | `(userData, doNotSendEvent?)` | Identify a user |
| `lead` | `(payload, directSend?)` | Track a lead |
| `group` | `(companyProps, doNotSendEvent?)` | Associate a company or group |
| `pageview` | `()` | Track a pageview |
| `set` | `(properties, opts?)` | Set global or event-scoped properties |
| `unset` | `(propertyName, opts?)` | Remove a previously set property |
| `rawTrack` | `(payload)` | Send a raw event payload |
| `setUserId` | `(userId)` | Update the stored user ID |
| `reset` | `(resetAnonId?)` | Clear all tracked identity and persisted state |
| `getConfig` | `()` | Return the runtime configuration |
| `onLoad` | `(callback)` | Run a callback once the SDK is ready |

## Global Widget Commands

Widget control is available through the script-tag API.

| Command | Arguments | Description |
| --- | --- | --- |
| `boot` | `({ widgetKey, key?, host?, user? })` | Manually boot the widget |
| `shutdown` | `()` | End the widget session and remove it from the page |
| `show` | `()` | Open the widget |
| `hide` | `()` | Close the widget (keeps it mounted) |
| `toggle` | `()` | Toggle the widget open or closed |
| `showMessages` | `()` | Open the widget to the messages list |
| `showNewMessage` | `(content?)` | Start a new conversation, optionally with an initial message |
| `showConversation` | `(conversationId)` | Open a specific conversation |
| `showArticle` | `(articleId, options?)` | Display a help-center article |
| `onShow` | `(callback)` | Listen for widget open events |
| `onHide` | `(callback)` | Listen for widget close events |
| `onUnreadCountChange` | `(callback)` | Listen for unread count changes |
| `onUserEmailSupplied` | `(callback)` | Listen for visitor email submissions |
| `onConversationStarted` | `(callback)` | Listen for new conversations |
| `onMessageReceived` | `(callback)` | Listen for incoming messages |
| `getVisitorId` | `()` | Return the current anonymous visitor ID |
| `isWidgetReady` | `()` | Check whether the widget has finished loading |

## Module vs. Script Tag

The SDK ships two builds that differ in how they expose widget controls:

- **Module build** (`helpinClient(...)`) — returns an analytics client. The widget boots automatically in the browser, but widget control methods (`show`, `hide`, `toggle`, etc.) are not available on the returned `HelpinClient` instance.
- **Script-tag build** (`window.helpin(...)`) — exposes both analytics commands and widget control commands through a single global API.

If you need widget control alongside the module build, use the global `window.helpin(...)` commands for widget operations.

## Development

```bash
pnpm --filter @helpin-ai/sdk-js build
pnpm --filter @helpin-ai/sdk-js test
```

Widget E2E tests:

```bash
pnpm --filter @helpin-ai/sdk-js test:e2e:widget
```
