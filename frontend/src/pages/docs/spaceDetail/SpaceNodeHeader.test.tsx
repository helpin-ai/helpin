// @vitest-environment jsdom
import { act } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { afterEach, describe, expect, it, vi } from 'vitest'
import type { CollectionTreeNode } from '@/components/docs/docsCollectionTree'
import type { DocsSpace } from '@/lib/docsTypes'
import { TooltipProvider } from '@/components/ui/tooltip'
import type { NodeView } from './nodeSelection'
import { SpaceNodeHeader } from './SpaceNodeHeader'

;(globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true

const space = { id: 'space-1', name: 'Help Center', type: 'external_capable' } as DocsSpace

function collectionNode(id: string, name: string): CollectionTreeNode {
  return { collection: { id, name, depth: 0 }, children: [], documents: [] } as unknown as CollectionTreeNode
}

let root: Root | null = null

function renderHeader(view: NodeView) {
  const onNavigateToAllSpaces = vi.fn()
  const onNavigateToCollection = vi.fn()
  const container = document.createElement('div')
  document.body.appendChild(container)
  root = createRoot(container)
  act(() => root!.render(
    <TooltipProvider>
      <SpaceNodeHeader
        space={space}
        view={view}
        canEdit={false}
        onNavigateToCollection={onNavigateToCollection}
        onNavigateToAllSpaces={onNavigateToAllSpaces}
        onCreateDocument={vi.fn()}
        onCreateChildCollection={null}
        onEditSpace={vi.fn()}
        onDeleteSpace={vi.fn()}
        onEditCollection={null}
        onDeleteCollection={null}
      />
    </TooltipProvider>,
  ))
  const trail = () => Array.from(container.querySelectorAll('nav[aria-label="Breadcrumb"] button:not([aria-label])'))
    .map((button) => button.textContent)
  return { container, trail, onNavigateToAllSpaces, onNavigateToCollection }
}

describe('SpaceNodeHeader', () => {
  afterEach(() => {
    act(() => root?.unmount())
    root = null
    document.body.innerHTML = ''
  })

  it('goes back to All spaces from the space root', () => {
    const { container, trail, onNavigateToAllSpaces } = renderHeader({ kind: 'space_root' })

    expect(trail()).toEqual(['All spaces'])
    act(() => container.querySelector<HTMLButtonElement>('[aria-label="Back to All spaces"]')!.click())
    expect(onNavigateToAllSpaces).toHaveBeenCalled()
  })

  it('starts the collection trail at All spaces, then the space', () => {
    const inbox = collectionNode('inbox', 'Inbox')
    const { container, trail, onNavigateToAllSpaces, onNavigateToCollection } = renderHeader({
      kind: 'collection',
      node: collectionNode('routing', 'Routing'),
      ancestors: [inbox],
    })

    expect(trail()).toEqual(['All spaces', 'Help Center', 'Inbox'])
    const links = container.querySelectorAll<HTMLButtonElement>('nav[aria-label="Breadcrumb"] button:not([aria-label])')
    act(() => links[0].click())
    expect(onNavigateToAllSpaces).toHaveBeenCalledTimes(1)
    act(() => links[1].click())
    expect(onNavigateToCollection).toHaveBeenCalledWith(null)
    act(() => container.querySelector<HTMLButtonElement>('[aria-label="Back to Inbox"]')!.click())
    expect(onNavigateToCollection).toHaveBeenCalledWith('inbox')
  })

  it('keeps All spaces in the uncategorized trail and goes back to the space', () => {
    const { container, trail, onNavigateToCollection } = renderHeader({ kind: 'uncategorized' })
    expect(trail()).toEqual(['All spaces', 'Help Center'])
    act(() => container.querySelector<HTMLButtonElement>('[aria-label="Back to Help Center"]')!.click())
    expect(onNavigateToCollection).toHaveBeenCalledWith(null)
  })
})
