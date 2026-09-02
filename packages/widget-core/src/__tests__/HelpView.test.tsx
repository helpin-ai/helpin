import { act, fireEvent, render, waitFor } from '@testing-library/preact';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { HelpView } from '../components/HelpView';
import type { WidgetConfig } from '../types';

const configWithOneHelpSpace: WidgetConfig = {
  workspaceId: 'ws_123',
  workspaceName: 'Acme',
  branding: {
    primaryColor: '#6366f1',
    welcomeMessage: 'How can we help?',
    widgetPosition: 'bottom-right',
    showBranding: true,
    colorScheme: 'light',
  },
  features: {
    aiEnabled: false,
    aiFirst: false,
    showTalkToHuman: false,
    escalationMessage: 'Let me connect you with a team member who can help further.',
    fileUploads: false,
    preChatForm: false,
    requirePhone: false,
    csatRating: false,
    forceIdentify: false,
  },
  helpSpaces: [{ id: 'space-1', name: 'Developer Docs', slug: 'developer-docs' }],
};

function stubEmptyCollections() {
  vi.stubGlobal(
    'fetch',
    vi.fn(async () => new Response(JSON.stringify([]), { status: 200 })),
  );
}

function stubCollectionsAndSearch() {
  const fetchMock = vi.fn(async (input: RequestInfo | URL) => {
    const url = String(input);
    if (url.includes('/widget/support/help/search')) {
      return new Response(
        JSON.stringify([
          {
            id: 'doc-1',
            title: 'Reset password',
            slug: 'reset-password',
            public_id: 'abc12345',
            article_key: 'reset-password-abc12345',
            excerpt: 'Steps to reset your password.',
            collection_name: 'Accounts',
          },
        ]),
        { status: 200 },
      );
    }
    return new Response(JSON.stringify([]), { status: 200 });
  });
  vi.stubGlobal('fetch', fetchMock);
  return fetchMock;
}

function deferredResponse() {
  let resolve!: (response: Response) => void;
  const promise = new Promise<Response>((resolver) => {
    resolve = resolver;
  });
  return { promise, resolve };
}

