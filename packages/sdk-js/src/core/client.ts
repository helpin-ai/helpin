import { defaultConfig } from './config';
import { Config } from './types';
import { CompanyProps, EventPayload, Transport, UserProps } from './types';
import { getLogger, Logger } from '../utils/logger';
import { CookieManager } from '../utils/cookie';
import { PageviewTracking } from '../tracking/pageviews';
import { BeaconTransport } from '../transport/beacon';
import { FetchTransport } from '../transport/fetch';
import { XhrTransport } from '../transport/xhr';
import { LocalStoragePersistence } from '../persistence/local-storage';
import { MemoryPersistence } from '../persistence/memory';
import {
  generateId,
  getExclusionState,
  isObject,
  isString,
  isValidEmail,
  parseQueryString,
} from '../utils/helpers';
import { RetryQueue } from '../utils/queue';
import { isWindowAvailable } from '../utils/common';
import { HttpsTransport } from '../transport/https';

export class HelpinClient {
  private config: Config;
  private logger: Logger;
  private cookieManager?: CookieManager;
  private transport: Transport;
  private persistence: LocalStoragePersistence | MemoryPersistence;
  private pageviewTracking?: PageviewTracking;
  private retryQueue: RetryQueue;
  private anonymousId: string;
  private namespace: string;

  constructor(config: Config) {
    // Ensure host has protocol so URLs aren't treated as relative paths
    if (config.host && !/^https?:\/\//.test(config.host)) {
      config.host = `https://${config.host}`;
    }
    this.config = this.mergeConfig(config, defaultConfig);
    this.logger = getLogger(this.config.logLevel);
    this.namespace = config.namespace || 'default';
    this.transport = this.initializeTransport(this.config);
    this.persistence = this.initializePersistence();
    this.retryQueue = new RetryQueue(
      this.transport,
      this.config.maxSendAttempts || 3,
      this.config.minSendTimeout || 1000,
      10,
      200, // Reduced interval to .2 second
      this.logger,
      this.namespace,
    );

    if (isWindowAvailable()) {
      this.initializeBrowserFeatures();
    }

    this.anonymousId = this.getOrCreateAnonymousId();

    this.logger.info(
      `Helpin client initialized for namespace: ${this.namespace}`,
    );
  }

  private initializeBrowserFeatures(): void {
    this.cookieManager = new CookieManager(this.config.cookieDomain);

    if (this.config.autoPageview) {
      this.pageviewTracking = new PageviewTracking(this);
    }

    if (this.config.crossDomainLinking) {
      this.manageCrossDomainLinking();
    }

    // Setup page leave tracking
    this.setupPageLeaveTracking();
  }

  /**
   * Recursively merge the provided configuration with the existing defaultConfig
   * @param config
   * @param defaultConfig
   */
  mergeConfig(config: Partial<Config>, defaultConfig: Partial<Config>): Config {
    // remove undefined values from the config
    const cleanConfig = JSON.parse(JSON.stringify(config));
    let newConfig = { ...defaultConfig, ...cleanConfig };

    // recursively merge objects
    Object.keys(defaultConfig).forEach((key) => {
      if (isObject(defaultConfig[key as keyof Config])) {
        newConfig[key] = this.mergeConfig(config[key], defaultConfig[key]);
      }
    });

    return newConfig as Config;
  }

  public init(config: Config): void {
    this.config = { ...this.config, ...config };
    this.logger = getLogger(this.config.logLevel);
    this.namespace = config.namespace || this.namespace;
    this.transport = this.initializeTransport(config);
    this.persistence = this.initializePersistence();
    this.retryQueue = new RetryQueue(
      this.transport,
      this.config.maxSendAttempts || 3,
      this.config.minSendTimeout || 1000,
      10,
      250, // Reduced interval to .25 second
      this.logger,
      this.namespace,
    );

    if (isWindowAvailable()) {
      this.initializeBrowserFeatures();
    }

    this.anonymousId = this.getOrCreateAnonymousId();

    this.logger.info(
      `Helpin client reinitialized for namespace: ${this.namespace}`,
    );
  }

