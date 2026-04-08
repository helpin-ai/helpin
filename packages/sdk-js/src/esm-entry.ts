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
import type { Config } from './core/types';
import { createHelpinClient } from './core/create-client';
import { isWindowAvailable } from './utils/common';

const widgetManager = new WidgetManager();

export function helpinClient(config: Partial<Config>): HelpinClient | null {
  const client = createHelpinClient(config);
  if (!client) {
    return null;
  }
  const mergedConfig = client.getConfig();

  // Auto-boot the chat widget in browser environments
  if (isWindowAvailable() && mergedConfig?.widgetKey && mergedConfig.host) {
    widgetManager.boot({
      widgetKey: mergedConfig.widgetKey,
      host: mergedConfig.host,
    });
  }

  return client;
}
