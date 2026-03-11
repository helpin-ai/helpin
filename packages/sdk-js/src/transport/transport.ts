export interface Transport {
  send(payload: any | any[]): Promise<void>;
}
