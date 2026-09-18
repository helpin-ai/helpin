# @helpin-ai/sdk-js

Analytics and live chat for your website. Install as an npm module or add a script tag — both give you event tracking, user identification, and full control over the Helpin chat widget.

The npm module is a thin integration wrapper for analytics and widget commands. In browser environments it loads the live widget runtime from `https://cdn.helpin.ai/lib.js`, so widget UI and CSS updates can ship from the CDN without requiring customer application redeploys.

Self-hosted Community installations serve the same runtime from the API at `<PUBLIC_WIDGET_URL>/sdk/lib.js` (the `PUBLIC_SDK_URL` value in the bundle's `.env`). Use that URL in the script tag, or set `widgetRuntimeUrl` to it for npm installs. With `supportOnly: true` the SDK derives the runtime from `host` automatically.

## Installation

```bash
npm install @helpin-ai/sdk-js
```

## Quick Start (Module)

```ts
import { helpinClient } from '@helpin-ai/sdk-js';

const client = helpinClient({
  widgetKey: 'your-widget-key',
  host: 'https://client.helpin.ai', // Cloud; for Community use your PUBLIC_WIDGET_URL, e.g. http://localhost:8085
  namespace: 'helpin',
  autoBoot: false,
  // Optional: override the hosted widget runtime for staging or pinned deployments.
  // widgetRuntimeUrl: 'https://cdn.helpin.ai/lib.js',
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

Form, click, and scroll capture are off unless explicitly configured. Form values are limited to an allowlist and sensitive controls are never collected; click and scroll events always carry the rule key and version that requested them.

```ts
const client = helpinClient({
  widgetKey: 'your-widget-key',
  host: 'https://client.helpin.ai',
  formCapture: [{
    selector: '#demo-request',
    formId: 'demo-request',
    fields: ['work_email', 'company', 'role'],
    fieldMappings: {
      work_email: 'contact.email',
      company: 'company.name',
      role: 'contact.job_title',
    },
  }],
  interactionCaptureRules: [{
    ruleKey: 'versioned_interaction',
    version: 1,
    clickSelectors: ['[data-helpin-intent="pricing"]'],
    scrollMilestones: [75],
  }],
});

client?.articleView('security-overview');
```

Common field names such as `email`, `first_name`, `company`, `phone`, and `role` are mapped automatically. Use `fieldMappings` when a form uses non-standard names, or map a field to `ignore` to disable an automatic mapping. Every captured form still requires an explicit selector, stable form ID, and value allowlist; mappings never expand which values are collected. A form submission creates an untrusted lead identity. After login, call `id(...)` with an `identity_verification` proof (`{ version: 'v1', issued_at, expires_at, signature }`) signed by your server to upgrade it to a verified identity; see the [widget identity guide](../../docs/community/widget-identity.md).

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
| `LeadProps` | Validated lead identity and attributes |
| `ClientProperties` | Browser/request environment shape |
| `ShowArticleOptions` | Optional collection/space context for `openArticle(...)` |
| `LogLevel` | Logger verbosity enum |

## Configuration

Configure via the `HelpinOptions` object passed to `helpinClient(...)`, or with `data-*` attributes on the script tag.

| Option | Description |
| --- | --- |
| `widgetKey` | **Required.** Your public widget key |
| `host` | Helpin host URL, with or without protocol |
| `autoBoot` | Boot the widget on initialization (default: `true`) |
| `namespace` | Global name for the script-tag build (default: `helpin`) |
| `widgetRuntimeUrl` | Widget runtime URL for npm installs (default: `https://cdn.helpin.ai/lib.js`; Community: your `PUBLIC_SDK_URL`) |
| `supportOnly` | Support chat and identification only; no analytics collector or event queue. Loads the runtime from `host` + `/sdk/lib.js` unless `widgetRuntimeUrl` is set |
| `widgetRuntimeChannel` / `widgetRuntimeVersion` | Optional runtime selection metadata for hosted/pinned widget deployments |
| `autoPageview` | Track a pageview automatically on load |
| `useBeaconApi` | Prefer the Beacon API for event transport |
| `forceUseFetch` | Prefer `fetch` over `XMLHttpRequest` |
| `cookieDomain` / `cookieName` | Customize the anonymous visitor ID cookie |
| `crossDomainLinking` / `domains` | Share the visitor ID across specified domains |
| `propertyBlacklist` | Omit specific fields from outgoing payloads |
| `formCapture` | Explicit forms, safe value allowlists, and optional CRM field mappings |
| `logLevel` | Internal logging verbosity |

**Script tag equivalents:** `data-widget-key`, `data-host`, `data-auto-boot`, `data-namespace`, `data-auto-pageview`, `data-log-level`.

Use the public key for the in-app support widget you want to display. A public help center can intentionally advertise a different widget key. `host` is the Helpin application/API origin; it is not the URL where your site serves its JavaScript bundle.

## Client API

Every method below is available on the object returned by `helpinClient(...)`.

### Analytics

| Method | Signature | Description |
| --- | --- | --- |
| `init` | `(config: HelpinOptions) => void` | Re-initialize with new options |
| `id` | `(userData: UserProps, doNotSendEvent?: boolean) => Promise<void>` | Identify a user (optionally suppress the `user_identify` event) |
| `track` | `(eventName: string, payload?: EventPayload, directSend?: boolean) => void` | Track a custom event |
| `lead` | `(payload: LeadProps, directSend?: boolean) => void` | Track a validated lead event |
| `rawTrack` | `(payload: unknown) => void` | Send a raw payload as event type `raw` |
| `group` | `(company: { id: string; name: string; created_at: string; ... }, doNotSendEvent?: boolean) => Promise<void>` | Associate the user with a company or group |
| `articleView` | `(articleId: string, properties?: EventPayload) => void` | Track a help article view |
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
| `openArticle` | `(articleKey: string, options?: { collectionId?: string; spaceId?: string }) => void` | Open a help-center article inside the widget |
| `shutdown` | `() => void` | End the widget session and remove it from the page |

### Open a help-center article

Pass the final article segment from its Helpin URL. For example:

```ts
// https://acme.helpin.center/articles/getting-started-2906b16e
client.openArticle('getting-started-2906b16e');
```

For migrations from another help-center provider, map the old article ID to this Helpin article key. Use a normal link as a fallback when no reliable mapping exists:

```html
<a
  href="https://acme.helpin.center/articles/getting-started-2906b16e"
  target="_blank"
  rel="noreferrer"
>
  Learn more
</a>
```

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

Both builds provide the same analytics and widget capabilities. The script-tag build loads `lib.js` directly. The module build returns a typed `HelpinClient` object and injects that same hosted runtime in the browser, then delegates widget commands to `window.helpin(...)`. Choose whichever fits your stack.

## Troubleshooting

- **The widget does not appear:** verify `widgetKey`, `host`, and that the key belongs to the intended in-app widget.
- **Commands run before the widget loads:** calls are queued; with a script tag, register startup work through `helpin('onLoad', callback)` when ordering matters.
- **A custom launcher should control startup:** set `autoBoot: false`, then call `open()`, `openNewMessage()`, or `openArticle()`.
- **An article does not open:** pass the final Helpin article URL segment, not the legacy provider's article ID or the complete URL.

## Development

```bash
pnpm --filter @helpin-ai/sdk-js build
pnpm --filter @helpin-ai/sdk-js test
```

Widget E2E tests:

```bash
pnpm --filter @helpin-ai/sdk-js test:e2e:widget
```
