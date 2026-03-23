import { describe, it, expect, vi } from 'vitest';
import { render, fireEvent } from '@testing-library/preact';
import { EmojiPicker } from '../components/EmojiPicker';

describe('EmojiPicker', () => {
  it('loads the emoji catalog on demand, uses unified codepoints, and shows a visible fallback on load error', async () => {
    const { getByLabelText, findByPlaceholderText } = render(
      <EmojiPicker onEmojiSelect={() => {}} />
    );

    fireEvent.click(getByLabelText('Open emoji picker'));
    const searchInput = await findByPlaceholderText('Search emojis...');
    fireEvent.input(searchInput, { target: { value: 'heart' } });

    const emojiButton = getByLabelText('Insert ❤️') as HTMLButtonElement;
    const emojiImage = emojiButton.querySelector('img') as HTMLImageElement;
    expect(emojiImage.src).toContain('/2764-fe0f.png');

    const fallback = emojiButton.querySelector('span') as HTMLSpanElement;
    expect(fallback.style.display).toBe('none');

    fireEvent.error(emojiImage);

    const updatedImage = emojiButton.querySelector('img');
    const updatedFallback = emojiButton.querySelector('span') as HTMLSpanElement;
    expect(updatedImage).toBeNull();
    expect(updatedFallback.style.display).toBe('inline');
    expect(updatedFallback.textContent).toBe('❤️');
  });

  it('returns the selected emoji to the caller', async () => {
    const onEmojiSelect = vi.fn();
    const { getByLabelText, findByPlaceholderText } = render(
      <EmojiPicker onEmojiSelect={onEmojiSelect} />
    );

    fireEvent.click(getByLabelText('Open emoji picker'));
    const searchInput = await findByPlaceholderText('Search emojis...');
    fireEvent.input(searchInput, { target: { value: 'heart' } });
    fireEvent.click(getByLabelText('Insert ❤️'));

    expect(onEmojiSelect).toHaveBeenCalledWith('❤️');
  });
});
