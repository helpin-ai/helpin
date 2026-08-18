import type { InjectionKey } from 'vue';
import type { HelpinClient } from '@helpin-ai/sdk-js';

export const HelpinKey: InjectionKey<HelpinClient | null> = Symbol('Helpin');
