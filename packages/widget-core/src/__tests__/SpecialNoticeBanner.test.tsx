import { render, screen, cleanup, fireEvent } from '@testing-library/preact';
import { afterEach, beforeEach, describe, expect, it } from 'vitest';
import { SpecialNoticeBanner } from '../components/SpecialNoticeBanner';

describe('SpecialNoticeBanner', () => {
  beforeEach(() => {
    window.localStorage.clear();
  });
  afterEach(() => cleanup());

  it('renders nothing when text is empty', () => {
    const { container } = render(<SpecialNoticeBanner text="" workspaceId="ws-1" />);
    expect(container.firstChild).toBeNull();
  });

  it('renders nothing when text is only whitespace', () => {
    const { container } = render(<SpecialNoticeBanner text="   " workspaceId="ws-1" />);
    expect(container.firstChild).toBeNull();
  });

  it('renders the notice when text is present', () => {
    render(<SpecialNoticeBanner text="Backlog today." workspaceId="ws-1" />);
    expect(screen.getByText('Backlog today.')).toBeDefined();
    expect(screen.getByRole('status').textContent).toContain('Backlog today.');
  });

  it('dismisses on click and persists to localStorage', () => {
    const { container } = render(<SpecialNoticeBanner text="Backlog today." workspaceId="ws-1" />);
    const btn = screen.getByRole('button', { name: /dismiss/i });
    fireEvent.click(btn);
    expect(container.firstChild).toBeNull();
    // Find the persisted key — prefix is deterministic, hash is text-dependent.
    const keys = Object.keys(window.localStorage).filter((k) => k.startsWith('helpin:special-notice:ws-1:'));
    expect(keys.length).toBe(1);
    expect(window.localStorage.getItem(keys[0])).toBe('1');
  });

  it('respects a prior dismissal from storage on re-render', () => {
    // Simulate a prior dismissal — find the key our hash generates.
    render(<SpecialNoticeBanner text="Backlog today." workspaceId="ws-1" />);
    fireEvent.click(screen.getByRole('button', { name: /dismiss/i }));
    cleanup();

    const { container } = render(<SpecialNoticeBanner text="Backlog today." workspaceId="ws-1" />);
    expect(container.firstChild).toBeNull();
  });

  it('re-appears when the notice text changes after a prior dismissal', () => {
    render(<SpecialNoticeBanner text="Old notice." workspaceId="ws-1" />);
    fireEvent.click(screen.getByRole('button', { name: /dismiss/i }));
    cleanup();

    render(<SpecialNoticeBanner text="New notice." workspaceId="ws-1" />);
    expect(screen.getByText('New notice.')).toBeDefined();
  });
});
