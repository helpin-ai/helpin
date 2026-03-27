import { create } from 'zustand'

export interface DocsViewerState {
  name?: string
  avatarUrl?: string
}

interface DocsPresenceState {
  viewingUsers: Record<string, Record<string, DocsViewerState>>
  setViewingUser: (documentId: string, userId: string, viewing: boolean, metadata?: DocsViewerState) => void
  replaceViewingUsers: (documentId: string, viewers: Record<string, DocsViewerState>) => void
}

export const useDocsPresenceStore = create<DocsPresenceState>((set) => ({
  viewingUsers: {},

  setViewingUser: (documentId, userId, viewing, metadata) =>
    set((state) => {
      const current = state.viewingUsers[documentId] ?? {}
      if (viewing) {
        return {
          viewingUsers: {
            ...state.viewingUsers,
            [documentId]: {
              ...current,
              [userId]: {
                name: metadata?.name ?? current[userId]?.name,
                avatarUrl: metadata?.avatarUrl ?? current[userId]?.avatarUrl,
              },
            },
          },
        }
      }
      if (!(userId in current)) return state
      const { [userId]: _, ...rest } = current
      return {
        viewingUsers: {
          ...state.viewingUsers,
          [documentId]: rest,
        },
      }
    }),
  replaceViewingUsers: (documentId, viewers) =>
    set((state) => ({
      viewingUsers: {
        ...state.viewingUsers,
        [documentId]: viewers,
      },
    })),
}))
