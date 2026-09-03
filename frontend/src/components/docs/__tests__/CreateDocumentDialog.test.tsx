// @vitest-environment jsdom
import { act } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

const testState = vi.hoisted(() => ({
  spaces: [
    { id: 'space-1', name: 'General', icon: '', type: 'internal' },
    { id: 'space-2', name: 'Support', icon: '', type: 'internal' },
  ],
  collections: [] as Array<{ id: string; name: string }>,
  mutateAsync: vi.fn(),
}))

vi.mock('@/hooks/queries', () => ({
  useCreateDocsDocument: () => ({
    mutateAsync: testState.mutateAsync,
    isPending: false,
  }),
  useDocsSpaces: () => ({
    data: testState.spaces,
  }),
  useDocsCollections: () => ({
    data: testState.collections,
  }),
}))

vi.mock('@/components/ui/dialog', () => ({
  Dialog: ({ children }: { children: React.ReactNode }) => <>{children}</>,
  DialogContent: ({ children }: { children: React.ReactNode }) => <div>{children}</div>,
  DialogDescription: ({ children }: { children: React.ReactNode }) => <div>{children}</div>,
  DialogFooter: ({ children }: { children: React.ReactNode }) => <div>{children}</div>,
  DialogHeader: ({ children }: { children: React.ReactNode }) => <div>{children}</div>,
  DialogTitle: ({ children }: { children: React.ReactNode }) => <div>{children}</div>,
}))

vi.mock('@/components/ui/button', () => ({
  Button: ({
    children,
    onClick,
    type = 'button',
    disabled,
  }: {
    children: React.ReactNode
    onClick?: () => void
    type?: 'button' | 'submit' | 'reset'
    disabled?: boolean
  }) => (
    <button type={type} onClick={onClick} disabled={disabled}>
      {children}
    </button>
  ),
}))

vi.mock('@/components/ui/input', () => ({
  Input: ({
    value,
    onChange,
    id,
    placeholder,
    autoFocus,
  }: {
    value?: string
    onChange?: (event: React.ChangeEvent<HTMLInputElement>) => void
    id?: string
    placeholder?: string
    autoFocus?: boolean
  }) => (
    <input
      id={id}
      value={value}
      onChange={onChange}
      placeholder={placeholder}
      autoFocus={autoFocus}
    />
  ),
}))

vi.mock('@/components/ui/label', () => ({
  Label: ({
    children,
    htmlFor,
  }: {
    children: React.ReactNode
    htmlFor?: string
  }) => <label htmlFor={htmlFor}>{children}</label>,
}))

vi.mock('@/components/ui/select', () => ({
  Select: ({ children }: { children: React.ReactNode }) => <div>{children}</div>,
  SelectContent: ({ children }: { children: React.ReactNode }) => <div>{children}</div>,
  SelectItem: ({ children }: { children: React.ReactNode; value: string }) => <div>{children}</div>,
  SelectTrigger: ({ children }: { children: React.ReactNode }) => <div>{children}</div>,
  SelectValue: ({ placeholder }: { placeholder?: string }) => <span>{placeholder}</span>,
}))

vi.mock('@/components/ui/icon-picker', () => ({
  StoredIcon: () => null,
}))

vi.mock('@/components/billing/UpgradeRequiredDialog', () => ({
  UpgradeRequiredDialog: ({
    open,
    onUpgrade,
  }: {
    open: boolean
    onUpgrade?: () => void
  }) => open ? <button type="button" onClick={onUpgrade}>Mock upgrade</button> : null,
}))

vi.mock('@/components/docs/CollectionTreePicker', () => ({
  CollectionTreePicker: ({
    collections,
    value,
    onChange,
  }: {
    collections: Array<{ id: string; name: string }>
    value: string | null
    onChange: (next: string | null) => void
  }) => (
    <div>
      <span data-testid="collection-value">{value ?? 'none'}</span>
      <button type="button" onClick={() => onChange(null)}>pick-none</button>
      {collections.map((c) => (
        <button key={c.id} type="button" onClick={() => onChange(c.id)}>pick-{c.id}</button>
      ))}
    </div>
  ),
}))

vi.mock('@/lib/icons', () => ({
  FolderOpenIcon: () => null,
  Folder01Icon: () => null,
  ArrowDown01Icon: () => null,
  Tick01Icon: () => null,
}))

vi.mock('sonner', () => ({
  toast: {
    success: vi.fn(),
    error: vi.fn(),
  },
}))

import { CreateDocumentDialog } from '../CreateDocumentDialog'

(globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true

describe('CreateDocumentDialog', () => {
  let container: HTMLDivElement
  let root: Root

  beforeEach(() => {
    testState.mutateAsync.mockReset()
    testState.mutateAsync.mockResolvedValue({ id: 'doc-1' })
    container = document.createElement('div')
    document.body.appendChild(container)
    root = createRoot(container)
  })

  afterEach(() => {
    act(() => {
      root.unmount()
    })
    container.remove()
    testState.collections = []
    testState.spaces = [
      { id: 'space-1', name: 'General', icon: '', type: 'internal' },
      { id: 'space-2', name: 'Support', icon: '', type: 'internal' },
    ]
  })

  const typeTitle = async (value: string) => {
    const titleInput = container.querySelector('input')
    await act(async () => {
      if (titleInput instanceof HTMLInputElement) {
        const valueSetter = Object.getOwnPropertyDescriptor(HTMLInputElement.prototype, 'value')?.set
        valueSetter?.call(titleInput, value)
        titleInput.dispatchEvent(new Event('input', { bubbles: true }))
      }
    })
  }

  const clickButton = async (label: string) => {
    const button = Array.from(container.querySelectorAll('button')).find((b) => b.textContent === label)
    expect(button).toBeTruthy()
    await act(async () => {
      button?.dispatchEvent(new MouseEvent('click', { bubbles: true }))
    })
  }

  const collectionValue = () => container.querySelector('[data-testid="collection-value"]')?.textContent

  it('keeps the collection the user picked when the collections query refetches', async () => {
    testState.collections = [
      { id: 'col-1', name: 'Guides' },
      { id: 'col-2', name: 'Runbooks' },
    ]
    const onOpenChange = vi.fn()
    const render = () => act(async () => {
      root.render(<CreateDocumentDialog wsId="ws-1" open onOpenChange={onOpenChange} />)
    })

    await render()
    expect(collectionValue()).toBe('col-1')

    await clickButton('pick-col-2')
    expect(collectionValue()).toBe('col-2')

    // A background refetch returns a new array instance with the same rows.
    testState.collections = testState.collections.map((c) => ({ ...c }))
    await render()
    expect(collectionValue()).toBe('col-2')

    // An explicit "Uncategorized" pick must survive a refetch too.
    await clickButton('pick-none')
    testState.collections = testState.collections.map((c) => ({ ...c }))
    await render()
    expect(collectionValue()).toBe('none')

    await typeTitle('Runbook')
    await act(async () => {
      container.querySelector('form')?.dispatchEvent(new Event('submit', { bubbles: true, cancelable: true }))
    })
    expect(testState.mutateAsync).toHaveBeenCalledWith({
      title: 'Runbook',
      space_id: 'space-1',
      collection_id: undefined,
    })
  })

  it('keeps the typed title when the spaces query refetches while open', async () => {
    const onOpenChange = vi.fn()
    const render = () => act(async () => {
      root.render(<CreateDocumentDialog wsId="ws-1" open onOpenChange={onOpenChange} />)
    })

    await render()
    await typeTitle('Keep me')

    testState.spaces = testState.spaces.map((s) => ({ ...s }))
    await render()

    expect(container.querySelector('input')?.value).toBe('Keep me')
  })

  it('resets to the provided default space when reopened from a selected space context', async () => {
    const onOpenChange = vi.fn()

    await act(async () => {
      root.render(
        <CreateDocumentDialog
          wsId="ws-1"
          open
          onOpenChange={onOpenChange}
        />,
      )
    })

    await act(async () => {
      root.render(
        <CreateDocumentDialog
          wsId="ws-1"
          open={false}
          onOpenChange={onOpenChange}
        />,
      )
    })

    await act(async () => {
      root.render(
        <CreateDocumentDialog
          wsId="ws-1"
          open
          onOpenChange={onOpenChange}
          defaultSpaceId="space-2"
        />,
      )
    })

    const titleInput = container.querySelector('input')
    const form = container.querySelector('form')

    await act(async () => {
      if (titleInput instanceof HTMLInputElement) {
        const valueSetter = Object.getOwnPropertyDescriptor(
          HTMLInputElement.prototype,
          'value',
        )?.set
        valueSetter?.call(titleInput, 'Runbook')
        titleInput.dispatchEvent(new Event('input', { bubbles: true }))
        titleInput.dispatchEvent(new Event('change', { bubbles: true }))
      }
    })

    await act(async () => {
      form?.dispatchEvent(new Event('submit', { bubbles: true, cancelable: true }))
    })

    expect(testState.mutateAsync).toHaveBeenCalledWith({
      title: 'Runbook',
      space_id: 'space-2',
      collection_id: undefined,
    })
  })

  it('closes the document dialog before routing to upgrade billing', async () => {
    const onOpenChange = vi.fn()
    testState.mutateAsync.mockRejectedValueOnce(new Error('Starter includes up to 500 documents'))

    await act(async () => {
      root.render(
        <CreateDocumentDialog
          wsId="ws-1"
          open
          onOpenChange={onOpenChange}
        />,
      )
    })

    const titleInput = container.querySelector('input')
    const form = container.querySelector('form')

    await act(async () => {
      if (titleInput instanceof HTMLInputElement) {
        const valueSetter = Object.getOwnPropertyDescriptor(
          HTMLInputElement.prototype,
          'value',
        )?.set
        valueSetter?.call(titleInput, 'Limit check')
        titleInput.dispatchEvent(new Event('input', { bubbles: true }))
        titleInput.dispatchEvent(new Event('change', { bubbles: true }))
      }
    })

    await act(async () => {
      form?.dispatchEvent(new Event('submit', { bubbles: true, cancelable: true }))
    })

    const upgradeButton = Array.from(container.querySelectorAll('button')).find(
      (button) => button.textContent === 'Mock upgrade',
    )
    expect(upgradeButton).toBeTruthy()

    await act(async () => {
      upgradeButton?.dispatchEvent(new MouseEvent('click', { bubbles: true }))
    })

    expect(onOpenChange).toHaveBeenCalledWith(false)
  })
})
