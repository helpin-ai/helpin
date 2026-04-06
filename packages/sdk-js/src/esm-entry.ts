/**
 * Clean ESM entry point for npm consumers (React, Next.js, etc.).
 * No IIFE, no AMD, no window globals — just pure named exports.
 */
export { HelpinClient } from './core/client';
export { LogLevel } from './utils/logger';
export type { Config as HelpinOptions, UserProps, EventPayload, ClientProperties } from './core/types';

import { HelpinClient } from './core/client';
import { defaultConfig } from './core/config';
import type { Config } from './core/types';
import { convertKeysToCamelCase } from './utils/common';

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

  return new HelpinClient(mergedConfig);
}
