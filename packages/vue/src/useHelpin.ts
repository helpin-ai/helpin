import { inject } from 'vue';
import type {
  EventPayload,
  LeadProps,
  UserProps,
} from '@helpin-ai/sdk-js';
import { HelpinKey } from './injection';

export interface HelpinComposable {
  trackPageView(): void;
  id(userData: UserProps, doNotSendEvent?: boolean): Promise<void>;
  track(typeName: string, payload?: EventPayload): void;
  lead(payload: LeadProps, directSend?: boolean): void;
  show(): void;
  hide(): void;
  open(): void;
  close(): void;
  toggle(): void;
  openMessages(): void;
  openNewMessage(content?: string): void;
  shutdown(): void;
  rawTrack(payload: unknown): void;
  set(
    properties: Record<string, unknown>,
    opts?: { eventType?: string; persist?: boolean },
  ): void;
  unset(
    propertyName: string,
    opts?: { eventType?: string; persist?: boolean },
  ): void;
}

const missingClientMessage =
  '[Helpin] useHelpin() is running without an initialized client. Install HelpinPlugin with app.use(HelpinPlugin, { client: createClient(...) }).';

let hasLoggedMissingClient = false;

const noopClient: HelpinComposable = {
  trackPageView: () => {},
  id: async () => {},
  track: () => {},
  lead: () => {},
  show: () => {},
  hide: () => {},
  open: () => {},
  close: () => {},
  toggle: () => {},
  openMessages: () => {},
  openNewMessage: () => {},
  shutdown: () => {},
  rawTrack: () => {},
  set: () => {},
  unset: () => {},
};

function reportMissingClient(): void {
  if (typeof window === 'undefined' || hasLoggedMissingClient) {
    return;
  }

  hasLoggedMissingClient = true;
  console.error(missingClientMessage);
}

export default function useHelpin(): HelpinComposable {
  const client = inject(HelpinKey, null);
  if (!client) {
    reportMissingClient();
    return noopClient;
  }

  return {
    trackPageView: () => client.pageview(),
    id: (userData, doNotSendEvent) => client.id(userData, doNotSendEvent),
    track: (typeName, payload) => client.track(typeName, payload),
    lead: (payload, directSend) => client.lead(payload, directSend),
    show: () => client.show(),
    hide: () => client.hide(),
    open: () => client.open(),
    close: () => client.close(),
    toggle: () => client.toggle(),
    openMessages: () => client.openMessages(),
    openNewMessage: (content) => client.openNewMessage(content),
    shutdown: () => client.shutdown(),
    rawTrack: (payload) => client.rawTrack(payload),
    set: (properties, opts) => client.set(properties, opts),
    unset: (propertyName, opts) => client.unset(propertyName, opts),
  };
}