  private manageCrossDomainLinking(): void {
    if (!this.config.crossDomainLinking || !this.config.domains) {
      return;
    }

    const domains = this.config.domains.split(',').map((d) => d.trim());
    const cookieName =
      this.config.cookieName || `helpin_aid_${this.config.widgetKey}`;

    document.addEventListener('click', (event) => {
      const target = this.findClosestLink(event.target as HTMLElement);
      if (!target) return;

      const href = target.getAttribute('href');
      if (!href || !href.startsWith('http')) return;

      const url = new URL(href);
      if (url.hostname === window.location.hostname) return;

      if (domains.includes(url.hostname)) {
        const cookie = this.cookieManager?.get(cookieName);
        if (cookie) {
          url.searchParams.append('_hp', cookie);
          target.setAttribute('href', url.toString());
        }
      }
    });

    this.logger.debug('Cross-domain linking initialized');
  }

  private findClosestLink(
    element: HTMLElement | null,
  ): HTMLAnchorElement | null {
    while (element && element.tagName !== 'A') {
      element = element.parentElement;
    }
    return element as HTMLAnchorElement;
  }

  private initializeTransport(config: Config): Transport {
    const fallback = 'https://events.helpin.ai';

    if (!isWindowAvailable()) {
      return new HttpsTransport(config.host || fallback, config);
    }

    const isXhrAvailable = 'XMLHttpRequest' in window;
    const isFetchAvailable = typeof fetch !== 'undefined';
    const isBeaconAvailable =
      typeof navigator !== 'undefined' && 'sendBeacon' in navigator;

    if (config.useBeaconApi && isBeaconAvailable) {
      return new BeaconTransport(
        config.host || fallback,
        config,
        this.logger,
      );
    } else if (config.forceUseFetch && isFetchAvailable) {
      return new FetchTransport(
        config.host || fallback,
        config,
        this.logger,
      );
    } else if (isXhrAvailable) {
      return new XhrTransport(
        config.host || fallback,
        config,
        this.logger,
      );
    } else if (isFetchAvailable) {
      return new FetchTransport(
        config.host || fallback,
        config,
        this.logger,
      );
    } else {
      throw new Error('No suitable transport method available');
    }
  }

  private initializePersistence(): LocalStoragePersistence | MemoryPersistence {
    if (this.config.disableEventPersistence || !isWindowAvailable()) {
      return new MemoryPersistence();
    } else {
      return new LocalStoragePersistence(
        `${this.namespace}_${this.config.widgetKey}`,
        this.logger,
      );
    }
  }

  private getOrCreateAnonymousId(): string {
    if (!isWindowAvailable()) {
      return generateId(); // Use a function to generate a unique ID for server-side
    }

    if (
      this.config.privacyPolicy === 'strict' ||
      this.config.cookiePolicy === 'strict'
    ) {
      return ''; // empty in case of strict policy
    }

    const cookieName =
      this.config.cookieName || `helpin_aid_${this.config.widgetKey}`;
    let id = this.cookieManager?.get(cookieName);

    if (!id) {
      if (this.config.crossDomainLinking) {
        const urlParams = new URLSearchParams(window.location.search);
        const queryId = urlParams.get('_hp');

        const urlHash = window.location.hash.substring(1);
        const hashedValues = urlHash.split('~');
        const fragmentId =
          hashedValues.length > 1 ? hashedValues[1] : undefined;

        id = queryId || fragmentId || generateId();
      }

      if (!id) {
        id = generateId();
      }

      // Set cookie for 10 years
      const tenYearsInDays = 365 * 10;
      this.cookieManager?.set(
        cookieName,
        id,
        tenYearsInDays,
        document.location.protocol !== 'http:',
        false,
      );
    }

    return id;
  }

