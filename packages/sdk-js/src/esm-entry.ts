/**
 * Clean ESM entry point for npm consumers (React, Next.js, etc.).
 * No IIFE, no AMD, no window globals — just pure named exports.
 * Loads the hosted widget runtime for npm consumers so chat UI updates can ship
 * through the CDN without requiring customer application redeploys.
 */
export { HelpinClient } from './core/client';
export { LogLevel } from './utils/logger';
export type { Config as HelpinOptions, UserProps, LeadProps, CompanyPayload, EventPayload, ClientProperties } from './core/types';

import { HelpinClient } from './core/client';
import { HostedWidgetController } from './core/hosted-widget';
import type { Config } from './core/types';
import { createHelpinClient } from './core/create-client';
import { isWindowAvailable } from './utils/common';

export function helpinClient(config: Partial<Config>): HelpinClient | null {
  if (!isWindowAvailable()) {
    return createHelpinClient(config);
  }

  return createHelpinClient(config, new HostedWidgetController(config));
}
