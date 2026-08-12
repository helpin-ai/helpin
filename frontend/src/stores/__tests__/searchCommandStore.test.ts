import { beforeEach, describe, expect, it } from 'vitest'
import { useSearchCommandStore } from '../searchCommandStore'

describe('search command store', () => {
  beforeEach(() => useSearchCommandStore.setState({ open: false }))

  it('opens and closes the shared command palette', () => {
    useSearchCommandStore.getState().openSearch()
    expect(useSearchCommandStore.getState().open).toBe(true)

    useSearchCommandStore.getState().setOpen(false)
    expect(useSearchCommandStore.getState().open).toBe(false)
  })
})
