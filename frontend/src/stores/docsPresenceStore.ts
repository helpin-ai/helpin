import { create } from 'zustand'

export interface DocsViewerState {
  name?: string
  avatarUrl?: string
}

export interface DocsEditorState extends DocsViewerState {
  area: string
  section?: string
}

interface DocsPresenceState {
  viewingUsers: Record<string, Record<string, DocsViewerState>>
  editingUsers: Record<string, Record<string, DocsEditorState>>
  setViewingUser: (documentId: string, userId: string, viewing: boolean, metadata?: DocsViewerState) => void
  replaceViewingUsers: (documentId: string, viewers: Record<string, DocsViewerState>) => void
  setEditingUser: (documentId: string, userId: string, editing: boolean, metadata?: DocsEditorState) => void
  replaceEditingUsers: (documentId: string, editors: Record<string, DocsEditorState>) => void
}

export const useDocsPresenceStore = create<DocsPresenceState>((set) => ({
  viewingUsers: {},
  editingUsers: {},

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
  setEditingUser: (documentId, userId, editing, metadata) =>
    set((state) => {
      const current = state.editingUsers[documentId] ?? {}
      if (editing && metadata) {
        return {
          editingUsers: {
            ...state.editingUsers,
            [documentId]: {
              ...current,
              [userId]: {
                area: metadata.area,
                section: metadata.section,
                name: metadata.name ?? current[userId]?.name,
                avatarUrl: metadata.avatarUrl ?? current[userId]?.avatarUrl,
              },
            },
          },
        }
      }
      if (!(userId in current)) return state
      const { [userId]: _, ...rest } = current
      return {
        editingUsers: {
          ...state.editingUsers,
          [documentId]: rest,
        },
      }
    }),
  replaceEditingUsers: (documentId, editors) =>
    set((state) => ({
      editingUsers: {
        ...state.editingUsers,
        [documentId]: editors,
      },
    })),
}))
