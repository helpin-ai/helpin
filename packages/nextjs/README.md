# Official Helpin SDK for NextJS


## General

This package is a wrapper around `@helpin-ai/sdk-js`, with added functionality related to NextJS.

## Installation

With NextJS there're several ways on how to add Helpin tracking

## Client Side Tracking

First, create or update your `_app.js` following this code
```jsx
import { createClient, HelpinProvider } from "@helpin-ai/nextjs";

// initialize Helpin core
const helpinClient = createClient({
  host: "__HELPIN_HOST__",
  widgetKey: "__API_KEY__",
  // See Helpin SDK parameters section for more options
});

// wrap our app with Helpin provider
function MyApp({Component, pageProps}) {
  return <HelpinProvider client={helpinClient}>
    <Component {...pageProps} />
  </HelpinProvider>
}

export default MyApp
```
See [parameters list](https://helpin.com/docs/sending-data/js-sdk/parameters-reference) for `createClient()` call.

After helpin client and provider are configured you will be able to use `useHelpin` hook in your components
```jsx
import { useHelpin } from "@helpin-ai/nextjs";

const Main = () => {
  const {id, trackPageView, track} = useHelpin(); // import methods from useHelpin hook

  useEffect(() => {
    id({id: '__USER_ID__', email: '__USER_EMAIL__'}); // identify current user for all events
    trackPageView() // send pageview event
  }, [])

  const onClick = (btnName) => {
    track('btn_click', {btn: btnName}); // send btn_click event with button name payload on click
  }

  return (
    <button onClick="() => onClick('test_btn')">Test button</button>
  )
}
```
Please note, that `useHelpin` uses `useEffect()` with related side effects.

\
To enable automatic pageview tracking, add `usePageView()` hook to your `_app.js`. This hook will send pageview each time
user loads a new page. This hook relies on [NextJS Router](https://nextjs.org/docs/api-reference/next/router)
```jsx
import { createClient, HelpinProvider, usePageView } from "@helpin-ai/nextjs";

// initialize Helpin core
const helpinClient = createClient({
  host: "__HELPIN_HOST__",
  widgetKey: "__API_KEY__",
  // See Helpin SDK parameters section for more options
});

function MyApp({Component, pageProps}) {
  usePageView(helpinClient); // this hook will send pageview track event on URL change

  // wrap our app with Helpin provider
  return <HelpinProvider client={helpinClient}>
    <Component {...pageProps} />
  </HelpinProvider>
}

export default MyApp
```
If you need to pre-configure helpin event - for example, identify a user, it's possible to do via `before` callback:
```javascript
usePageView(helpinClient, {before: (helpin) => helpin.id({id: '__USER_ID__', email: '__USER_EMAIL__'})})
```

## Server Side Tracking

Helpin can track events on server-side:
* **Pros:** this method is 100% reliable and ad-block resistant
* **Cons:** static rendering will not be possible; `next export` will not work; fewer data points will be collected - attributes such as screen-size, device

### Manual tracking

For manual tracking you need to initialize Helpin client
```javascript
import { createClient } from "@helpin-ai/nextjs";

// initialize Helpin core
const helpinClient = createClient({
  host: "__HELPIN_HOST__",
  widgetKey: "__API_KEY__",
  // See Helpin SDK parameters section for more options
});
```
after that, you will be able to user [Helpin client](https://helpin.com/docs/sending-data/js-sdk/methods-reference), for example, in `getServerSideProps`
```
export async function getServerSideProps() {
  helpin.track("page_view", {page: req.page})

  return { props: {} }
}
```

### Automated page view tracking with middleware

Helpin provides a `middlewareEnv` helper for Next.js middleware (introduced in Next.js 12). It handles anonymous ID management, source IP extraction, and client property collection:

```javascript
import { NextRequest, NextResponse } from 'next/server';
import { createClient, middlewareEnv } from "@helpin-ai/nextjs";

const helpinClient = createClient({
  host: "__HELPIN_HOST__",
  widgetKey: "__API_KEY__",
});

export function middleware(req) {
  const res = NextResponse.next();
  const env = middlewareEnv(req, res, { disableCookies: false });

  helpinClient.track("page_view", {
    ...env.describeClient(),
    source_ip: env.getSourceIp(),
    anonymous_id: env.getAnonymousId({ name: "__helpin_id", domain: ".yourdomain.com" }),
  });

  return res;
}
```

The `middlewareEnv(req, res, opts?)` function returns:
- `getAnonymousId({ name, domain? })` - Gets or creates an anonymous visitor ID via cookies
- `getSourceIp()` - Extracts the client IP from request headers
- `describeClient()` - Returns `ClientProperties` (URL, user agent, language, referrer, etc.)

## Exports

The `@helpin-ai/nextjs` package exports the following:

| Export | Description |
|--------|-------------|
| `HelpinProvider` | React context provider for the Helpin client |
| `HelpinContext` | React context object |
| `createClient` | Factory function to create a configured client |
| `useHelpin` | Hook returning tracking methods (`id`, `track`, `trackPageView`, `lead`, `rawTrack`, `set`, `unset`) |
| `usePageView` | Hook for automatic pageview tracking on URL changes |
| `middlewareEnv` | Helper for Next.js middleware (anonymous IDs, IP, client properties) |

### `usePageView` signature

```typescript
usePageView(
  helpin: HelpinClient | null,
  opts?: {
    before?: (helpin: HelpinClient) => void;
    typeName?: string;
    payload?: EventPayload;
  }
): HelpinClient | null
```

This hook monitors URL changes (including `pushState`/`replaceState` and `popstate`) and automatically sends pageview events. It does not depend on Next.js Router -- it uses the History API directly.

## Example app

You can find example app [here](https://github.com/helpin/helpin-next-example).
