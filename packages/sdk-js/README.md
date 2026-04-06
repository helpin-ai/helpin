# Helpin SDK

Helpin SDK is a powerful and flexible JavaScript/TypeScript library for tracking user behavior and events in web applications. It supports both client-side and server-side usage, with a focus on privacy, configurability, and robustness.

## Features

- Cross-platform compatibility (browser and server-side)
- Flexible event tracking with custom payloads
- Automatic tracking of page views, form submissions, and user interactions
- Privacy-focused with configurable data sanitization
- Robust error handling and retry mechanisms
- Extensible architecture for custom tracking features
- Performance optimizations including event batching and debouncing

## Installation

### NPM or Yarn

You can install the Helpin SDK using npm:

```bash
npm install @helpin-ai/sdk-js
```

Or using yarn:

```bash
yarn add @helpin-ai/sdk-js
```

### UMD (Universal Module Definition)

For quick integration without a module bundler, you can include the SDK directly in your HTML using a script tag:

```html
<script src="https://cdn.helpin.ai/sdk/v1/lib.js"
        data-key="your-api-key"
        data-tracking-host="https://events.yourdomain.com"
        data-log-level="debug"
        data-autocapture="true"
        data-form-tracking="true"
        data-auto-pageview="true"></script>
```

Replace `https://cdn.helpin.ai/sdk/v1/lib.js` with the actual URL where the Helpin SDK is hosted.

## Basic Usage

### Using as a module

```javascript
import { helpinClient } from '@helpin-ai/sdk-js';

const client = helpinClient({
  apiKey: 'your-api-key',
  trackingHost: 'https://events.yourdomain.com',
  // Add other configuration options as needed
});

// Track an event
client.track('button_click', {
  buttonId: 'submit-form',
  pageUrl: window.location.href
});

// Identify a user
client.id({
  id: 'user123',
  email: 'user@example.com',
  name: 'John Doe'
});

// Track a page view
client.pageview();
```

### Using via UMD

When you include the SDK via a script tag, it automatically initializes with the configuration provided in the data attributes. You can then use the global `helpin` function to interact with the SDK:

```html
<script>
  // Track an event
  helpin('track', 'button_click', {
    buttonId: 'submit-form',
    pageUrl: window.location.href
  });

  // Identify a user
  helpin('id', {
    id: 'user123',
    email: 'user@example.com',
    name: 'John Doe'
  });

  // Track a page view (if not set to automatic in the script tag)
  helpin('pageview');
</script>
```

## Advanced Configuration

The SDK supports various configuration options to customize its behavior. When using as a module:

```javascript
const client = helpinClient({
  apiKey: 'your-api-key',
  trackingHost: 'https://events.yourdomain.com',
  cookieDomain: '.yourdomain.com',
  logLevel: 'DEBUG',
  useBeaconApi: true,
  autocapture: true,
  formTracking: 'all',
  autoPageview: true,
  // ... other options
});
```

When using via UMD, you can set these options using data attributes on the script tag:

```html
<script src="https://cdn.helpin.ai/sdk/v1/lib.js"
        data-key="your-api-key"
        data-tracking-host="https://events.yourdomain.com"
        data-log-level="debug"
        data-autocapture="true"
        data-form-tracking="all"
        data-auto-pageview="true"
        data-use-beacon-api="true"
        data-cookie-domain=".yourdomain.com"></script>
```

Refer to the `Config` interface in `src/core/config.ts` for a full list of configuration options.

## Server-Side Usage

The SDK can also be used in server-side environments:

```javascript
const { helpinClient } = require('@helpin-ai/sdk-js');

const client = helpinClient({
  apiKey: 'your-api-key',
  trackingHost: 'https://events.yourdomain.com'
});

client.track('server_event', {
  userId: 'user123',
  action: 'item_purchased'
});
```

## Development

To set up the project for development:

1. Clone the repository
2. Install dependencies: `npm install`
3. Run tests: `npm test`
4. Build the project: `npm run build`

## Testing

### Unit Tests

To run unit tests:

```bash
pnpm --filter @helpin-ai/sdk-js test
```

Unit tests cover:
- `core/client.ts` - HelpinClient initialization and methods
- `core/event-tracking.ts` - Event tracking functionality
- `core/widget.ts` - WidgetManager for chat widget
- `utils/queue.ts` - Retry queue implementation
- `utils/helpers.ts` - Utility functions
- `persistence/local-storage.ts` - Local storage persistence

### Widget E2E Tests

To run widget E2E tests (requires mock server):

```bash
pnpm exec playwright test --config=playwright.widget.config.ts
```

Widget E2E tests cover:
- Widget boot & initialization
- Show/hide functionality
- Callbacks (onShow, onHide, onUnreadCountChange, onUserEmailSupplied)
- Launcher interactions
- UI elements (header, pre-chat form, compose bar)
- Mobile responsiveness
- Widget configuration

### All E2E Tests

To run end-to-end tests with a specific browser (e.g., Chrome):

```bash
pnpm --filter @helpin-ai/sdk-js test:e2e --project=chromium
```

You can replace `chromium` with other browsers like `firefox` or `webkit` to test on different browsers.

To run E2E tests with a UI for debugging:

```bash
pnpm --filter @helpin-ai/sdk-js test:e2e:ui
```

To view the Playwright report after running tests:

```bash
pnpm exec playwright show-report
```

## Contributing

Contributions are welcome! Please read our contributing guidelines and code of conduct before submitting pull requests.
