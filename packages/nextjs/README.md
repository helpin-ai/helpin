# Official Helpin SDK for NextJS


## General

This package is a wrapper around `@helpin/sdk-js`, with added functionality related to NextJS.

## Installation

With NextJS there're several ways on how to add Helpin tracking

## Client Side Tracking

First, create or update your `_app.js` following this code
```jsx
import { createClient, HelpinProvider } from "@helpin/nextjs";

// initialize Helpin core
const helpinClient = createClient({
  tracking_host: "__HELPIN_HOST__",
  key: "__API_KET__",
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
import { useHelpin } from "@helpin/nextjs";

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
import { createClient, HelpinProvider } from "@helpin/nextjs";

// initialize Helpin core
const helpinClient = createClient({
  tracking_host: "__HELPIN_HOST__",
  key: "__API_KET__",
  // See Helpin SDK parameters section for more options
});

function MyApp({Component, pageProps}) {
  usePageView(helpinClient); // this hook will send pageview track event on router change

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
import { createClient } from "@helpin/nextjs";

// initialize Helpin core
const helpinClient = createClient({
  tracking_host: "__HELPIN_HOST__",
  key: "__API_KET__",
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

### Automated page view tracking

Helpin could track page views automatically via use of `_middleware.js` which has been introduced in NextJS 12

```javascript
export function middleware(req, ev) {
  const {page} = req
  if ( !page?.name ) {
    return;
  }
  helpin.track("page_view", {page: req.page})
}
```


## Example app

You can find example app [here](https://github.com/helpin/helpin-next-example).
