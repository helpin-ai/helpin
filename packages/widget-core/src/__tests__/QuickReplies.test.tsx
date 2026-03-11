import { describe, it, expect, vi } from 'vitest';
import { render } from '@testing-library/preact';
import { QuickReplies } from '../components/QuickReplies';

describe('QuickReplies', () => {
  const replies = ['Talk to a person', 'I need help', 'Billing question'];

  it('renders all quick reply options', () => {
    const { container } = render(
      <QuickReplies replies={replies} onSelect={() => {}} />
    );
    
    const buttons = container.querySelectorAll('.helpin-quick-reply');
    expect(buttons.length).toBe(3);
  });

  it('displays correct reply text', () => {
    const { container } = render(
      <QuickReplies replies={replies} onSelect={() => {}} />
    );
    
    expect(container.textContent).toContain('Talk to a person');
    expect(container.textContent).toContain('I need help');
    expect(container.textContent).toContain('Billing question');
  });

  it('calls onSelect with correct reply when clicked', () => {
    const onSelect = vi.fn();
    const { container } = render(
      <QuickReplies replies={replies} onSelect={onSelect} />
    );
    
    const buttons = container.querySelectorAll('.helpin-quick-reply');
    (buttons[0] as HTMLButtonElement).click();
    
    expect(onSelect).toHaveBeenCalledWith('Talk to a person');
  });

  it('renders empty when no replies provided', () => {
    const { container } = render(
      <QuickReplies replies={[]} onSelect={() => {}} />
    );
    
    const buttons = container.querySelectorAll('.helpin-quick-reply');
    expect(buttons.length).toBe(0);
  });

  it('renders single reply', () => {
    const { container } = render(
      <QuickReplies replies={['Single option']} onSelect={() => {}} />
    );
    
    const buttons = container.querySelectorAll('.helpin-quick-reply');
    expect(buttons.length).toBe(1);
    expect(container.textContent).toContain('Single option');
  });
});
