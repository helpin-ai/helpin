// @vitest-environment jsdom
import { act } from 'react';
import { createRoot } from 'react-dom/client';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { showEntityCreatedToast } from '../entity-created-toast';

const mocks = vi.hoisted(() => ({ success: vi.fn(), custom: vi.fn() }));
vi.mock('sonner', () => ({ toast: mocks }));
(globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true;

beforeEach(() => {
  mocks.success.mockReset().mockReturnValue('toast-1');
  mocks.custom.mockReset();
});

describe('showEntityCreatedToast', () => {
  it('uses the shared success styling and preserves open and copy actions', async () => {
    const onOpen = vi.fn();
    const writeText = vi.fn().mockResolvedValue(undefined);
    Object.defineProperty(navigator, 'clipboard', { configurable: true, value: { writeText } });

    expect(showEntityCreatedToast({
      entityLabel: 'Task',
      title: 'Review onboarding',
      identifier: { label: 'Task ID', value: 'HLP-42' },
      subtitle: 'Recurring schedule added.',
      onOpen,
    })).toBe('toast-1');

    expect(mocks.custom).not.toHaveBeenCalled();
    expect(mocks.success).toHaveBeenCalledWith('Task created', expect.objectContaining({
      action: { label: 'Open', onClick: onOpen },
      duration: 6000,
    }));
    const options = mocks.success.mock.calls[0][1];
    expect(options.unstyled).toBeUndefined();

    const container = document.createElement('div');
    const root = createRoot(container);
    await act(async () => root.render(options.description));
    expect(container.textContent).toContain('Review onboarding');
    expect(container.textContent).toContain('Recurring schedule added.');
    await act(async () => container.querySelector<HTMLButtonElement>('[aria-label="Copy Task ID HLP-42"]')?.click());
    expect(writeText).toHaveBeenCalledWith('HLP-42');
    expect(container.querySelector('[aria-label="Task ID copied"]')).not.toBeNull();
    options.action.onClick();
    expect(onOpen).toHaveBeenCalledOnce();
    await act(async () => root.unmount());
  });

  it('keeps the duplicated-task label and omits unavailable actions', () => {
    showEntityCreatedToast({ entityLabel: 'Task', title: 'Copy of review', eyebrow: 'Task duplicated', openLabel: 'Open duplicate' });
    expect(mocks.success).toHaveBeenCalledWith('Task duplicated', expect.objectContaining({ action: undefined }));

    const onOpen = vi.fn();
    showEntityCreatedToast({ entityLabel: 'Task', title: 'Copy of review', eyebrow: 'Task duplicated', openLabel: 'Open duplicate', onOpen });
    expect(mocks.success).toHaveBeenLastCalledWith('Task duplicated', expect.objectContaining({
      action: { label: 'Open duplicate', onClick: onOpen },
    }));
  });
});
