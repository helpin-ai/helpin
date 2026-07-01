import { describe, it, expect, vi, afterEach } from 'vitest';
import { helpinClient } from '../../src/esm-entry';

describe('helpinClient', () => {
  afterEach(() => {
    vi.restoreAllMocks();
    document.head.innerHTML = '';
    document.body.innerHTML = '';
    delete (window as any).helpin;
    delete (window as any).helpinQ;
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

  it('auto-boots the hosted widget runtime by default in browser environments', () => {
    const client = helpinClient({
      widgetKey: 'test-key',
      host: 'https://test.helpin.ai',
    });

    expect(client).not.toBeNull();

    const script = document.querySelector<HTMLScriptElement>('script[data-helpin-runtime="hosted"]');
    expect(script).not.toBeNull();
    expect(script?.src).toBe('https://cdn.helpin.ai/lib.js');
    expect(script?.getAttribute('data-widget-key')).toBe('test-key');
    expect(script?.getAttribute('data-host')).toBe('https://test.helpin.ai');
    expect(script?.getAttribute('data-no-auto-init')).toBe('true');
    expect((window as any).helpinQ).toEqual([
      ['boot', { widgetKey: 'test-key', host: 'https://test.helpin.ai', user: undefined }],
    ]);
  });

  it('supports lazy hosted widget boot when autoBoot is false', () => {
    const client = helpinClient({
      widgetKey: 'test-key',
      host: 'https://test.helpin.ai',
      autoBoot: false,
    });

    expect(client).not.toBeNull();
    expect(document.querySelector('script[data-helpin-runtime="hosted"]')).toBeNull();

    client?.show();

    expect(document.querySelector('script[data-helpin-runtime="hosted"]')).not.toBeNull();
    expect((window as any).helpinQ).toEqual([
      ['boot', { widgetKey: 'test-key', host: 'https://test.helpin.ai', user: undefined }],
      ['show'],
    ]);
  });

  it('allows npm consumers to override the hosted widget runtime URL', () => {
    const client = helpinClient({
      widgetKey: 'test-key',
      host: 'https://test.helpin.ai',
      widgetRuntimeUrl: 'https://cdn-stage.helpin.ai/lib.js',
    });

    expect(client).not.toBeNull();

    const script = document.querySelector<HTMLScriptElement>('script[data-helpin-runtime="hosted"]');
    expect(script?.src).toBe('https://cdn-stage.helpin.ai/lib.js');
  });

  it('reuses an existing script-tag runtime instead of injecting a duplicate', () => {
    const existingScript = document.createElement('script');
    existingScript.src = 'https://cdn.helpin.ai/lib.js';
    existingScript.setAttribute('data-widget-key', 'existing-key');
    existingScript.setAttribute('data-host', 'https://test.helpin.ai');
    document.head.appendChild(existingScript);

    const client = helpinClient({
      widgetKey: 'test-key',
      host: 'https://test.helpin.ai',
    });

    expect(client).not.toBeNull();
    expect(document.querySelectorAll('script[src="https://cdn.helpin.ai/lib.js"]')).toHaveLength(1);
    expect(document.querySelector('script[data-helpin-runtime="hosted"]')).toBeNull();
    expect((window as any).helpinQ).toEqual([
      ['boot', { widgetKey: 'test-key', host: 'https://test.helpin.ai', user: undefined }],
    ]);
  });
});
