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
    fileUploads: false,
    preChatForm: true,
    requireName: true,
    csatRating: false,
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

  it('advances to name step after email submit', async () => {
    const { container } = render(<PreChatForm config={mockConfig} onSubmit={() => {}} />);

    const emailInput = container.querySelector('input[type="email"]') as HTMLInputElement;
    fireEvent.input(emailInput, { target: { value: 'test@example.com' } });

    const form = container.querySelector('form') as HTMLFormElement;
    fireEvent.submit(form);

    await waitFor(() => {
      const nameInput = container.querySelector('input[type="text"]');
      expect(nameInput).toBeTruthy();
      expect(container.textContent).toContain("What's your name");
    });
  });

  it('calls onSubmit with email and name after both steps', async () => {
    const onSubmit = vi.fn();
    const { container } = render(<PreChatForm config={mockConfig} onSubmit={onSubmit} />);

    // Submit email
    const emailInput = container.querySelector('input[type="email"]') as HTMLInputElement;
    fireEvent.input(emailInput, { target: { value: 'test@example.com' } });
    fireEvent.submit(container.querySelector('form') as HTMLFormElement);

    // Submit name
    await waitFor(() => {
      expect(container.querySelector('input[type="text"]')).toBeTruthy();
    });
    const nameInput = container.querySelector('input[type="text"]') as HTMLInputElement;
    fireEvent.input(nameInput, { target: { value: 'John' } });
    fireEvent.submit(container.querySelector('form') as HTMLFormElement);

    await waitFor(() => {
      expect(onSubmit).toHaveBeenCalledWith({
        name: 'John',
        email: 'test@example.com',
      });
    });
  });

  it('skips name step when requireName is false', async () => {
    const onSubmit = vi.fn();
    const emailOnlyConfig: WidgetConfig = {
      ...mockConfig,
      features: { ...mockConfig.features, requireName: false },
    };
    const { container } = render(<PreChatForm config={emailOnlyConfig} onSubmit={onSubmit} />);

    const emailInput = container.querySelector('input[type="email"]') as HTMLInputElement;
    fireEvent.input(emailInput, { target: { value: 'test@example.com' } });
    fireEvent.submit(container.querySelector('form') as HTMLFormElement);

    await waitFor(() => {
      expect(onSubmit).toHaveBeenCalledWith({
        name: '',
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

    // Submit name (skip)
    await waitFor(() => {
      expect(container.querySelector('input[type="text"]')).toBeTruthy();
    });
    fireEvent.submit(container.querySelector('form') as HTMLFormElement);

    await waitFor(() => {
      expect(container.querySelector('.helpin-inline-prechat')).toBeFalsy();
    });
  });
});
