import { useCallback, useContext } from 'react';
import HelpinContext from './HelpinContext';
import { EventPayload, UserProps } from '@helpin/sdk-js';

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

/**
 * See for details http://jitsu.com/docs/sending-data/js-sdk/react
 */
function useHelpin(): HelpinClient {
  const client = useContext(HelpinContext);
  if (!client) {
    throw new Error(
      'Before calling useHelpin() hook, please wrap your component into <JitsuProvider />. Read more in http://jitsu.com/docs/sending-data/js-sdk/react',
    );
  }

  const id = useCallback(
    (userData: UserProps, doNotSendEvent?: boolean): Promise<void> =>
      client?.id(userData, doNotSendEvent),
    [client],
  );

  const trackPageView = useCallback(
    (): void => client?.track('pageview'),
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
    rawTrack,
    set,
    unset,
  };
}

export default useHelpin;
