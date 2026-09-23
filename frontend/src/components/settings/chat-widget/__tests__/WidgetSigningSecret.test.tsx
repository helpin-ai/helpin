// @vitest-environment jsdom
import React, { act } from 'react';
import { createRoot, type Root } from 'react-dom/client';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { TooltipProvider } from '@/components/ui/tooltip';
import { WidgetSigningSecret } from '../WidgetSigningSecret';

const mocks = vi.hoisted(() => ({ reveal: vi.fn(), rotate: vi.fn() }));
vi.mock('@/hooks/queries/useSupport', () => ({
  useRevealWidgetSigningSecret: () => ({ mutateAsync: mocks.reveal, isPending: false }),
  useRotateWidgetSigningSecret: () => ({ mutateAsync: mocks.rotate, isPending: false }),
}));
vi.mock('sonner', () => ({ toast: { success: vi.fn(), error: vi.fn() } }));
(globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true;

let container: HTMLDivElement;
let root: Root;

beforeEach(() => {
  container = document.createElement('div');
  document.body.append(container);
  root = createRoot(container);
});
afterEach(async () => {
  await act(async () => root.unmount());
  container.remove();
  vi.clearAllMocks();
});

async function render(props: Partial<React.ComponentProps<typeof WidgetSigningSecret>> = {}) {
  await act(async () => root.render(
    <TooltipProvider>
      <WidgetSigningSecret workspaceId="ws" configured canManage enforced {...props} />
    </TooltipProvider>,
  ));
}

const button = (label: string) =>
  Array.from(document.body.querySelectorAll('button')).find(candidate =>
    candidate.getAttribute('aria-label') === label || candidate.textContent === label);

describe('widget signing secret', () => {
  it('explains that only admins can manage the secret and never fetches it for others', async () => {
    await render({ canManage: false });
    expect(container.textContent).toContain('Only workspace admins can view or regenerate the signing secret.');
    expect(container.querySelector('input')).toBeNull();
    expect(mocks.reveal).not.toHaveBeenCalled();
  });

  it('masks the secret until an admin reveals it', async () => {
    mocks.reveal.mockResolvedValue({ secret_key: 'sk_live_secret' });
    await render();
    const input = container.querySelector('input')!;
    expect(input.value).not.toContain('sk_live_secret');
    expect(mocks.reveal).not.toHaveBeenCalled();

    await act(async () => button('Reveal signing secret')!.click());
    expect(mocks.reveal).toHaveBeenCalledTimes(1);
    expect(input.value).toBe('sk_live_secret');

    await act(async () => button('Hide signing secret')!.click());
    expect(input.value).not.toContain('sk_live_secret');
    await act(async () => button('Reveal signing secret')!.click());
    expect(mocks.reveal).toHaveBeenCalledTimes(1);
  });

  it('copies the secret without revealing it on screen', async () => {
    mocks.reveal.mockResolvedValue({ secret_key: 'sk_live_secret' });
    const writeText = vi.fn().mockResolvedValue(undefined);
    Object.defineProperty(navigator, 'clipboard', { value: { writeText }, configurable: true });
    await render();
    await act(async () => button('Copy signing secret')!.click());
    expect(writeText).toHaveBeenCalledWith('sk_live_secret');
    expect(container.querySelector('input')!.value).not.toContain('sk_live_secret');
  });

  it('requires confirmation before regenerating and then shows the new secret', async () => {
    mocks.rotate.mockResolvedValue({ secret_key: 'sk_live_rotated' });
    await render();
    await act(async () => button('Regenerate secret')!.click());
    expect(mocks.rotate).not.toHaveBeenCalled();
    const dialog = document.body.querySelector('[role="alertdialog"]')!;
    expect(dialog.textContent).toContain('The current secret stops working immediately');
    expect(dialog.textContent).toContain('can only chat anonymously');

    const confirm = Array.from(dialog.querySelectorAll('button')).find(candidate => candidate.textContent === 'Regenerate secret')!;
    await act(async () => confirm.click());
    expect(mocks.rotate).toHaveBeenCalledTimes(1);
    expect(container.querySelector('input')!.value).toBe('sk_live_rotated');
  });

  it('keeps the current secret when regeneration is cancelled', async () => {
    await render({ enforced: false });
    await act(async () => button('Regenerate secret')!.click());
    const dialog = document.body.querySelector('[role="alertdialog"]')!;
    expect(dialog.textContent).toContain('treated as unverified');
    const cancel = Array.from(dialog.querySelectorAll('button')).find(candidate => candidate.textContent === 'Cancel')!;
    await act(async () => cancel.click());
    expect(mocks.rotate).not.toHaveBeenCalled();
  });
});
