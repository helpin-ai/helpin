import { EMOJI_CATEGORIES, searchEmojis, type EmojiCategory } from '@helpin/shared';

export interface EmojiCatalog {
  categories: EmojiCategory[];
  search: (query: string) => string[];
}

export function getEmojiCatalog(): EmojiCatalog {
  return {
    categories: EMOJI_CATEGORIES,
    search: searchEmojis,
  };
}
