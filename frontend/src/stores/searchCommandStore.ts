import { create } from 'zustand'

type SearchCommandState = {
  open: boolean
  setOpen: (open: boolean) => void
  openSearch: () => void
}

export const useSearchCommandStore = create<SearchCommandState>((set) => ({
  open: false,
  setOpen: (open) => set({ open }),
  openSearch: () => set({ open: true }),
}))
