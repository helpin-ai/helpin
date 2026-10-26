import { api } from '@/lib/api'

export type IconSuggestionKind = 'collection' | 'inbox'

export const iconSuggestionsService = {
  async suggest(workspaceId: string, kind: IconSuggestionKind, name: string): Promise<string> {
    const path = kind === 'collection' ? '/docs/icon-suggestion' : '/support/inbox/icon-suggestion'
    const { data, error } = await api.post<{ icon: string }>(
      `${path}?workspace_id=${encodeURIComponent(workspaceId)}`,
      { name },
    )
    // This optional enhancement must never interrupt creation, including when
    // admission is unavailable. No error toast or upgrade prompt while typing.
    if (error) return ''
    return data?.icon ?? ''
  },
}
