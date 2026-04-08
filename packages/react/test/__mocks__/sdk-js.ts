/**
 * Mock for @helpin-ai/sdk-js used in React package tests.
 */
export class HelpinClient {
  private config: any;

  constructor(config: any) {
    this.config = config;
  }

  id = async (_userData: any, _doNotSendEvent?: boolean): Promise<void> => {};
  track = (_typeName: string, _payload?: any): void => {};
  lead = (_payload: any, _directSend?: boolean): void => {};
  pageview = (): void => {};
  set = (_properties: Record<string, any>, _opts?: any): void => {};
  unset = (_propertyName: string, _opts?: any): void => {};
  rawTrack = (_payload: any): void => {};
  setUserId = (_userId: string): void => {};
  getConfig = (): any => this.config;
}

export type HelpinOptions = {
  widgetKey: string;
  host: string;
  [key: string]: any;
};

export type UserProps = {
  id?: string;
  email?: string;
  name?: string;
  [key: string]: any;
};

export type EventPayload = Record<string, any>;

export type ClientProperties = Record<string, any>;

export function helpinClient(config: Partial<HelpinOptions>): HelpinClient | null {
  if (!config.widgetKey) {
    console.error('[Helpin] Widget initialization skipped: widgetKey is required.');
    return null;
  }
  return new HelpinClient(config);
}
