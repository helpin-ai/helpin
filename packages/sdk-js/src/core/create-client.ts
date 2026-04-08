import { HelpinClient } from './client';
import { defaultConfig } from './config';
import type { Config } from './types';
import { convertKeysToCamelCase } from '../utils/common';

type SafeInitMetadata = {
  hasWidgetKey: boolean;
  hasHost: boolean;
  namespace: string;
};

function getSafeInitMetadata(config: Partial<Config>): SafeInitMetadata {
  return {
    hasWidgetKey: Boolean(config.widgetKey || config.widget_key),
    hasHost: Boolean(config.host),
    namespace: config.namespace || 'default',
  };
}

export function logClientInitializationError(
  message: string,
  config: Partial<Config>,
  error?: unknown,
): void {
  const metadata = getSafeInitMetadata(config);
  if (typeof error === 'undefined') {
    console.error(`[Helpin] ${message}`, metadata);
    return;
  }
  console.error(`[Helpin] ${message}`, metadata, error);
}

export function normalizeClientConfig(config: Partial<Config>): Config {
  const cleanConfig = JSON.parse(JSON.stringify(config ?? {}));
  const camelCaseConfig = convertKeysToCamelCase(cleanConfig);
  return {
    ...defaultConfig,
    ...camelCaseConfig,
  } as Config;
}

export function createHelpinClient(
  config: Partial<Config>,
): HelpinClient | null {
  const mergedConfig = normalizeClientConfig(config);

  if (!mergedConfig.widgetKey) {
    logClientInitializationError(
      'Widget initialization skipped: widgetKey is required.',
      config,
    );
    return null;
  }

  try {
    return new HelpinClient(mergedConfig);
  } catch (error) {
    logClientInitializationError(
      'Widget initialization failed.',
      config,
      error,
    );
    return null;
  }
}
