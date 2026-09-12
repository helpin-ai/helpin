import CodeBlockLowlight from '@tiptap/extension-code-block-lowlight'
import { ReactNodeViewRenderer } from '@tiptap/react'
import { common, createLowlight } from 'lowlight'
import { CodeBlockNodeView } from './CodeBlockNodeView'
import { pickBlockNodeViewAttrs } from './nodeViewAttrs'

const lowlight = createLowlight(common)
lowlight.registerAlias({ xml: ['svg'] })

export const CodeBlockExtension = CodeBlockLowlight.extend({
  addNodeView() {
    return ReactNodeViewRenderer(CodeBlockNodeView, { attrs: ({ HTMLAttributes }) => pickBlockNodeViewAttrs(HTMLAttributes) })
  },
}).configure({
  lowlight,
  defaultLanguage: 'plaintext',
})