describe('HelpView', () => {
  afterEach(() => {
    vi.useRealTimers();
    vi.unstubAllGlobals();
  });

  it('uses Help Center as the tab heading', () => {
    stubEmptyCollections();

    const { container } = render(
      <HelpView
        config={configWithOneHelpSpace}
        host="https://client.helpin.ai"
        widgetKey="wk_123"
        onSelectSpace={() => {}}
        onSelectCollection={() => {}}
      />,
    );

    expect(container.querySelector('.helpin-help-header .helpin-help-title')?.textContent).toBe('Help Center');
  });

  it('does not show Contact us in the Help tab', async () => {
    stubEmptyCollections();

    const { getByText, queryByText } = render(
      <HelpView
        config={configWithOneHelpSpace}
        host="https://client.helpin.ai"
        widgetKey="wk_123"
        onSelectSpace={() => {}}
        onSelectCollection={() => {}}
      />,
    );

    await waitFor(() => {
      expect(getByText('No published collections are available yet.')).toBeTruthy();
    });

    expect(queryByText('Contact us')).toBeNull();
  });

  it('does not show the docs space name when only one help space is configured', async () => {
    stubEmptyCollections();

    const { getByText, queryByText } = render(
      <HelpView
        config={configWithOneHelpSpace}
        host="https://client.helpin.ai"
        widgetKey="wk_123"
        onSelectSpace={() => {}}
        onSelectCollection={() => {}}
      />,
    );

    await waitFor(() => {
      expect(getByText('No published collections are available yet.')).toBeTruthy();
    });

    expect(queryByText('Developer Docs')).toBeNull();
  });

  it('does not repeat Help Center above the collections list', async () => {
    stubEmptyCollections();

    const { getByText, container } = render(
      <HelpView
        config={configWithOneHelpSpace}
        host="https://client.helpin.ai"
        widgetKey="wk_123"
        onSelectSpace={() => {}}
        onSelectCollection={() => {}}
      />,
    );

    await waitFor(() => {
      expect(getByText('No published collections are available yet.')).toBeTruthy();
    });

    expect(container.querySelector('.helpin-help-section-label')).toBeNull();
    expect(container.querySelectorAll('.helpin-help-title')).toHaveLength(1);
  });

  it('does not play the drill-in animation for the single-space root Help tab', async () => {
    stubEmptyCollections();

    const { getByText, container } = render(
      <HelpView
        config={configWithOneHelpSpace}
        host="https://client.helpin.ai"
        widgetKey="wk_123"
        onSelectSpace={() => {}}
        onSelectCollection={() => {}}
      />,
    );

    await waitFor(() => {
      expect(getByText('No published collections are available yet.')).toBeTruthy();
    });

    expect(container.querySelector('.helpin-help-inline-section .helpin-help-drilldown-view')).toBeNull();
  });

  it('searches help center articles and opens a result', async () => {
    const fetchMock = stubCollectionsAndSearch();
    const onSelectArticle = vi.fn();

    const { getByPlaceholderText, getByText } = render(
      <HelpView
        config={configWithOneHelpSpace}
        host="https://client.helpin.ai"
        widgetKey="wk_123"
        onSelectSpace={() => {}}
        onSelectCollection={() => {}}
        onSelectArticle={onSelectArticle}
      />,
    );

    fireEvent.input(getByPlaceholderText('Search help articles...'), {
      target: { value: 'reset' },
    });

    await waitFor(() => {
      expect(getByText('Reset password')).toBeTruthy();
    });

    expect(fetchMock).toHaveBeenCalledWith(
      'https://client.helpin.ai/widget/support/help/search?q=reset&limit=8&widget_key=wk_123',
    );

    fireEvent.click(getByText('Reset password'));

    expect(onSelectArticle).toHaveBeenCalledWith('reset-password-abc12345');
  });

  it('attributes the search to the visitor when an anonymous id is known', async () => {
    const fetchMock = stubCollectionsAndSearch();

    const { getByPlaceholderText, getByText } = render(
      <HelpView
        config={configWithOneHelpSpace}
        host="https://client.helpin.ai"
        widgetKey="wk_123"
        anonymousId="anon-visitor-1"
        onSelectSpace={() => {}}
        onSelectCollection={() => {}}
      />,
    );

    fireEvent.input(getByPlaceholderText('Search help articles...'), {
      target: { value: 'reset' },
    });

    await waitFor(() => {
      expect(getByText('Reset password')).toBeTruthy();
    });

    expect(fetchMock).toHaveBeenCalledWith(
      'https://client.helpin.ai/widget/support/help/search?q=reset&limit=8&anonymous_id=anon-visitor-1&widget_key=wk_123',
    );
  });

  it('uses one right-side search action slot for loading and clear', async () => {
    const pendingSearch = deferredResponse();
    const fetchMock = vi.fn(async (input: RequestInfo | URL) => {
      const url = String(input);
      if (url.includes('/widget/support/help/search')) {
        return pendingSearch.promise;
      }
      return new Response(JSON.stringify([]), { status: 200 });
    });
    vi.stubGlobal('fetch', fetchMock);

    const { container, getByLabelText, getByPlaceholderText, queryByLabelText } = render(
      <HelpView
        config={configWithOneHelpSpace}
        host="https://client.helpin.ai"
        widgetKey="wk_123"
        onSelectSpace={() => {}}
        onSelectCollection={() => {}}
      />,
    );

    fireEvent.input(getByPlaceholderText('Search help articles...'), {
      target: { value: 'reset' },
    });

    await waitFor(() => {
      expect(getByLabelText('Searching articles')).toBeTruthy();
    });
    expect(queryByLabelText('Clear search')).toBeNull();
    expect(container.querySelectorAll('.helpin-help-search-action')).toHaveLength(1);

    pendingSearch.resolve(new Response(JSON.stringify([]), { status: 200 }));

    await waitFor(() => {
      expect(getByLabelText('Clear search')).toBeTruthy();
    });
    expect(queryByLabelText('Searching articles')).toBeNull();
    expect(container.querySelectorAll('.helpin-help-search-action')).toHaveLength(1);

    fireEvent.click(getByLabelText('Clear search'));

    expect((getByPlaceholderText('Search help articles...') as HTMLInputElement).value).toBe('');
  });

  it('debounces help article search requests while typing', async () => {
    vi.useFakeTimers();
    const fetchMock = vi.fn(async (input: RequestInfo | URL) => {
      const url = String(input);
      if (url.includes('/widget/support/help/search')) {
        return new Response(JSON.stringify([]), { status: 200 });
      }
      return new Response(JSON.stringify([]), { status: 200 });
    });
    vi.stubGlobal('fetch', fetchMock);

    const { getByPlaceholderText } = render(
      <HelpView
        config={configWithOneHelpSpace}
        host="https://client.helpin.ai"
        widgetKey="wk_123"
        onSelectSpace={() => {}}
        onSelectCollection={() => {}}
      />,
    );

    const input = getByPlaceholderText('Search help articles...');
    fireEvent.input(input, { target: { value: 'an' } });
    fireEvent.input(input, { target: { value: 'ana' } });
    fireEvent.input(input, { target: { value: 'analytics' } });

    await act(async () => {
      vi.advanceTimersByTime(249);
    });

    expect(fetchMock).not.toHaveBeenCalledWith(
      'https://client.helpin.ai/widget/support/help/search?q=analytics&limit=8&widget_key=wk_123',
    );

    await act(async () => {
      vi.advanceTimersByTime(1);
    });

    await waitFor(() => {
      expect(fetchMock).toHaveBeenCalledWith(
        'https://client.helpin.ai/widget/support/help/search?q=analytics&limit=8&widget_key=wk_123',
      );
    });
    expect(
      fetchMock.mock.calls.filter(([input]) => String(input).includes('/widget/support/help/search')),
    ).toHaveLength(1);
  });

  it('emits a coverage-qualified search only after two seconds idle', async () => {
	vi.useFakeTimers();
	const fetchMock = vi.fn(async () => new Response(JSON.stringify([]), { status: 200 }));
	vi.stubGlobal('fetch', fetchMock);
	const { getByPlaceholderText } = render(
	  <HelpView
		config={configWithOneHelpSpace}
		host="https://client.helpin.ai"
		widgetKey="wk_123"
		anonymousId="anon-1"
		onSelectSpace={() => {}}
		onSelectCollection={() => {}}
	  />,
	);
	fireEvent.input(getByPlaceholderText('Search help articles...'), { target: { value: 'reset account password' } });
	await act(async () => { vi.advanceTimersByTime(1999); });
	expect(fetchMock.mock.calls.some(([input]) => String(input).includes('coverage_signal=1'))).toBe(false);
	await act(async () => { vi.advanceTimersByTime(1); await Promise.resolve(); });
	expect(fetchMock.mock.calls.some(([input]) => String(input).includes('coverage_signal=1'))).toBe(true);
  });
});
