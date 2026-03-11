import { describe, it, expect, vi } from 'vitest';
import { render, fireEvent, waitFor } from '@testing-library/preact';
import { PreChatForm } from '../components/PreChatForm';

describe('PreChatForm', () => {
  it('renders with default props', () => {
    const { container } = render(<PreChatForm onSubmit={() => {}} />);
    expect(container.querySelector('.helpin-pre-chat-form')).toBeTruthy();
  });

  it('renders welcome message', () => {
    const { container } = render(
      <PreChatForm onSubmit={() => {}} welcomeMessage="Welcome!" />
    );
    expect(container.textContent).toContain('Welcome!');
  });

  it('shows email input first', () => {
    const { container } = render(<PreChatForm onSubmit={() => {}} />);
    const emailInput = container.querySelector('input[type="email"]');
    expect(emailInput).toBeTruthy();
  });

  it('requires email when requireEmail is true', () => {
    const { container } = render(
      <PreChatForm onSubmit={() => {}} requireEmail={true} />
    );
    const emailInput = container.querySelector('input[type="email"]') as HTMLInputElement;
    expect(emailInput.required).toBe(true);
  });

  it('does not require email when requireEmail is false', () => {
    const { container } = render(
      <PreChatForm onSubmit={() => {}} requireEmail={false} />
    );
    const emailInput = container.querySelector('input[type="email"]') as HTMLInputElement;
    expect(emailInput.required).toBe(false);
  });

  it('advances to name step after email submit', async () => {
    const { container } = render(
      <PreChatForm onSubmit={() => {}} requireName={true} />
    );
    
    const emailInput = container.querySelector('input[type="email"]') as HTMLInputElement;
    fireEvent.input(emailInput, { target: { value: 'test@example.com' } });
    
    const form = container.querySelector('form') as HTMLFormElement;
    fireEvent.submit(form);
    
    await waitFor(() => {
      const nameInput = container.querySelector('input[type="text"]');
      expect(nameInput).toBeTruthy();
    });
  });

  it('skips name step when requireName is false', async () => {
    const onSubmit = vi.fn();
    const { container } = render(
      <PreChatForm onSubmit={onSubmit} requireName={false} />
    );
    
    const emailInput = container.querySelector('input[type="email"]') as HTMLInputElement;
    fireEvent.input(emailInput, { target: { value: 'test@example.com' } });
    
    const form = container.querySelector('form') as HTMLFormElement;
    fireEvent.submit(form);
    
    await waitFor(() => {
      expect(onSubmit).toHaveBeenCalledWith({
        name: '',
        email: 'test@example.com'
      });
    });
  });
});
