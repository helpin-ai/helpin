import { render, waitFor } from '@testing-library/preact';
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

describe('HelpView', () => {
  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it('uses Help Center as the tab heading', () => {
    stubEmptyCollections();

    const { container } = render(
      <HelpView
        config={configWithOneHelpSpace}
        host="https://client.helpin.ai"
        widgetKey="wk_123"
        onContact={() => {}}
        onSelectSpace={() => {}}
        onSelectCollection={() => {}}
      />,
    );

    expect(container.querySelector('.helpin-help-header .helpin-help-title')?.textContent).toBe('Help Center');
  });

  it('keeps Contact us below help center content', async () => {
    stubEmptyCollections();

    const { container, getByText } = render(
      <HelpView
        config={configWithOneHelpSpace}
        host="https://client.helpin.ai"
        widgetKey="wk_123"
        onContact={() => {}}
        onSelectSpace={() => {}}
        onSelectCollection={() => {}}
      />,
    );

    await waitFor(() => {
      expect(getByText('No published collections are available yet.')).toBeTruthy();
    });

    const helpContent = container.querySelector('.helpin-help-inline-section');
    const contactUs = getByText('Contact us').closest('.helpin-help-links');

    expect(helpContent).toBeTruthy();
    expect(contactUs).toBeTruthy();
    expect(Boolean(helpContent?.compareDocumentPosition(contactUs!) & Node.DOCUMENT_POSITION_FOLLOWING)).toBe(true);
  });

  it('does not show the docs space name when only one help space is configured', async () => {
    stubEmptyCollections();

    const { getByText, queryByText } = render(
      <HelpView
        config={configWithOneHelpSpace}
        host="https://client.helpin.ai"
        widgetKey="wk_123"
        onContact={() => {}}
        onSelectSpace={() => {}}
        onSelectCollection={() => {}}
      />,
    );

    await waitFor(() => {
      expect(getByText('No published collections are available yet.')).toBeTruthy();
    });

    expect(queryByText('Developer Docs')).toBeNull();
  });
});
