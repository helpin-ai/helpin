import { Node, mergeAttributes } from '@tiptap/core'
import { ReactNodeViewRenderer } from '@tiptap/react'
import { FileAttachmentNodeView } from './FileAttachmentNodeView'

export interface FileAttachmentAttrs {
  fileName?: string | null
  fileSize?: number | null
  contentType?: string | null
  url?: string | null
  attachmentId?: string | null
}

declare module '@tiptap/core' {
  interface Commands<ReturnType> {
    fileAttachment: {
      setFileAttachment: (attrs: FileAttachmentAttrs) => ReturnType
    }
  }
}

export const FileAttachmentExtension = Node.create({
  name: 'fileAttachment',
  group: 'block',
  atom: true,
  draggable: true,

  addAttributes() {
    return {
      fileName: { default: null, parseHTML: (el) => (el as HTMLElement).getAttribute('data-file-name'), renderHTML: (attrs) => (attrs.fileName ? { 'data-file-name': attrs.fileName } : {}) },
      fileSize: { default: null, parseHTML: (el) => Number((el as HTMLElement).getAttribute('data-file-size') || 0) || null, renderHTML: (attrs) => (attrs.fileSize ? { 'data-file-size': String(attrs.fileSize) } : {}) },
      contentType: { default: null, parseHTML: (el) => (el as HTMLElement).getAttribute('data-content-type'), renderHTML: (attrs) => (attrs.contentType ? { 'data-content-type': attrs.contentType } : {}) },
      url: { default: null, parseHTML: (el) => (el as HTMLElement).getAttribute('data-file-url'), renderHTML: (attrs) => (attrs.url ? { 'data-file-url': attrs.url } : {}) },
      attachmentId: { default: null, parseHTML: (el) => (el as HTMLElement).getAttribute('data-attachment-id'), renderHTML: (attrs) => (attrs.attachmentId ? { 'data-attachment-id': attrs.attachmentId } : {}) },
    }
  },

  parseHTML() {
    return [{ tag: 'div[data-file-attachment]' }]
  },

  renderHTML({ HTMLAttributes }) {
    return ['div', mergeAttributes(HTMLAttributes, { 'data-file-attachment': '' }), HTMLAttributes.fileName || 'Attachment']
  },

  addNodeView() {
    return ReactNodeViewRenderer(FileAttachmentNodeView)
  },

  addCommands() {
    return {
      setFileAttachment:
        (attrs) =>
        ({ commands }) =>
          commands.insertContent({ type: this.name, attrs }),
    }
  },
})
