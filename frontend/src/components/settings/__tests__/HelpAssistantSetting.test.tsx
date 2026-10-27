// @vitest-environment jsdom
import { act } from 'react';
import { createRoot, type Root } from 'react-dom/client';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

const mocks = vi.hoisted(() => ({ settings: vi.fn(), mutate: vi.fn() }));

vi.mock('@/hooks/queries/useSupport', () => ({
  useChatSettings: () => mocks.settings(),
  useUpdateCustomerPortalSettings: () => ({ mutate: mocks.mutate, isPending: false }),
}));

import { HelpAssistantSetting } from '../HelpAssistantSetting';

Object.assign(globalThis, { IS_REACT_ACT_ENVIRONMENT: true });

let root: Root;
let container: HTMLDivElement;
beforeEach(() => {
  container = document.createElement('div');
  document.body.append(container);
  root = createRoot(container);
});
afterEach(() => {
  act(() => root.unmount());
  container.remove();
  vi.resetAllMocks();
});

describe('HelpAssistantSetting', () => {
  it('turns the shared help assistant setting on', async () => {
    mocks.settings.mockReturnValue({ data: { settings: { help_assistant_enabled: false } } });
    await act(async () => root.render(<HelpAssistantSetting workspaceId="ws" editable />));
    const toggle = container.querySelector<HTMLButtonElement>('[aria-label="Ask AI help assistant"]');
    expect(toggle?.getAttribute('aria-checked')).toBe('false');
    await act(async () => toggle?.click());
    expect(mocks.mutate).toHaveBeenCalledWith({ help_assistant_enabled: true });
  });

  it('cannot be changed without permission', async () => {
    mocks.settings.mockReturnValue({ data: { settings: { help_assistant_enabled: true } } });
    await act(async () => root.render(<HelpAssistantSetting workspaceId="ws" editable={false} />));
    const toggle = container.querySelector<HTMLButtonElement>('[aria-label="Ask AI help assistant"]');
    expect(toggle?.getAttribute('aria-checked')).toBe('true');
    expect(toggle?.disabled).toBe(true);
  });
});
