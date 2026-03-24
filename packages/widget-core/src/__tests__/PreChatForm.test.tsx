import { describe, it, expect, vi } from 'vitest';
import { render, fireEvent, waitFor } from '@testing-library/preact';
import { PreChatForm } from '../components/PreChatForm';
import type { WidgetConfig } from '../types';

const mockConfig: WidgetConfig = {
  workspaceId: 'ws-1',
  workspaceName: 'Test Support',
  branding: {
    primaryColor: '#6366f1',
    welcomeMessage: 'Hi there',
    widgetPosition: 'bottom-right',
    showBranding: true,
  },
  features: {
    aiEnabled: false,
    showTalkToHuman: false,
    fileUploads: false,
    preChatForm: true,
    requirePhone: true,
    csatRating: false,
    forceIdentify: false,
  },
  availability: {
    isOnline: true,
    statusText: 'Online now',
    replyTimeText: 'We typically reply in a few minutes',
  },
};

describe('PreChatForm', () => {
  it('renders inline form with email input', () => {
    const { container } = render(<PreChatForm config={mockConfig} onSubmit={() => {}} />);
    expect(container.querySelector('.helpin-inline-prechat')).toBeTruthy();
    expect(container.querySelector('input[type="email"]')).toBeTruthy();
  });

  it('shows prompt text for email', () => {
    const { container } = render(<PreChatForm config={mockConfig} onSubmit={() => {}} />);
    expect(container.textContent).toContain('Please enter your email address');
  });

  it('shows workspace name as agent name', () => {
    const { container } = render(<PreChatForm config={mockConfig} onSubmit={() => {}} />);
    expect(container.textContent).toContain('Test Support');
  });

  it('advances to phone step after email submit', async () => {
    const { container } = render(<PreChatForm config={mockConfig} onSubmit={() => {}} />);

    const emailInput = container.querySelector('input[type="email"]') as HTMLInputElement;
    fireEvent.input(emailInput, { target: { value: 'test@example.com' } });

    const form = container.querySelector('form') as HTMLFormElement;
    fireEvent.submit(form);

    await waitFor(() => {
      const phoneInput = container.querySelector('input[type="tel"]');
      expect(phoneInput).toBeTruthy();
      expect(container.textContent).toContain("What's your phone number");
    });
  });

  it('calls onSubmit with email and phone after both steps', async () => {
    const onSubmit = vi.fn();
    const { container } = render(<PreChatForm config={mockConfig} onSubmit={onSubmit} />);

    // Submit email
    const emailInput = container.querySelector('input[type="email"]') as HTMLInputElement;
    fireEvent.input(emailInput, { target: { value: 'test@example.com' } });
    fireEvent.submit(container.querySelector('form') as HTMLFormElement);

    // Submit phone
    await waitFor(() => {
      expect(container.querySelector('input[type="tel"]')).toBeTruthy();
    });
    const phoneInput = container.querySelector('input[type="tel"]') as HTMLInputElement;
    fireEvent.input(phoneInput, { target: { value: '+1234567890' } });
    fireEvent.submit(container.querySelector('form') as HTMLFormElement);

    await waitFor(() => {
      expect(onSubmit).toHaveBeenCalledWith({
        phone: '+1234567890',
        email: 'test@example.com',
      });
    });
  });

  it('skips phone step when requirePhone is false', async () => {
    const onSubmit = vi.fn();
    const emailOnlyConfig: WidgetConfig = {
      ...mockConfig,
      features: { ...mockConfig.features, requirePhone: false },
    };
    const { container } = render(<PreChatForm config={emailOnlyConfig} onSubmit={onSubmit} />);

    const emailInput = container.querySelector('input[type="email"]') as HTMLInputElement;
    fireEvent.input(emailInput, { target: { value: 'test@example.com' } });
    fireEvent.submit(container.querySelector('form') as HTMLFormElement);

    await waitFor(() => {
      expect(onSubmit).toHaveBeenCalledWith({
        phone: '',
        email: 'test@example.com',
      });
    });
  });

  it('removes form after submission', async () => {
    const onSubmit = vi.fn();
    const { container } = render(<PreChatForm config={mockConfig} onSubmit={onSubmit} />);

    // Submit email
    const emailInput = container.querySelector('input[type="email"]') as HTMLInputElement;
    fireEvent.input(emailInput, { target: { value: 'test@example.com' } });
    fireEvent.submit(container.querySelector('form') as HTMLFormElement);

    // Submit phone (skip)
    await waitFor(() => {
      expect(container.querySelector('input[type="tel"]')).toBeTruthy();
    });
    fireEvent.submit(container.querySelector('form') as HTMLFormElement);

    await waitFor(() => {
      expect(container.querySelector('.helpin-inline-prechat')).toBeFalsy();
    });
  });
});
