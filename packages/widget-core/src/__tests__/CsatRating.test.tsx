import { describe, expect, it, vi } from 'vitest';
import { fireEvent, render } from '@testing-library/preact';
import { CsatRating } from '../components/CsatRating';

describe('CsatRating', () => {
  it('collects a rating with optional feedback and confirms submission', async () => {
    const onSubmit = vi.fn();
    const rendered = render(<CsatRating onSubmit={onSubmit} />);

    fireEvent.click(rendered.getByRole('radio', { name: 'Satisfied' }));
    fireEvent.input(rendered.getByRole('textbox', { name: 'Additional feedback' }), {
      target: { value: 'Fast and clear' },
    });
    fireEvent.click(rendered.getByRole('button', { name: 'Send feedback' }));

    expect(onSubmit).toHaveBeenCalledWith(4, 'Fast and clear');
    expect(await rendered.findByText('Thank you for your feedback!')).toBeTruthy();
  });

  it('keeps the form and shows the error when saving fails', async () => {
    const onSubmit = vi.fn().mockRejectedValue(new Error('This conversation can no longer be rated.'));
    const rendered = render(<CsatRating onSubmit={onSubmit} />);

    fireEvent.click(rendered.getByRole('radio', { name: 'Unsatisfied' }));
    fireEvent.click(rendered.getByRole('button', { name: 'Send feedback' }));

    expect((await rendered.findByRole('alert')).textContent).toBe('This conversation can no longer be rated.');
    expect(rendered.queryByText('Thank you for your feedback!')).toBeNull();
    expect(rendered.getByRole('button', { name: 'Send feedback' })).toBeTruthy();
  });

  it('can be dismissed without submitting a rating', () => {
    const onSubmit = vi.fn();
    const rendered = render(<CsatRating onSubmit={onSubmit} />);

    fireEvent.click(rendered.getByRole('button', { name: 'Not now' }));

    expect(rendered.queryByText('How was your support experience?')).toBeNull();
    expect(onSubmit).not.toHaveBeenCalled();
  });
});
