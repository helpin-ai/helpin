// @vitest-environment jsdom
import { act } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

type MockEditorBundle = ReturnType<typeof createMockEditor>

const testState = vi.hoisted(() => ({
  editorBundle: null as MockEditorBundle | null,
  editorOptions: null as Record<string, any> | null,
  uploadEditorImage: vi.fn(),
  importExternalImage: vi.fn(),
  slashMenuGetState: vi.fn(() => null),
}))

function createChain() {
  const chain = {
    focus: vi.fn(() => chain),
    setTextSelection: vi.fn(() => chain),
    setResizableImage: vi.fn(() => chain),
    setVideoEmbed: vi.fn(() => chain),
    unsetLink: vi.fn(() => chain),
    run: vi.fn(() => true),
  }

  return chain
}

function createMockEditor() {
  let currentJson: Record<string, any> = { type: 'doc', content: [{ type: 'paragraph' }] }
  const chain = createChain()
  const doc = {
    descendants: vi.fn(),
    nodeAt: vi.fn(() => null),
    nodesBetween: vi.fn(),
    resolve: vi.fn(() => ({ marks: () => [] })),
    content: { size: 4 },
  }
  const tr = {
    setNodeMarkup: vi.fn(() => tr),
  }
  const editor = {
    state: {
      selection: { from: 1, to: 1, empty: true },
      doc,
      tr,
    },
    view: {
      dispatch: vi.fn(),
      coordsAtPos: vi.fn(() => ({ top: 0, bottom: 0, left: 0 })),
    },
    commands: {
      setContent: vi.fn(),
      focus: vi.fn(),
    },
    chain: vi.fn(() => chain),
    getJSON: vi.fn(() => currentJson),
    getHTML: vi.fn(() => '<p>Doc</p>'),
    storage: {
      markdown: {
        getMarkdown: vi.fn(() => '# Doc'),
      },
    },
    setEditable: vi.fn(),
    isActive: vi.fn(() => false),
    isFocused: false,
    getAttributes: vi.fn(() => ({})),
    on: vi.fn(),
    off: vi.fn(),
  }

  return {
    editor,
    doc,
    setCurrentJson: (json: Record<string, any>) => {
      currentJson = json
    },
  }
}

vi.mock('@tiptap/react', () => ({
  useEditor: (options: Record<string, any>) => {
    testState.editorOptions = options
    return testState.editorBundle?.editor ?? null
  },
  EditorContent: () => <div data-testid="editor-content" />,
}))

vi.mock('@tiptap/starter-kit', () => ({
  default: {
    configure: () => ({}),
  },
}))

vi.mock('@tiptap/extension-placeholder', () => ({
  default: {
    configure: () => ({}),
  },
}))

vi.mock('tiptap-markdown', () => ({
  Markdown: {
    configure: () => ({}),
  },
}))

vi.mock('@tiptap/extension-underline', () => ({ default: {} }))
vi.mock('@tiptap/extension-subscript', () => ({ default: {} }))
vi.mock('@tiptap/extension-superscript', () => ({ default: {} }))
vi.mock('@tiptap/extension-table', () => ({ Table: { configure: () => ({}) } }))
vi.mock('@tiptap/extension-table-row', () => ({ TableRow: {} }))
vi.mock('@tiptap/extension-table-header', () => ({ TableHeader: {} }))
vi.mock('@tiptap/extension-table-cell', () => ({ TableCell: {} }))

vi.mock('@/components/ui/resizable-image-extension', () => ({
  ResizableImageExtension: {},
}))

vi.mock('../SlashMenuExtension', () => ({
  SlashMenuExtension: {},
  slashMenuPluginKey: {
    getState: (...args: unknown[]) => testState.slashMenuGetState(...args),
  },
}))

