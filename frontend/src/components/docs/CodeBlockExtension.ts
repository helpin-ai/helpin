import CodeBlockLowlight from '@tiptap/extension-code-block-lowlight'
import { common, createLowlight } from 'lowlight'
import { ReactNodeViewRenderer } from '@tiptap/react'
import { CodeBlockNodeView } from './CodeBlockNodeView'

// Create lowlight instance with common languages (~35 languages including
// javascript, typescript, python, go, java, rust, bash, sql, json, yaml, etc.)
const lowlight = createLowlight(common)

export const CodeBlockExtension = CodeBlockLowlight.extend({
  addNodeView() {
    return ReactNodeViewRenderer(CodeBlockNodeView)
  },
}).configure({
  lowlight,
  defaultLanguage: 'plaintext',
})
