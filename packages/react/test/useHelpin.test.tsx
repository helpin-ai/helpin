import { describe, it, expect } from 'vitest';
import { vi } from 'vitest';
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

describe('useHelpin', () => {
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
    expect(typeof result.current.toggle).toBe('function');
    expect(typeof result.current.showMessages).toBe('function');
    expect(typeof result.current.showNewMessage).toBe('function');
    expect(typeof result.current.shutdown).toBe('function');
  });

  it('should return a no-op client when used outside HelpinProvider', () => {
    const errorSpy = vi.spyOn(console, 'error').mockImplementation(() => {});

    const { result } = renderHook(() => useHelpin());

    expect(typeof result.current.id).toBe('function');
    expect(typeof result.current.track).toBe('function');
    expect(typeof result.current.show).toBe('function');
    expect(typeof result.current.hide).toBe('function');
    expect(typeof result.current.toggle).toBe('function');
    expect(typeof result.current.showMessages).toBe('function');
    expect(typeof result.current.showNewMessage).toBe('function');
    expect(typeof result.current.shutdown).toBe('function');
    expect(errorSpy).toHaveBeenCalled();
    errorSpy.mockRestore();
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
