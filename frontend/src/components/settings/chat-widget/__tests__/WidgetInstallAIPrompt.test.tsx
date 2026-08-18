// @vitest-environment jsdom

import { act } from 'react';
import { createRoot, type Root } from 'react-dom/client';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { WidgetInstallAIPrompt } from '../WidgetInstallAIPrompt';

(globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true;

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

describe('WidgetInstallAIPrompt', () => {
  it('shows the generated prompt and sends copy actions to its handler', () => {
    const onCopy = vi.fn();

    act(() => {
      root.render(
        <WidgetInstallAIPrompt
          prompt="Install Helpin with widget-public-key"
          onCopy={onCopy}
        />,
      );
    });

    const prompt = container.querySelector<HTMLTextAreaElement>('textarea[aria-label="AI installation prompt"]');
    expect(prompt?.value).toBe('Install Helpin with widget-public-key');
    expect(container.querySelector('svg[data-icon="install-with-ai"]')).not.toBeNull();

    const copyButton = Array.from(container.querySelectorAll('button')).find((button) =>
      button.textContent?.includes('Copy prompt'),
    );
    act(() => copyButton?.click());
    expect(onCopy).toHaveBeenCalledOnce();
  });
});
