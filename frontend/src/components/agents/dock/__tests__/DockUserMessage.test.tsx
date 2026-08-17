// @vitest-environment jsdom
import { act } from 'react';
import { createRoot, type Root } from 'react-dom/client';
import { afterEach, beforeEach, describe, expect, it } from 'vitest';
import { DockUserMessage } from '../DockUserMessage';

(globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true;

describe('DockUserMessage', () => {
  let container: HTMLDivElement;
  let root: Root;

  beforeEach(() => {
    container = document.createElement('div');
    document.body.appendChild(container);
    root = createRoot(container);
  });

  afterEach(() => {
    act(() => root.unmount());
    container.remove();
  });

  it('renders pending messages with the same metadata header as persisted messages', () => {
    act(() => {
      root.render(
        <DockUserMessage
          content="Please investigate this issue."
          timestamp="2026-08-17T07:00:00Z"
          pending
        />,
      );
    });

    const header = container.querySelector('[data-dock-user-message-header]');
    const bubble = container.querySelector('[data-dock-user-bubble]');
    expect(header).not.toBeNull();
    expect(bubble).not.toBeNull();
    expect(header?.compareDocumentPosition(bubble!) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();
  });
});
