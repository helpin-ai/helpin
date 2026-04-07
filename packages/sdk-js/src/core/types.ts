import { LogLevel } from '../utils/logger';

export interface EventPayload {
  [key: string]: any;
}

export interface UserProps extends EventPayload {
  id?: string;
  email?: string;
  company?: {
    id?: string;
    name?: string;
    created_at?: string;
    custom?: {
      [key: string]: any;
    };
  };
  [key: string]: any;
}

export interface Transport {
  send(payload: any): Promise<void>;
}

export type Policy = 'strict' | 'keep' | 'comply';

export interface CompanyProps {
  id: string;
  name: string;
  created_at: string;
  [key: string]: any;
}

/**
 * Environment where the event have happened.
 */
export type ClientProperties = {
  screen_resolution: string; //screen resolution
  user_agent: string; //user
  referer: string; //document referer
  url: string; //current url
  page_title: string; //page title
  //see UTM_TYPES for all supported utm tags
  doc_path: string; //document path
  doc_host: string; //document host
  doc_search: string; //document search string

  vp_size: string; //viewport size
  user_language: string; //user language
  doc_encoding: string;
};

type CamelCaseConfig = {
  widgetKey: string;
  host: string;
  cookieDomain?: string;
  cookieName?: string;
  logLevel?: LogLevel;
  useBeaconApi?: boolean;
  forceUseFetch?: boolean;
  autoPageview?: boolean;
  disableEventPersistence?: boolean;
  gaHook?: boolean;
  segmentHook?: boolean;
  randomizeUrl?: boolean;
  capture3rdPartyCookies?: string[] | false;
  idMethod?: 'cookie' | 'localStorage';
  privacyPolicy?: 'strict';
  ipPolicy?: Policy;
  cookiePolicy?: Policy;
  customHeaders?: Record<string, string> | (() => Record<string, string>);
  minSendTimeout?: number;
  maxSendTimeout?: number;
  maxSendAttempts?: number;
  propertiesStringMaxLength?: number | null;
  propertyBlacklist?: string[];
  exclude?: string;
  namespace?: string;
  crossDomainLinking?: boolean;
  domains?: string;
};

type SnakeCaseConfig = {
  widget_key: string;
  host: string;
  cookie_domain?: string;
  cookie_name?: string;
  log_level?: LogLevel;
  use_beacon_api?: boolean;
  force_use_fetch?: boolean;
  auto_pageview?: boolean;
  disable_event_persistence?: boolean;
  ga_hook?: boolean;
  segment_hook?: boolean;
  randomize_url?: boolean;
  capture_3rd_party_cookies?: string[] | false;
  id_method?: 'cookie' | 'localStorage';
  privacy_policy?: 'strict';
  ip_policy?: Policy;
  cookie_policy?: Policy;
  custom_headers?: Record<string, string> | (() => Record<string, string>);
  min_send_timeout?: number;
  max_send_timeout?: number;
  max_send_attempts?: number;
  properties_string_max_length?: number | null;
  property_blacklist?: string[];
  exclude?: string;
  namespace?: string;
  cross_domain_linking?: boolean;
  domains?: string;
};

export type Config = Partial<CamelCaseConfig & SnakeCaseConfig> & {
  widgetKey: string;
  host?: string;
  widget_key?: string;
} & { host: string };

/**
 * HelpinGlobal interface that supports both command-style and object-oriented API styles
 */
export interface HelpinGlobal {
  // Command-style API
  (command: 'track', eventName: string, payload?: any): void;
  (command: 'id', userData: UserProps, doNotSendEvent?: boolean): Promise<void>;
  (command: 'pageview'): void;
  (
    command: 'group',
    companyProps: CompanyProps,
    doNotSendEvent?: boolean,
  ): Promise<void>;
  (command: 'reset', resetAnonId?: boolean): Promise<void>;
  (
    command: 'set',
    properties: Record<string, any>,
    options?: { eventType?: string; persist?: boolean },
  ): void;
  (
    command: 'unset',
    propertyName: string,
    options?: { eventType?: string; persist?: boolean },
  ): void;
  (command: 'rawTrack', payload: any): void;
  (command: 'lead', payload: EventPayload, directSend?: boolean): void;
  (command: 'setUserId', userId: string): void;
  (command: 'onLoad', callback: () => void): void;
  (command: 'show', ...args: never[]): void;
  (command: 'hide', ...args: never[]): void;
  (command: 'toggle', ...args: never[]): void;
  (command: 'showMessages', ...args: never[]): void;
  (command: 'showNewMessage', content?: string): void;
  (command: 'showConversation', conversationId: string): void;
  (command: 'showArticle', articleId: string): void;
  (command: 'boot', settings: { widgetKey?: string; key?: string; host?: string; user?: Record<string, unknown> }): void;
  (command: 'shutdown', ...args: never[]): void;

  // Object-oriented API
  track(eventName: string, payload?: any): void;
  id(userData: UserProps, doNotSendEvent?: boolean): Promise<void>;
  pageview(): void;
  group(companyProps: CompanyProps, doNotSendEvent?: boolean): Promise<void>;
  reset(resetAnonId?: boolean): Promise<void>;
  set(
    properties: Record<string, any>,
    options?: { eventType?: string; persist?: boolean },
  ): void;
  unset(
    propertyName: string,
    options?: { eventType?: string; persist?: boolean },
  ): void;
  rawTrack(payload: any): void;
  lead(payload: EventPayload, directSend?: boolean): void;
  setUserId(userId: string): void;
  boot(settings: { widgetKey?: string; key?: string; host?: string; user?: Record<string, unknown> }): void;
  shutdown(): void;
  show(): void;
  hide(): void;
  toggle(): void;
  showMessages(): void;
  showNewMessage(content?: string): void;
  showConversation(conversationId: string): void;
  showArticle(articleId: string): void;
  getConfig(): Config | null;
}