vi.mock('../CalloutExtension', () => ({ CalloutExtension: {} }))
vi.mock('../VideoEmbedExtension', () => ({ VideoEmbedExtension: {} }))
vi.mock('../HtmlBlockExtension', () => ({ HtmlBlockExtension: {} }))
vi.mock('../CodeBlockExtension', () => ({ CodeBlockExtension: {} }))
vi.mock('../SearchReplaceExtension', () => ({ SearchReplaceExtension: {} }))
vi.mock('../SearchReplaceBar', () => ({ SearchReplaceBar: () => null }))
vi.mock('../EmojiPickerPopover', () => ({ EmojiPickerPopover: () => null }))
vi.mock('../InsertVideoDialog', () => ({ InsertVideoDialog: () => null }))
vi.mock('../TableControls', () => ({ TableControls: () => null }))
vi.mock('../BlockGapInserter', () => ({ BlockGapInserter: () => null }))
vi.mock('../SlashMenu', () => ({ SlashMenu: () => null }))

vi.mock('@/hooks/useEditorImageUpload', () => ({
  uploadEditorImage: (...args: unknown[]) => testState.uploadEditorImage(...args),
}))

vi.mock('@/lib/services/docsService', () => ({
  docsService: {
    importExternalImage: (...args: unknown[]) => testState.importExternalImage(...args),
  },
}))

vi.mock('sonner', () => ({
  toast: {
    success: vi.fn(),
    error: vi.fn(),
    info: vi.fn(),
  },
}))

vi.mock('@/components/ui/quick-tooltip', () => ({
  QuickTooltip: ({ children }: { children: React.ReactNode }) => <>{children}</>,
}))

vi.mock('@/components/ui/dropdown-menu', () => ({
  DropdownMenu: ({ children }: { children: React.ReactNode }) => <>{children}</>,
  DropdownMenuContent: ({ children }: { children: React.ReactNode }) => <div>{children}</div>,
  DropdownMenuItem: ({
    children,
    onSelect,
  }: {
    children: React.ReactNode
    onSelect?: () => void
  }) => <button type="button" onClick={onSelect}>{children}</button>,
  DropdownMenuSeparator: () => <div />,
  DropdownMenuTrigger: ({ children }: { children: React.ReactNode }) => <>{children}</>,
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
    disabled,
    className,
    type = 'button',
  }: {
    children: React.ReactNode
    onClick?: () => void
    disabled?: boolean
    className?: string
    type?: 'button' | 'submit' | 'reset'
  }) => (
    <button type={type} onClick={onClick} disabled={disabled} className={className}>
      {children}
    </button>
  ),
}))

vi.mock('lucide-react', () => {
  const Icon = () => null
  return {
    AlertCircle: Icon,
    Bold: Icon,
    Check: Icon,
    ChevronDown: Icon,
    Code2: Icon,
    Copy: Icon,
    ExternalLink: Icon,
    FileDown: Icon,
    FileUp: Icon,
    Heading2: Icon,
    Heading3: Icon,
    Italic: Icon,
    Link2: Icon,
    List: Icon,
    ListOrdered: Icon,
    Loader2: Icon,
    Quote: Icon,
    Underline: Icon,
    X: Icon,
  }
})

import { DocsEditor } from '../DocsEditor'

(globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true

describe('DocsEditor', () => {
  let container: HTMLDivElement
  let root: Root

  beforeEach(() => {
    vi.useFakeTimers()
    testState.editorBundle = createMockEditor()
    testState.editorOptions = null
    testState.uploadEditorImage.mockReset()
    testState.importExternalImage.mockReset()
    testState.importExternalImage.mockResolvedValue({ data: { url: 'https://cdn.helpin.ai/imported.png' }, error: null })
    testState.slashMenuGetState.mockReset()
    testState.slashMenuGetState.mockReturnValue(null)
    container = document.createElement('div')
    document.body.appendChild(container)
    root = createRoot(container)
  })

  afterEach(() => {
    act(() => {
      root.unmount()
    })
    container.remove()
    vi.runOnlyPendingTimers()
    vi.useRealTimers()
  })

  it('skips redundant autosave and manual save when content matches the last saved snapshot', async () => {
    const initialContent = { type: 'doc', content: [{ type: 'paragraph', content: [{ type: 'text', text: 'alpha' }] }] }
    const changedContent = { type: 'doc', content: [{ type: 'paragraph', content: [{ type: 'text', text: 'beta' }] }] }
    const onSave = vi.fn().mockResolvedValue(undefined)

    testState.editorBundle?.setCurrentJson(initialContent)

    await act(async () => {
      root.render(
        <DocsEditor
          initialContent={initialContent}
          onSave={onSave}
          autoSaveMs={25}
        />,
      )
    })

    await act(async () => {
      testState.editorOptions?.onUpdate?.({ editor: testState.editorBundle?.editor })
      vi.advanceTimersByTime(25)
    })

    expect(onSave).not.toHaveBeenCalled()

    testState.editorBundle?.setCurrentJson(changedContent)

    await act(async () => {
      testState.editorOptions?.onUpdate?.({ editor: testState.editorBundle?.editor })
      vi.advanceTimersByTime(25)
    })

    expect(onSave).toHaveBeenCalledTimes(1)
    expect(onSave).toHaveBeenCalledWith(changedContent)

    await act(async () => {
      window.dispatchEvent(new KeyboardEvent('keydown', { key: 's', metaKey: true, bubbles: true }))
    })

    expect(onSave).toHaveBeenCalledTimes(1)
  })

  it('does not scan the document for imported images on ordinary editor updates', async () => {
    const onSave = vi.fn().mockResolvedValue(undefined)
    const content = { type: 'doc', content: [{ type: 'paragraph', content: [{ type: 'text', text: 'steady' }] }] }

    testState.editorBundle?.setCurrentJson(content)

    await act(async () => {
      root.render(
        <DocsEditor
          initialContent={content}
          onSave={onSave}
          autoSaveMs={25}
          uploadConfig={{ workspaceId: 'ws_1', entityType: 'editor_upload', entityId: 'doc_1' }}
        />,
      )
    })

    await act(async () => {
      testState.editorOptions?.onUpdate?.({ editor: testState.editorBundle?.editor })
    })

    expect(testState.editorBundle?.doc.descendants).not.toHaveBeenCalled()
  })

  it('queues imported-image persistence only when pasted HTML contains images', async () => {
    const onSave = vi.fn().mockResolvedValue(undefined)
    const content = { type: 'doc', content: [{ type: 'paragraph' }] }

    testState.editorBundle?.setCurrentJson(content)

    await act(async () => {
      root.render(
        <DocsEditor
          initialContent={content}
          onSave={onSave}
          uploadConfig={{ workspaceId: 'ws_1', entityType: 'editor_upload', entityId: 'doc_1' }}
        />,
      )
    })

    const plainPaste = {
      clipboardData: {
        items: [],
        getData: vi.fn((type: string) => (type === 'text/html' ? '<p>plain</p>' : '')),
      },
    } as unknown as ClipboardEvent

    await act(async () => {
      testState.editorOptions?.editorProps?.handlePaste?.(null, plainPaste)
      vi.runAllTimers()
    })

    expect(testState.editorBundle?.doc.descendants).not.toHaveBeenCalled()

    const embeddedImagePaste = {
      clipboardData: {
        items: [],
        getData: vi.fn((type: string) =>
          type === 'text/html' ? '<img src="https://cdn.example.com/shot.png" />' : ''),
      },
    } as unknown as ClipboardEvent

    testState.editorBundle?.doc.descendants.mockImplementation((visitor: (node: any, pos: number) => void) => {
      visitor({
        type: { name: 'resizableImage' },
        attrs: { src: 'https://cdn.example.com/shot.png', attachmentId: '', title: null },
      }, 5)
    })
    testState.editorBundle?.doc.nodeAt.mockReturnValue({
      attrs: { src: 'https://cdn.example.com/shot.png', attachmentId: '', title: 'imported-1' },
    })

    await act(async () => {
      testState.editorOptions?.editorProps?.handlePaste?.(null, embeddedImagePaste)
      vi.runAllTimers()
    })

    expect(testState.editorBundle?.doc.descendants).toHaveBeenCalled()
    expect(testState.importExternalImage).toHaveBeenCalledWith('ws_1', 'https://cdn.example.com/shot.png')
  })
})
