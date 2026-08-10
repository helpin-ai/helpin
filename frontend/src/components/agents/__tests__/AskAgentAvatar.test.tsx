import { renderToStaticMarkup } from 'react-dom/server';
import { describe, expect, it } from 'vitest';

import { AskAgentAvatar } from '@/components/agents/AskAgentAvatar';

describe('AskAgentAvatar', () => {
  it('renders the product mark and requested state', () => {
    const markup = renderToStaticMarkup(
      <AskAgentAvatar state="speaking" decorative={false} size={28} />,
    );

    expect(markup).toContain('class="ask-agent-avatar');
    expect(markup).toContain('data-state="speaking"');
    expect(markup).toContain('data-plate-style="feather"');
    expect(markup).toContain('aria-label="Ask Agent is replying"');
    expect(markup).toContain('width:28px');
    expect(markup).toContain('viewBox="-2.5 -2.5 105 105"');
    expect(markup).toContain('feGaussianBlur');
    expect(markup).toContain('ask-agent-gradient-');
    expect(markup).toContain('ask-agent-body');
  });

  it('can render without a plate and remains decorative by default', () => {
    const markup = renderToStaticMarkup(<AskAgentAvatar plate={false} />);

    expect(markup).toContain('data-plate-style="none"');
    expect(markup).toContain('aria-hidden="true"');
    expect(markup).not.toContain('ask-agent-plate');
  });
});
