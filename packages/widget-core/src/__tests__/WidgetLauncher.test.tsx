import { describe, it, expect } from 'vitest';
import { render } from '@testing-library/preact';
import { WidgetLauncher } from '../components/WidgetLauncher';

describe('WidgetLauncher', () => {
  it('renders closed state with icon', () => {
    const { container } = render(<WidgetLauncher onClick={() => {}} isOpen={false} />);
    expect(container.querySelector('.helpin-launcher')).toBeTruthy();
  });

  it('renders open state', () => {
    const { container } = render(<WidgetLauncher onClick={() => {}} isOpen={true} />);
    expect(container.querySelector('.helpin-launcher--open')).toBeTruthy();
  });

  it('displays unread badge when count > 0', () => {
    const { container } = render(
      <WidgetLauncher onClick={() => {}} isOpen={false} unreadCount={5} />
    );
    expect(container.querySelector('.helpin-unread-badge')).toBeTruthy();
    expect(container.textContent).toContain('5');
  });

  it('shows 9+ for count > 9', () => {
    const { container } = render(
      <WidgetLauncher onClick={() => {}} isOpen={false} unreadCount={15} />
    );
    expect(container.textContent).toContain('9+');
  });

  it('does not show badge when count is 0', () => {
    const { container } = render(
      <WidgetLauncher onClick={() => {}} isOpen={false} unreadCount={0} />
    );
    expect(container.querySelector('.helpin-unread-badge')).toBeFalsy();
  });

  it('applies custom brand color', () => {
    const { container } = render(
      <WidgetLauncher onClick={() => {}} isOpen={false} brandColor="#ff0000" />
    );
    const launcher = container.querySelector('.helpin-launcher') as HTMLElement;
    expect(launcher.style.backgroundColor).toBe('rgb(255, 0, 0)');
  });

  it('calls onClick handler', () => {
    const handleClick = vi.fn();
    const { container } = render(<WidgetLauncher onClick={handleClick} isOpen={false} />);
    container.querySelector('.helpin-launcher')?.dispatchEvent(new MouseEvent('click'));
    expect(handleClick).toHaveBeenCalled();
  });
});
