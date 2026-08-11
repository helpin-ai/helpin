// @vitest-environment jsdom
import { act } from 'react';
import type { ReactNode } from 'react';
import { createRoot, type Root } from 'react-dom/client';
import { afterEach, describe, expect, it, vi } from 'vitest';

import { EmailVerificationBanner } from '@/components/auth/EmailVerificationBanner';

const resendVerification = vi.fn();
const toastSuccess = vi.fn();
const toastError = vi.fn();

vi.mock('@/lib/services/authService', () => ({
  authService: {
    resendVerification: (...args: unknown[]) => resendVerification(...args),
  },
}));

vi.mock('sonner', () => ({
  toast: {
    success: (...args: unknown[]) => toastSuccess(...args),
    error: (...args: unknown[]) => toastError(...args),
  },
}));

globalThis.IS_REACT_ACT_ENVIRONMENT = true;

function render(ui: ReactNode) {
  const container = document.createElement('div');
  document.body.appendChild(container);
  let root: Root;
  act(() => {
    root = createRoot(container);
    root.render(ui);
  });
  return {
    container,
    unmount: () => {
      act(() => root.unmount());
      container.remove();
    },
  };
}

describe('EmailVerificationBanner', () => {
  afterEach(() => {
    vi.useRealTimers();
    vi.clearAllMocks();
    document.body.innerHTML = '';
  });

  it('renders nothing for verified users', () => {
    const { container, unmount } = render(<EmailVerificationBanner emailVerified />);

    expect(container.textContent).toBe('');

    unmount();
  });

  it('resends verification from the top notification bar', async () => {
    resendVerification.mockResolvedValue({ data: { message: 'sent' }, error: null });
    const { container, unmount } = render(<EmailVerificationBanner emailVerified={false} />);
    const button = Array.from(container.querySelectorAll('button')).find(
      (candidate) => candidate.textContent === 'Resend email',
    );

    expect(container.textContent).toContain('We sent a verification email');
    expect(container.textContent).toContain('Open it and click the verification link');
    expect(button).toBeTruthy();

    await act(async () => {
      button?.dispatchEvent(new MouseEvent('click', { bubbles: true }));
    });

    expect(resendVerification).toHaveBeenCalledTimes(1);
    expect(toastSuccess).toHaveBeenCalledWith('Verification email sent. Check your inbox.');

    unmount();
  });

  it('centers the notification message and action as one responsive group', () => {
    const { container, unmount } = render(<EmailVerificationBanner emailVerified={false} />);
    const layout = container.querySelector('.max-w-screen-2xl');
    const message = layout?.querySelector('.text-sm');

    expect(layout?.classList.contains('justify-center')).toBe(true);
    expect(layout?.classList.contains('flex-wrap')).toBe(true);
    expect(message?.classList.contains('text-center')).toBe(true);

    unmount();
  });

  it('prevents repeated resend clicks during the cooldown', async () => {
    vi.useFakeTimers();
    resendVerification.mockResolvedValue({ data: { message: 'sent' }, error: null });
    const { container, unmount } = render(<EmailVerificationBanner emailVerified={false} resendCooldownSeconds={3} />);

    const clickResend = async () => {
      const button = Array.from(container.querySelectorAll('button')).find(
        (candidate) => candidate.textContent?.includes('Resend'),
      );
      await act(async () => {
        button?.dispatchEvent(new MouseEvent('click', { bubbles: true }));
      });
    };

    await clickResend();
    await clickResend();

    expect(resendVerification).toHaveBeenCalledTimes(1);
    expect(container.textContent).toContain('Resend in 3s');

    await act(async () => {
      vi.advanceTimersByTime(3000);
    });
    await clickResend();

    expect(resendVerification).toHaveBeenCalledTimes(2);

    unmount();
  });
});
