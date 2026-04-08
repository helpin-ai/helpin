import { describe, it, expect, vi, afterEach } from 'vitest';
import { helpinClient } from '../../src/esm-entry';
import { WidgetManager } from '../../src/core/widget';

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

  it('auto-boots the widget by default in browser environments', () => {
    const bootSpy = vi
      .spyOn(WidgetManager.prototype, 'boot')
      .mockImplementation(() => {});

    const client = helpinClient({
      widgetKey: 'test-key',
      host: 'https://test.helpin.ai',
    });

    expect(client).not.toBeNull();
    expect(bootSpy).toHaveBeenCalledWith({
      widgetKey: 'test-key',
      host: 'https://test.helpin.ai',
      user: undefined,
    });
  });

  it('supports lazy widget boot when autoBoot is false', () => {
    const bootSpy = vi
      .spyOn(WidgetManager.prototype, 'boot')
      .mockImplementation(() => {});
    const showSpy = vi
      .spyOn(WidgetManager.prototype, 'show')
      .mockImplementation(() => {});

    const client = helpinClient({
      widgetKey: 'test-key',
      host: 'https://test.helpin.ai',
      autoBoot: false,
    });

    expect(client).not.toBeNull();
    expect(bootSpy).not.toHaveBeenCalled();

    client?.show();

    expect(bootSpy).toHaveBeenCalledWith({
      widgetKey: 'test-key',
      host: 'https://test.helpin.ai',
      user: undefined,
    });
    expect(showSpy).toHaveBeenCalled();
  });
});
