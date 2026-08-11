import type { App, Plugin } from 'vue';
import type { HelpinClient } from '@helpin-ai/sdk-js';
import { HelpinKey } from './injection';

export interface HelpinPluginOptions {
  client: HelpinClient | null;
}

export const HelpinPlugin: Plugin<[HelpinPluginOptions]> = {
  install(app: App, options: HelpinPluginOptions): void {
    const client = options?.client ?? null;
    app.provide(HelpinKey, client);
    app.config.globalProperties.$helpin = client;
  },
};

declare module 'vue' {
  interface ComponentCustomProperties {
    $helpin: HelpinClient | null;
  }
}
