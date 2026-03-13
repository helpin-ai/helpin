// src/core/config.ts

import { LogLevel } from '../utils/logger';
import { Config } from './types';

export type Policy = 'strict' | 'keep' | 'comply';

export const defaultConfig: Partial<Config> = {
  logLevel: LogLevel.ERROR,
  useBeaconApi: false,
  forceUseFetch: false,
  trackingHost: 'https://client.prod.helpin.ai',
  autoPageview: true,
  disableEventPersistence: false,
  gaHook: false,
  segmentHook: false,
  randomizeUrl: false,
  capture3rdPartyCookies: [
    '_ga',
    '_fbp',
    '_ym_uid',
    'ajs_user_id',
    'ajs_anonymous_id',
  ],
  idMethod: 'cookie',
  ipPolicy: 'keep',
  cookiePolicy: 'keep',
  minSendTimeout: 0,
  maxSendTimeout: 2000,
  maxSendAttempts: 4,
  propertiesStringMaxLength: null,
  propertyBlacklist: [],
  crossDomainLinking: true,
};
