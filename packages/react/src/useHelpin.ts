import { useCallback, useContext } from 'react';
import HelpinContext from './HelpinContext';
import { EventPayload, UserProps } from '@helpin-ai/sdk-js';

export type HelpinClient = {
  trackPageView: () => void;
  id: (userData: UserProps, doNotSendEvent?: boolean) => Promise<void>;
  track: (typeName: string, payload?: EventPayload) => void;
  lead: (payload: EventPayload, directSend?: boolean) => void;
  show: () => void;
  hide: () => void;
  open: () => void;
  close: () => void;
  toggle: () => void;
  openMessages: () => void;
  openNewMessage: (content?: string) => void;
  shutdown: () => void;
  rawTrack: (payload: any) => void;
  set: (
    properties: Record<string, any>,
    opts?: { eventType?: string; persist?: boolean },
  ) => void;
  unset: (
    propertyName: string,
    opts?: { eventType?: string; persist?: boolean },
  ) => void;
};

const missingClientMessage =
  '[Helpin] useHelpin() is running without an initialized client. Wrap your app in <HelpinProvider client={createClient(...)} /> and ensure widgetKey and host are defined.';

let hasLoggedMissingClient = false;

const noopClient: HelpinClient = {
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
  if (hasLoggedMissingClient) {
    return;
  }
  hasLoggedMissingClient = true;
  console.error(missingClientMessage);
}

function useHelpin(): HelpinClient {
  const client = useContext(HelpinContext);
  if (!client) {
    reportMissingClient();
    return noopClient;
  }

  const id = useCallback(
    (userData: UserProps, doNotSendEvent?: boolean): Promise<void> =>
      client?.id(userData, doNotSendEvent),
    [client],
  );

  const trackPageView = useCallback(
    (): void => client?.pageview(),
    [client],
  );

  const track = useCallback(
    (typeName: string, payload?: EventPayload): void =>
      client?.track(typeName, payload),
    [client],
  );

  const lead = useCallback(
    (payload: EventPayload, directSend?: boolean): void =>
      client?.lead(payload, directSend),
    [client],
  );

  const rawTrack = useCallback(
    (payload: any): void => client?.rawTrack(payload),
    [client],
  );

  const show = useCallback(
    (): void => client?.show(),
    [client],
  );

  const hide = useCallback(
    (): void => client?.hide(),
    [client],
  );

  const toggle = useCallback(
    (): void => client?.toggle(),
    [client],
  );

  const open = useCallback(
    (): void => client?.open(),
    [client],
  );

  const close = useCallback(
    (): void => client?.close(),
    [client],
  );

  const openMessages = useCallback(
    (): void => client?.openMessages(),
    [client],
  );

  const openNewMessage = useCallback(
    (content?: string): void => client?.openNewMessage(content),
    [client],
  );

  const shutdown = useCallback(
    (): void => client?.shutdown(),
    [client],
  );

  const set = useCallback(
    (
      properties: Record<string, any>,
      opts?: {
        eventType?: string;
        persist?: boolean;
      },
    ): void => client?.set(properties, opts),
    [client],
  );

  const unset = useCallback(
    (
      propertyName: string,
      opts?: {
        eventType?: string;
        persist?: boolean;
      },
    ): void => client?.unset(propertyName, opts),
    [client],
  );

  return {
    ...client,
    id,
    track,
    lead,
    trackPageView,
    show,
    hide,
    open,
    close,
    toggle,
    openMessages,
    openNewMessage,
    shutdown,
    rawTrack,
    set,
    unset,
  };
}

export default useHelpin;
