import { describe, it, expect, vi } from 'vitest';
import { render, fireEvent } from '@testing-library/preact';
import { ComposeBar } from '../components/ComposeBar';

describe('ComposeBar', () => {
  it('renders textarea with placeholder', () => {
    const { container } = render(
      <ComposeBar onSend={() => {}} placeholder="Type here..." />
    );
    const textarea = container.querySelector('.helpin-compose-input') as HTMLTextAreaElement;
    expect(textarea.placeholder).toBe('Type here...');
  });

  it('uses default placeholder when not provided', () => {
    const { container } = render(<ComposeBar onSend={() => {}} />);
    const textarea = container.querySelector('.helpin-compose-input') as HTMLTextAreaElement;
    expect(textarea.placeholder).toBe('Ask a question...');
  });

  it('is disabled when disabled prop is true', () => {
    const { container } = render(<ComposeBar onSend={() => {}} disabled={true} />);
    const textarea = container.querySelector('.helpin-compose-input') as HTMLTextAreaElement;
    expect(textarea.disabled).toBe(true);
  });

  it('calls onSend when form is submitted', () => {
    const onSend = vi.fn();
    const { container } = render(<ComposeBar onSend={onSend} />);
    
    const input = container.querySelector('.helpin-compose-input') as HTMLTextAreaElement;
    fireEvent.input(input, { target: { value: 'Hello world' } });
    
    const form = container.querySelector('.helpin-compose-bar') as HTMLFormElement;
    fireEvent.submit(form);
    
    expect(onSend).toHaveBeenCalledWith('Hello world', undefined);
  });

  it('does not send empty messages', () => {
    const onSend = vi.fn();
    const { container } = render(<ComposeBar onSend={onSend} />);
    
    const input = container.querySelector('.helpin-compose-input') as HTMLTextAreaElement;
    fireEvent.input(input, { target: { value: '   ' } });
    
    const form = container.querySelector('.helpin-compose-bar') as HTMLFormElement;
    fireEvent.submit(form);
    
    expect(onSend).not.toHaveBeenCalled();
  });

  it('clears input after sending', async () => {
    const onSend = vi.fn();
    const { container } = render(<ComposeBar onSend={onSend} />);
    
    const input = container.querySelector('.helpin-compose-input') as HTMLTextAreaElement;
    fireEvent.input(input, { target: { value: 'Test message' } });
    
    const form = container.querySelector('.helpin-compose-bar') as HTMLFormElement;
    fireEvent.submit(form);
    
    expect(input.value).toBe('');
  });

  it('has send button', () => {
    const { container } = render(<ComposeBar onSend={() => {}} />);
    const sendButton = container.querySelector('.helpin-compose-send');
    expect(sendButton).toBeTruthy();
  });

  it('send button is disabled when input is empty', () => {
    const { container } = render(<ComposeBar onSend={() => {}} />);
    const sendButton = container.querySelector('.helpin-compose-send') as HTMLButtonElement;
    expect(sendButton.disabled).toBe(true);
  });

  it('send button is enabled when input has content', () => {
    const onSend = vi.fn();
    const { container } = render(<ComposeBar onSend={onSend} />);
    
    const input = container.querySelector('.helpin-compose-input') as HTMLTextAreaElement;
    fireEvent.input(input, { target: { value: 'Hello' } });
    
    const sendButton = container.querySelector('.helpin-compose-send') as HTMLButtonElement;
    expect(sendButton.disabled).toBe(false);
  });

  it('handles Enter key to send', () => {
    const onSend = vi.fn();
    const { container } = render(<ComposeBar onSend={onSend} />);
    
    const input = container.querySelector('.helpin-compose-input') as HTMLTextAreaElement;
    fireEvent.input(input, { target: { value: 'Enter message' } });
    
    fireEvent.keyDown(input, { key: 'Enter', bubbles: true });
    
    expect(onSend).toHaveBeenCalledWith('Enter message', undefined);
  });

  it('does not send on Shift+Enter', () => {
    const onSend = vi.fn();
    const { container } = render(<ComposeBar onSend={onSend} />);
    
    const input = container.querySelector('.helpin-compose-input') as HTMLTextAreaElement;
    fireEvent.input(input, { target: { value: 'Multiline' } });
    
    fireEvent.keyDown(input, { key: 'Enter', shiftKey: true, bubbles: true });
    
    expect(onSend).not.toHaveBeenCalled();
  });

  it('renders correctly when disabled', () => {
    const onSend = vi.fn();
    const { container } = render(<ComposeBar onSend={onSend} disabled={true} />);
    
    const input = container.querySelector('.helpin-compose-input') as HTMLTextAreaElement;
    const sendButton = container.querySelector('.helpin-compose-send') as HTMLButtonElement;
    
    expect(input.disabled).toBe(true);
    expect(sendButton.disabled).toBe(true);
  });

  it('renders the whole composer attribution as one shared alignment link', () => {
    const { container } = render(<ComposeBar onSend={() => {}} />);

    const attribution = container.querySelector('a.helpin-compose-footer.helpin-brand-attribution');

    expect(attribution).not.toBeNull();
    expect(attribution?.children).toHaveLength(2);
    expect(attribution?.children[0]?.classList.contains('helpin-brand-attribution-label')).toBe(true);
    expect(attribution?.children[0]?.textContent).toBe('We run on');
    expect(attribution?.children[1]?.classList.contains('helpin-brand-attribution-brand')).toBe(true);
    expect(attribution?.querySelector('.helpin-brand-attribution-name')?.textContent).toBe('Helpin');
  });

  it('inserts an emoji selected from the picker into the textarea', async () => {
    const { container, getByLabelText, findByPlaceholderText } = render(<ComposeBar onSend={() => {}} />);

    fireEvent.click(getByLabelText('Open emoji picker'));
    const searchInput = await findByPlaceholderText('Search emojis...');
    fireEvent.input(searchInput, { target: { value: 'heart' } });
    fireEvent.click(getByLabelText('Insert ❤️'));

    const input = container.querySelector('.helpin-compose-input') as HTMLTextAreaElement;
    expect(input.value).toBe('❤️');
  });
});