  public async id(
    userData: UserProps,
    doNotSendEvent: boolean = false,
  ): Promise<void> {
    if (!isObject(userData)) {
      throw new Error('User data must be an object');
    }

    if (userData.email && !isValidEmail(userData.email)) {
      throw new Error('Invalid email provided');
    }

    if (!userData.id || !isString(userData.id)) {
      throw new Error('User ID must be a string');
    }

    const userId = userData.id;
    this.persistence.set('userId', userId);
    this.persistence.set('userProps', userData);

    if (!doNotSendEvent) {
      const identifyPayload = {
        ...userData,
        anonymous_id: this.anonymousId,
      };

      await this.track('user_identify', identifyPayload);
    }

    // Also send to Go backend for CRM contact creation + conversation backfill
    if (userData.email) {
      this.sendIdentifyToBackend(userData.email.trim(), userData.name || '', 'sdk_identify');
    }

    this.logger.info('User identified:', userData);
  }

  public track(
    typeName: string,
    payload?: EventPayload,
    directSend: boolean = false,
  ): void {
    this.trackInternal(typeName, payload, directSend);
  }

  public lead(payload: EventPayload, directSend: boolean = false): void {
    if (!isObject(payload)) {
      throw new Error(
        'Lead payload must be a non-null object and not an array',
      );
    }

    const email = payload.email;

    if (!isString(email)) {
      this.logger.error('Lead event requires a valid email attribute');
      return;
    }

    const trimmedEmail = email.trim();

    if (!trimmedEmail || !isValidEmail(trimmedEmail)) {
      this.logger.error('Lead event requires a valid email attribute');
      return;
    }

    payload.email = trimmedEmail;

    this.track('lead', payload, directSend);

    // Also send to Go backend for CRM lead creation + conversation backfill
    this.sendIdentifyToBackend(trimmedEmail, (payload.name as string) || '', 'sdk_lead');
  }

  private trackInternal(
    typeName: string,
    payload?: EventPayload,
    directSend: boolean = false,
  ): void {
    // Check if user has opted out of tracking
    const exclusionState = getExclusionState();

    if (exclusionState) {
      this.logger.debug(`Tracking disabled due to helpin_exclusion setting`);
      return;
    }

    if (!isString(typeName)) {
      throw new Error('Event name must be a string');
    }

    if (
      payload !== undefined &&
      (typeof payload !== 'object' ||
        payload === null ||
        Array.isArray(payload))
    ) {
      throw new Error(
        'Event payload must be a non-null object and not an array',
      );
    }

    const eventPayload = this.createEventPayload(typeName, payload);

    try {
      if (directSend) {
        this.transport.send(eventPayload);
        this.logger.debug(`Event sent: ${typeName}`, [eventPayload]);
        return;
      }
      this.retryQueue.add(eventPayload);
      this.logger.debug(`Event tracked: ${typeName}`, [eventPayload]);
    } catch (error) {
      this.logger.error(`Failed to track event: ${typeName}`, error);
      throw new Error(`Failed to track event: ${typeName}`);
    }
  }

  public rawTrack(payload: any): void {
    if (!isObject(payload)) {
      throw new Error('Event payload must be an object');
    }

    this.track('raw', payload);
  }

  public async group(
    props: CompanyProps,
    doNotSendEvent: boolean = false,
  ): Promise<void> {
    if (!isObject(props)) {
      throw new Error('Company properties must be an object');
    }

    if (!props.id || !props.name || !props.created_at) {
      throw new Error(
        'Company properties must include id, name, and created_at',
      );
    }

    this.persistence.set('companyProps', props);

    if (!doNotSendEvent) {
      await this.track('group', props);
    }

    this.logger.info('Company identified:', props);
  }

