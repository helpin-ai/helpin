# @helpin-ai/vue

Helpin for Vue 3. Add analytics, user identification, and the Helpin support widget to Vue or Nuxt applications.

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

## Development

```bash
pnpm --filter @helpin-ai/vue build
pnpm --filter @helpin-ai/vue test
```
