export class HelpinClient {
  constructor(private config: Record<string, unknown>) {}

  id = async (): Promise<void> => {};
  track = (): void => {};
  lead = (): void => {};
  pageview = (): void => {};
  show = (): void => {};
  hide = (): void => {};
  open = (): void => {};
  close = (): void => {};
  toggle = (): void => {};
  openMessages = (): void => {};
  openNewMessage = (): void => {};
  openConversation = (_conversationId: string): void => {};
  openArticle = (_articleKey: string, _options?: ShowArticleOptions): void => {};
  shutdown = (): void => {};
  set = (): void => {};
  unset = (): void => {};
  rawTrack = (): void => {};
  getConfig = (): Record<string, unknown> => this.config;
}

export type HelpinOptions = Record<string, unknown> & { widgetKey: string };
export type UserProps = Record<string, unknown>;
export type EventPayload = Record<string, unknown>;
export type LeadProps = Record<string, unknown>;
export type ShowArticleOptions = {
  collectionId?: string;
  spaceId?: string;
};

export function helpinClient(
  config: HelpinOptions,
): HelpinClient | null {
  return config.widgetKey ? new HelpinClient(config) : null;
}
