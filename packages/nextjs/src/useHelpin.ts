import { useCallback, useContext } from 'react';
import HelpinContext from './HelpinContext';
import { EventPayload, UserProps } from '@helpin-ai/sdk-js';

export type HelpinClient = {
  trackPageView: () => void;
  id: (userData: UserProps, doNotSendEvent?: boolean) => Promise<void>;
  track: (typeName: string, payload?: EventPayload) => void;
  lead: (payload: EventPayload, directSend?: boolean) => void;
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

// Create a no-op client for server-side rendering
const createNoopClient = (): HelpinClient => ({
  trackPageView: () => {},
  id: async () => {},
  track: () => {},
  lead: () => {},
  rawTrack: () => {},
  set: () => {},
  unset: () => {},
});

function useHelpin(): HelpinClient {
  const client = useContext(HelpinContext);

  // Return no-op client if we're in a server environment or client is not initialized
  if (!client) {
    return createNoopClient();
  }

  const id = useCallback(
    (userData: UserProps, doNotSendEvent?: boolean): Promise<void> =>
      client.id(userData, doNotSendEvent),
    [client],
  );

  const trackPageView = useCallback(
    (): void => client.track('pageview'),
    [client],
  );

  const track = useCallback(
    (typeName: string, payload?: EventPayload): void =>
      client.track(typeName, payload),
    [client],
  );

  const lead = useCallback(
    (payload: EventPayload, directSend?: boolean): void =>
      client.lead(payload, directSend),
    [client],
  );

  const rawTrack = useCallback(
    (payload: any): void => client.rawTrack(payload),
    [client],
  );

  const set = useCallback(
    (
      properties: Record<string, any>,
      opts?: {
        eventType?: string;
        persist?: boolean;
      },
    ): void => client.set(properties, opts),
    [client],
  );

  const unset = useCallback(
    (
      propertyName: string,
      opts?: {
        eventType?: string;
        persist?: boolean;
      },
    ): void => client.unset(propertyName, opts),
    [client],
  );

  return {
    ...client,
    id,
    track,
    lead,
    trackPageView,
    rawTrack,
    set,
    unset,
  };
}

export default useHelpin;
