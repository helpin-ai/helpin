import { describe, it, expect, vi } from 'vitest';
import * as React from 'react';
import { render } from '@testing-library/react';
import HelpinProvider from '../src/HelpinProvider';
import usePageView from '../src/usePageView';
import { HelpinClient } from '@helpin-ai/sdk-js';

function createMockClient(): HelpinClient {
  const client = new HelpinClient({
    widgetKey: 'test',
    host: 'https://test.helpin.ai',
  });
  vi.spyOn(client, 'track');
  return client;
}

describe('usePageView (Next.js)', () => {
  it('should not throw when called with a valid client', () => {
    const client = createMockClient();

    function TestComponent() {
      usePageView(client);
      return <div data-testid="page">Page content</div>;
    }

    expect(() =>
      render(
        <HelpinProvider client={client}>
          <TestComponent />
        </HelpinProvider>,
      ),
    ).not.toThrow();
  });

  it('should not throw when called with null client (SSR)', () => {
    function TestComponent() {
      usePageView(null);
      return <div data-testid="page">SSR content</div>;
    }

    expect(() =>
      render(
        <HelpinProvider client={null}>
          <TestComponent />
        </HelpinProvider>,
      ),
    ).not.toThrow();
  });

  it('should return the client that was passed', () => {
    const client = createMockClient();
    let returnedClient: HelpinClient | null = null;

    function TestComponent() {
      returnedClient = usePageView(client);
      return <div>Test</div>;
    }

    render(
      <HelpinProvider client={client}>
        <TestComponent />
      </HelpinProvider>,
    );

    expect(returnedClient).toBe(client);
  });

  it('should return null when called with null', () => {
    let returnedClient: HelpinClient | null = 'not-null' as any;

    function TestComponent() {
      returnedClient = usePageView(null);
      return <div>Test</div>;
    }

    render(
      <HelpinProvider client={null}>
        <TestComponent />
      </HelpinProvider>,
    );

    expect(returnedClient).toBeNull();
  });

  it('should track a pageview when URL is set', () => {
    const client = createMockClient();

    function TestComponent() {
      usePageView(client);
      return <div>Tracked</div>;
    }

    render(
      <HelpinProvider client={client}>
        <TestComponent />
      </HelpinProvider>,
    );

    // The hook should call track with 'pageview' after useEffect runs
    // In jsdom the URL effect fires, triggering a pageview track
    expect(client.track).toHaveBeenCalled();
  });
});