  private createEventPayload(
    eventName: string,
    eventProps?: EventPayload,
  ): any {
      const { event_id: incomingEventId, ...restEventProps } = eventProps || {};
    const userProps = this.persistence.get('userProps') || {};
    const companyProps =
      this.persistence.get('companyProps') || userProps?.company || {};
    const userId = this.persistence.get('userId');
    const globalProps = this.persistence.get('global_props') || {};
    const eventTypeProps = this.persistence.get(`props_${eventName}`) || {};

    let processedProps = restEventProps;

    const payload: any = {
      event_id: incomingEventId || generateId(),
      user: {
        anonymous_id: this.anonymousId,
        id: userId,
        ...userProps,
      },
      ...(companyProps && { company: companyProps }),
      ids: this.getThirdPartyIds(),
      utc_time: new Date().toISOString(),
      local_tz_offset: new Date().getTimezoneOffset(),
      api_key: this.config.widgetKey,
      src: 'helpin',
      event_type: eventName,
      namespace: this.namespace,
      ...globalProps,
      ...eventTypeProps,
    };

    if (eventName !== 'user_identify' && eventName !== 'group') {
      if (Array.isArray(this.config.propertyBlacklist)) {
        this.config.propertyBlacklist.forEach((prop) => {
          delete processedProps[prop];
        });
      }
      payload.event_attributes = processedProps;
    }

    if (isWindowAvailable()) {
      payload.referer = document.referrer;
      payload.url = window.location.href;
      payload.page_title = document.title;
      payload.doc_path = window.location.pathname;
      payload.doc_host = window.location.hostname;
      payload.doc_search = window.location.search;
      payload.screen_resolution = `${window.screen.width}x${window.screen.height}`;
      payload.vp_size = `${window.innerWidth}x${window.innerHeight}`;
      payload.user_agent = navigator.userAgent;
      payload.user_language = navigator.language;
      payload.doc_encoding = document.characterSet;
      payload.utm = this.getUtmParams();
    }

    return payload;
  }

  public getCookie(name: string): string | null {
    return this.cookieManager?.get(name) || null;
  }

  private getThirdPartyIds(): Record<string, string> {
    const thirdPartyIds: Record<string, string> = {};
    if (isWindowAvailable()) {
      const fbpCookie = this.getCookie('_fbp');
      if (fbpCookie) {
        thirdPartyIds['fbp'] = fbpCookie;
      }
    }
    return thirdPartyIds;
  }

  private getUtmParams(): Record<string, string> {
    const utmParams: Record<string, string> = {};
    const queryParams = parseQueryString(window.location.search);
    const utmKeys = [
      'utm_source',
      'utm_medium',
      'utm_campaign',
      'utm_term',
      'utm_content',
    ];

    utmKeys.forEach((key) => {
      if (queryParams[key]) {
        utmParams[key.replace('utm_', '')] = queryParams[key];
      }
    });

    return utmParams;
  }

  public pageview(): void {
    if (isWindowAvailable()) {
      this.track(
        'pageview',
        {
          url: window.location.href,
          referrer: document.referrer,
          title: document.title,
        },
        true,
      );
    } else {
      this.logger.warn(
        'Pageview tracking is not available in server-side environments',
      );
    }
  }

  private setupPageLeaveTracking(): void {
    if (!isWindowAvailable()) return;

    let isLeaving = false;
    let isRefreshing = false;

    const trackPageLeave = () => {
      if (!isLeaving && !isRefreshing) {
        isLeaving = true;
        this.track('$pageleave', {
          url: window.location.href,
          referrer: document.referrer,
          title: document.title,
        });
      }
    };

    // Check for refresh
    window.addEventListener('beforeunload', (event) => {
      isRefreshing = true;
      setTimeout(() => {
        isRefreshing = false;
      }, 100);
    });

    // Track on visibilitychange event (when the page becomes hidden)
    document.addEventListener('visibilitychange', () => {
      if (document.visibilityState === 'hidden' && !isRefreshing) {
        trackPageLeave();
      }
    });

    // For Single Page Applications, track when the user navigates away
    const originalPushState = history.pushState;
    history.pushState = function () {
      trackPageLeave();
      return originalPushState.apply(this, arguments as any);
    };

    window.addEventListener('popstate', trackPageLeave);
  }

