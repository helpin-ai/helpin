import { describe, it, expect } from 'vitest';
import createClient from '../src/client';

describe('createClient', () => {
  it('should create a client with the given config', () => {
    const config = { widgetKey: 'test-key', host: 'https://test.helpin.ai' };
    const client = createClient(config);

    expect(client).toBeDefined();
    expect(client).not.toBeNull();
  });

  it('should return an object with expected SDK methods', () => {
    const config = { widgetKey: 'test-key', host: 'https://test.helpin.ai' };
    const client = createClient(config);

    expect(typeof client.id).toBe('function');
    expect(typeof client.track).toBe('function');
    expect(typeof client.lead).toBe('function');
    expect(typeof client.rawTrack).toBe('function');
    expect(typeof client.set).toBe('function');
    expect(typeof client.unset).toBe('function');
  });

  it('should create distinct clients for different configs', () => {
    const client1 = createClient({
      widgetKey: 'key-1',
      host: 'https://one.helpin.ai',
    });
    const client2 = createClient({
      widgetKey: 'key-2',
      host: 'https://two.helpin.ai',
    });

    expect(client1).not.toBe(client2);
  });

  it('should return null when widgetKey is missing', () => {
    const client = createClient({
      host: 'https://test.helpin.ai',
    } as any);

    expect(client).toBeNull();
  });
});
