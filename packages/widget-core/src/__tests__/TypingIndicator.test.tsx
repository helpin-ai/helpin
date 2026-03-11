import { describe, it, expect } from 'vitest';
import { render } from '@testing-library/preact';
import { TypingIndicator } from '../components/TypingIndicator';

describe('TypingIndicator', () => {
  it('renders default typing text', () => {
    const { container } = render(<TypingIndicator />);
    expect(container.textContent).toContain('is typing...');
  });

  it('renders custom label', () => {
    const { container } = render(<TypingIndicator label="Agent is typing..." />);
    expect(container.textContent).toContain('Agent is typing...');
  });

  it('has typing dots', () => {
    const { container } = render(<TypingIndicator />);
    const dots = container.querySelector('.helpin-typing-dots');
    expect(dots).toBeTruthy();
  });

  it('has three bouncing dots', () => {
    const { container } = render(<TypingIndicator />);
    const dots = container.querySelectorAll('.helpin-typing-dots span');
    expect(dots.length).toBe(3);
  });

  it('renders with custom label and has correct structure', () => {
    const { container } = render(<TypingIndicator label="Support" />);
    expect(container.querySelector('.helpin-typing-indicator')).toBeTruthy();
    expect(container.querySelector('.helpin-typing-label')).toBeTruthy();
  });
});
