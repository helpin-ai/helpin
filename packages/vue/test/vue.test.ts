import { defineComponent, h, nextTick } from 'vue';
import { mount } from '@vue/test-utils';
import { describe, expect, it, vi } from 'vitest';
import { HelpinClient } from '@helpin-ai/sdk-js';
import createClient from '../src/client';
import { HelpinPlugin } from '../src/plugin';
import useHelpin from '../src/useHelpin';
import usePageView from '../src/usePageView';

function makeClient(): HelpinClient {
  return new HelpinClient({
    widgetKey: 'test-key',
    host: 'https://test.helpin.ai',
  });
}

describe('@helpin-ai/vue', () => {
  it('creates a browser client and preserves its options', () => {
    const client = createClient({
      widgetKey: 'test-key',
      host: 'https://test.helpin.ai',
      autoBoot: false,
    });

    expect(client).not.toBeNull();
    expect(client?.getConfig().autoBoot).toBe(false);
  });

  it('provides analytics and widget methods through useHelpin', () => {
    const client = makeClient();
    const track = vi.spyOn(client, 'track');
    const open = vi.spyOn(client, 'open');
    const component = defineComponent({
      setup() {
        const helpin = useHelpin();
        helpin.track('checkout_started', { plan: 'pro' });
        helpin.open();
        return () => h('div');
      },
    });

    mount(component, {
      global: { plugins: [[HelpinPlugin, { client }]] },
    });

    expect(track).toHaveBeenCalledWith('checkout_started', { plan: 'pro' });
    expect(open).toHaveBeenCalledOnce();
  });

  it('returns safe no-op methods when the plugin is missing', () => {
    const error = vi.spyOn(console, 'error').mockImplementation(() => {});
    let helpin: ReturnType<typeof useHelpin> | undefined;
    const component = defineComponent({
      setup() {
        helpin = useHelpin();
        return () => h('div');
      },
    });

    mount(component);

    expect(() => helpin?.open()).not.toThrow();
    expect(error).toHaveBeenCalledOnce();
    error.mockRestore();
  });

  it('tracks the initial page and Vue Router navigation', async () => {
    const client = makeClient();
    const track = vi.spyOn(client, 'track');
    let navigationHook: (() => void) | undefined;
    const removeHook = vi.fn();
    const router = {
      afterEach: vi.fn((hook: () => void) => {
        navigationHook = hook;
        return removeHook;
      }),
    };
    const component = defineComponent({
      setup() {
        usePageView({ router: router as never, payload: { framework: 'vue' } });
        return () => h('div');
      },
    });

    const wrapper = mount(component, {
      global: { plugins: [[HelpinPlugin, { client }]] },
    });
    await nextTick();

    expect(track).toHaveBeenCalledWith(
      'pageview',
      expect.objectContaining({ framework: 'vue', url: window.location.href }),
    );

    window.history.pushState({}, '', '/pricing');
    navigationHook?.();
    expect(track).toHaveBeenCalledTimes(2);

    wrapper.unmount();
    expect(removeHook).toHaveBeenCalledOnce();
  });
});
