import { describe, it, expect, vi, afterEach } from 'vitest';
import { helpinClient } from '../../src/esm-entry';

describe('helpinClient', () => {
  afterEach(() => {
    vi.restoreAllMocks();
  });

  it('returns null and logs when widgetKey is missing', () => {
    const errorSpy = vi.spyOn(console, 'error').mockImplementation(() => {});

    const client = helpinClient({
      host: 'https://test.helpin.ai',
    } as any);

    expect(client).toBeNull();
    expect(errorSpy).toHaveBeenCalled();
  });

  it('creates a client when required config is present', () => {
    const client = helpinClient({
      widgetKey: 'test-key',
      host: 'https://test.helpin.ai',
    });

    expect(client).not.toBeNull();
  });
});
