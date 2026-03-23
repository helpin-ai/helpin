import type { EmojiCatalog } from './emoji-catalog';

let emojiCatalogPromise: Promise<EmojiCatalog> | null = null;
const viteMode = (import.meta as ImportMeta & { env?: { MODE?: string } }).env?.MODE;

const TEST_EMOJI_CATALOG: EmojiCatalog = {
  categories: [
    { id: 'smileys', label: 'Smileys & People', emojis: ['😀'] },
    { id: 'symbols', label: 'Symbols', emojis: ['❤️'] },
  ],
  search: (query: string) => {
    const normalized = query.toLowerCase();
    return normalized.includes('heart') ? ['❤️'] : [];
  },
};

export function loadEmojiCatalog(): Promise<EmojiCatalog> {
  if (viteMode === 'test') {
    return Promise.resolve(TEST_EMOJI_CATALOG);
  }

  if (!emojiCatalogPromise) {
    emojiCatalogPromise = import('./emoji-catalog').then((module) => module.getEmojiCatalog());
  }

  return emojiCatalogPromise;
}
