import { describe, it, expect } from 'vitest';
import * as React from 'react';
import { render, screen } from '@testing-library/react';
import HelpinProvider from '../src/HelpinProvider';
import HelpinContext from '../src/HelpinContext';
import { HelpinClient } from '@helpin-ai/sdk-js';

function createMockClient(): HelpinClient {
  return new HelpinClient({ widgetKey: 'test', host: 'https://test.helpin.ai' });
}

describe('HelpinProvider (Next.js)', () => {
  it('should render children', () => {
    const client = createMockClient();
    render(
      <HelpinProvider client={client}>
        <div data-testid="child">Hello</div>
      </HelpinProvider>,
    );

    expect(screen.getByTestId('child')).toBeDefined();
    expect(screen.getByText('Hello')).toBeDefined();
  });

  it('should provide the client via context', () => {
    const client = createMockClient();
    let contextValue: HelpinClient | null = null;

    function Consumer() {
      contextValue = React.useContext(HelpinContext);
      return <div data-testid="consumer">consumed</div>;
    }

    render(
      <HelpinProvider client={client}>
        <Consumer />
      </HelpinProvider>,
    );

    expect(contextValue).toBe(client);
  });

  it('should accept null client for SSR', () => {
    render(
      <HelpinProvider client={null}>
        <div data-testid="child">SSR Child</div>
      </HelpinProvider>,
    );

    expect(screen.getByTestId('child')).toBeDefined();
    expect(screen.getByText('SSR Child')).toBeDefined();
  });

  it('should render multiple children', () => {
    const client = createMockClient();
    render(
      <HelpinProvider client={client}>
        <div data-testid="child-1">First</div>
        <div data-testid="child-2">Second</div>
      </HelpinProvider>,
    );

    expect(screen.getByTestId('child-1')).toBeDefined();
    expect(screen.getByTestId('child-2')).toBeDefined();
  });
});
