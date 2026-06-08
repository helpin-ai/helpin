declare module '@helpin-ai/widget-core' {
  export interface EmojiCategory {
    id: string
    label: string
    emojis: string[]
  }

  export interface EmojiCatalog {
    categories: EmojiCategory[]
    search: (query: string) => string[]
  }

  export function loadEmojiCatalog(): Promise<EmojiCatalog>
}

declare module '@helpin-ai/widget-core/styles' {}