  /**
   * Sends identity data to the Go backend for CRM contact creation and conversation backfill.
   * If the widget WebSocket is open, sends via session:upgrade; otherwise falls back to HTTP POST.
   */
  private sendIdentifyToBackend(email: string, name: string, source: string): void {
    // Try widget WS path first via the public sendSessionUpgrade method
    const namespace = this.config.namespace || 'helpin';
    const nsFunc = (globalThis as any)[namespace];
    if (nsFunc?._widgetManager?.sendSessionUpgrade?.(email, name, source)) {
      return;
    }

    // HTTP fallback: POST /api/widget/identify
    const host = this.config.host || 'https://events.helpin.ai';
    // Derive the API host from host (strip /api/v1/event suffix if present)
    const apiHost = host.replace(/\/api\/v1\/event\/?$/, '').replace(/\/+$/, '');

    const body = JSON.stringify({
      api_key: this.config.widgetKey,
      anonymous_id: this.anonymousId,
      email,
      name,
      source,
    });

    if (typeof fetch !== 'undefined') {
      fetch(`${apiHost}/widget/identify`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body,
      }).catch((err) => {
        this.logger.error('Failed to send identify to backend:', err);
      });
    }
  }

  public getConfig(): Config {
    return this.config;
  }

  public getLogger(): Logger {
    return this.logger;
  }

  public async reset(resetAnonId: boolean = false): Promise<void> {
    this.persistence.clear();

    if (resetAnonId && this.cookieManager) {
      const cookieName =
        this.config.cookieName || `helpin_aid_${this.config.widgetKey}`;
      this.cookieManager.delete(cookieName);
      this.anonymousId = this.getOrCreateAnonymousId();
    }

    this.logger.info('core state reset', {
      resetAnonId,
      namespace: this.namespace,
    });
  }

  public set(
    properties: Record<string, any>,
    opts?: { eventType?: string; persist?: boolean },
  ): void {
    if (!isObject(properties)) {
      throw new Error('Properties must be an object');
    }

    const eventType = opts?.eventType;
    const persist = opts?.persist ?? true;

    if (eventType) {
      let props = this.persistence.get(`props_${eventType}`) || {};
      props = { ...props, ...properties };
      this.persistence.set(`props_${eventType}`, props);
    } else {
      let globalProps = this.persistence.get('global_props') || {};
      globalProps = { ...globalProps, ...properties };
      this.persistence.set('global_props', globalProps);
    }

    if (persist) {
      this.persistence.save();
    }

    this.logger.debug(`Properties set`, {
      properties,
      eventType: eventType || 'global',
      persist,
    });
  }

  public setUserId(userId: string): void {
    // update the user id in the persistence
    this.persistence.set('userId', userId);

    // also in the user props
    let userProps = this.persistence.get('userProps') || {};
    userProps['id'] = userId;
    this.persistence.set('userProps', userProps);

    this.persistence.save();
  }

  public unset(
    propertyName: string,
    options?: { eventType?: string; persist?: boolean },
  ): void {
    const eventType = options?.eventType;
    const persist = options?.persist ?? true;

    if (eventType) {
      let props = this.persistence.get(`props_${eventType}`) || {};
      delete props[propertyName];
      this.persistence.set(`props_${eventType}`, props);
    } else {
      let props = this.persistence.get('global_props') || {};
      delete props[propertyName];
      this.persistence.set('global_props', props);
    }

    if (persist) {
      this.persistence.save();
    }

    this.logger.debug(
      `Property unset: ${propertyName}`,
      `Event type: ${eventType || 'global'}`,
    );
  }
}
