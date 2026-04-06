/**
 * Clean ESM entry point for npm consumers (React, Next.js, etc.).
 * No IIFE, no AMD, no window globals — just pure named exports.
 * Includes WidgetManager for automatic chat widget rendering.
 */
export { HelpinClient } from './core/client';
export { LogLevel } from './utils/logger';
export type { Config as HelpinOptions, UserProps, EventPayload, ClientProperties } from './core/types';

import { HelpinClient } from './core/client';
import { WidgetManager } from './core/widget';
import { defaultConfig } from './core/config';
import type { Config } from './core/types';
import { convertKeysToCamelCase, isWindowAvailable } from './utils/common';

const widgetManager = new WidgetManager();

export function helpinClient(config: Partial<Config>): HelpinClient {
  const cleanConfig = JSON.parse(JSON.stringify(config));
  const camelCaseConfig = convertKeysToCamelCase(cleanConfig);
  const mergedConfig: Config = {
    ...defaultConfig,
    ...camelCaseConfig,
  } as Config;

  if (!mergedConfig.host) {
    throw new Error('Host is required!');
  }
  if (!mergedConfig.widgetKey) {
    throw new Error('Widget key is required!');
  }

  const client = new HelpinClient(mergedConfig);

  // Auto-boot the chat widget in browser environments
  if (isWindowAvailable() && mergedConfig.widgetKey) {
    widgetManager.boot({
      widgetKey: mergedConfig.widgetKey,
      host: mergedConfig.host,
    });
  }

  return client;
}
