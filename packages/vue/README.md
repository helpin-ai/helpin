# @helpin-ai/vue

Helpin for Vue 3. Add analytics, user identification, and the Helpin support widget to Vue or Nuxt applications. Requires Vue 3.3 or newer; Vue Router is optional.

## Installation

```bash
npm install @helpin-ai/vue @helpin-ai/sdk-js
```

## Vue 3 quick start

```ts
// main.ts
import { createApp } from 'vue';
import { createClient, HelpinPlugin } from '@helpin-ai/vue';
import App from './App.vue';

const client = createClient({
  widgetKey: import.meta.env.VITE_HELPIN_WIDGET_KEY,
  host: import.meta.env.VITE_HELPIN_HOST,
});

createApp(App)
  .use(HelpinPlugin, { client })
  .mount('#app');
```

Use Helpin in any descendant component:

```vue
<script setup lang="ts">
import { onMounted } from 'vue';
import { useHelpin } from '@helpin-ai/vue';

const helpin = useHelpin();

onMounted(() => {
  void helpin.id({ id: 'user_123', email: 'jane@example.com' });
});
</script>

<template>
  <button @click="helpin.open()">Chat with us</button>
</template>
```

## Identify signed-in users

Call `id(...)` after authentication so conversations and analytics are associated with the right customer:

```ts
await helpin.id({
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
```

## Widget controls

Every method below is available from `useHelpin()`:

| Method | Purpose |
| --- | --- |
| `show()` / `hide()` | Show or hide the launcher and widget |
| `open()` / `close()` / `toggle()` | Control the panel |
| `openMessages()` | Open the conversation list |
| `openNewMessage(content?)` | Start a conversation, optionally with draft content |
| `openConversation(conversationId)` | Open an existing conversation |
| `openArticle(articleKey, options?)` | Open a Helpin article inside the widget |
| `shutdown()` | End the session and remove the widget |

### Open a help article

Use the final segment from the Helpin article URL:

```vue
<script setup lang="ts">
import { useHelpin } from '@helpin-ai/vue';

const helpin = useHelpin();
const openGuide = () =>
  helpin.openArticle('how-to-add-first-comment-2906b16e');
</script>

<template>
  <button @click="openGuide">Learn more</button>
</template>
```

For a migration, map each legacy article ID to its Helpin article key. Use a normal external link only when no reliable mapping exists.

## Vue Router pageviews

Pass your Vue Router instance once in a component near the root of the app:

```vue
<script setup lang="ts">
import { useRouter } from 'vue-router';
import { usePageView } from '@helpin-ai/vue';

usePageView({
  router: useRouter(),
  payload: { framework: 'vue' },
});
</script>
```

`usePageView()` tracks the initial browser URL and subsequent successful navigations. Vue Router is an optional peer dependency; it is only required when the `router` option is used.

## Nuxt 3

Create a client-only plugin:

```ts
// plugins/helpin.client.ts
import { createClient, HelpinPlugin } from '@helpin-ai/vue';

export default defineNuxtPlugin((nuxtApp) => {
  const config = useRuntimeConfig();
  const client = createClient({
    widgetKey: config.public.helpinWidgetKey,
    host: config.public.helpinHost,
  });

  nuxtApp.vueApp.use(HelpinPlugin, { client });
  return { provide: { helpin: client } };
});
```

The `.client.ts` suffix ensures initialization only runs in the browser. `createClient()` also returns `null` during SSR.

## Configuration notes

- `widgetKey` must be the public key for the intended in-app widget. It can differ from the key embedded in a public help center.
- `host` is the Helpin application/API origin, for example `https://client.helpin.ai`.
- The widget boots automatically when `widgetKey` and `host` are present. Set `autoBoot: false` for a custom launcher.
- Widget UI is loaded from `https://cdn.helpin.ai/lib.js` by default. Override `widgetRuntimeUrl` only for a custom, staging, or pinned runtime.

The client returned by `createClient()` also exposes the complete JavaScript SDK API, including events and lower-level lifecycle methods. See the [JavaScript SDK reference](../sdk-js/README.md#client-api).

## Development

```bash
pnpm --filter @helpin-ai/vue build
pnpm --filter @helpin-ai/vue test
```
