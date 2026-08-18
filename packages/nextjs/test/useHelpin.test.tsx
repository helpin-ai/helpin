import { describe, it, expect, vi } from 'vitest';
import * as React from 'react';
import { render, screen } from '@testing-library/react';
import { renderHook } from '@testing-library/react';
import HelpinProvider from '../src/HelpinProvider';
import useHelpin from '../src/useHelpin';
import { HelpinClient } from '@helpin-ai/sdk-js';

function createMockClient(): HelpinClient {
  return new HelpinClient({ widgetKey: 'test', host: 'https://test.helpin.ai' });
}

function createWrapper(client: HelpinClient | null) {
  return function Wrapper({ children }: { children: React.ReactNode }) {
    return <HelpinProvider client={client}>{children}</HelpinProvider>;
  };
}

describe('useHelpin (Next.js)', () => {
  it('should return an object with id method', () => {
    const client = createMockClient();
    const { result } = renderHook(() => useHelpin(), {
      wrapper: createWrapper(client),
    });

    expect(typeof result.current.id).toBe('function');
  });

  it('should return an object with track method', () => {
    const client = createMockClient();
    const { result } = renderHook(() => useHelpin(), {
      wrapper: createWrapper(client),
    });

    expect(typeof result.current.track).toBe('function');
  });

  it('should return an object with lead method', () => {
    const client = createMockClient();
    const { result } = renderHook(() => useHelpin(), {
      wrapper: createWrapper(client),
    });

    expect(typeof result.current.lead).toBe('function');
  });

  it('should return an object with trackPageView method', () => {
    const client = createMockClient();
    const { result } = renderHook(() => useHelpin(), {
      wrapper: createWrapper(client),
    });

    expect(typeof result.current.trackPageView).toBe('function');
  });

  it('should return an object with rawTrack method', () => {
    const client = createMockClient();
    const { result } = renderHook(() => useHelpin(), {
      wrapper: createWrapper(client),
    });

    expect(typeof result.current.rawTrack).toBe('function');
  });

  it('should return an object with set and unset methods', () => {
    const client = createMockClient();
    const { result } = renderHook(() => useHelpin(), {
      wrapper: createWrapper(client),
    });

    expect(typeof result.current.set).toBe('function');
    expect(typeof result.current.unset).toBe('function');
  });

  it('should return widget control methods', () => {
    const client = createMockClient();
    const { result } = renderHook(() => useHelpin(), {
      wrapper: createWrapper(client),
    });

    expect(typeof result.current.show).toBe('function');
    expect(typeof result.current.hide).toBe('function');
    expect(typeof result.current.open).toBe('function');
    expect(typeof result.current.close).toBe('function');
    expect(typeof result.current.toggle).toBe('function');
    expect(typeof result.current.openMessages).toBe('function');
    expect(typeof result.current.openNewMessage).toBe('function');
    expect(typeof result.current.openConversation).toBe('function');
    expect(typeof result.current.openArticle).toBe('function');
    expect(typeof result.current.shutdown).toBe('function');
  });

  it('should forward article and conversation controls to the client', () => {
    const client = createMockClient();
    const openConversation = vi.spyOn(client, 'openConversation');
    const openArticle = vi.spyOn(client, 'openArticle');
    const { result } = renderHook(() => useHelpin(), {
      wrapper: createWrapper(client),
    });

    result.current.openConversation('conversation-123');
    result.current.openArticle('how-to-add-first-comment-2906b16e', {
      spaceId: 'space-123',
    });

    expect(openConversation).toHaveBeenCalledWith('conversation-123');
    expect(openArticle).toHaveBeenCalledWith(
      'how-to-add-first-comment-2906b16e',
      { spaceId: 'space-123' },
    );
  });

  it('should return no-op client when client is null (SSR)', () => {
    // Next.js createClient returns null on the server
    const { result } = renderHook(() => useHelpin(), {
      wrapper: createWrapper(null),
    });

    // Should return no-op functions instead of throwing
    expect(typeof result.current.id).toBe('function');
    expect(typeof result.current.track).toBe('function');
    expect(typeof result.current.lead).toBe('function');
    expect(typeof result.current.trackPageView).toBe('function');
    expect(typeof result.current.show).toBe('function');
    expect(typeof result.current.hide).toBe('function');
    expect(typeof result.current.open).toBe('function');
    expect(typeof result.current.close).toBe('function');
    expect(typeof result.current.toggle).toBe('function');
    expect(typeof result.current.openMessages).toBe('function');
    expect(typeof result.current.openNewMessage).toBe('function');
    expect(typeof result.current.openConversation).toBe('function');
    expect(typeof result.current.openArticle).toBe('function');
    expect(typeof result.current.shutdown).toBe('function');
    expect(typeof result.current.rawTrack).toBe('function');
    expect(typeof result.current.set).toBe('function');
    expect(typeof result.current.unset).toBe('function');
  });

  it('should work inside a component tree with HelpinProvider', () => {
    const client = createMockClient();

    function TestComponent() {
      const helpin = useHelpin();
      return (
        <div data-testid="result">
          {typeof helpin.id === 'function' ? 'has-id' : 'no-id'}
        </div>
      );
    }

    render(
      <HelpinProvider client={client}>
        <TestComponent />
      </HelpinProvider>,
    );

    expect(screen.getByTestId('result').textContent).toBe('has-id');
  });
});
