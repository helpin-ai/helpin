import { TaskItem } from '@tiptap/extension-task-item'

export interface DocsTaskItemMetadata {
  assigneeId?: string | null
  assigneeName?: string | null
  dueDate?: string | null
  pmTaskId?: string | null
  taskKey?: string | null
}

declare module '@tiptap/core' {
  interface Commands<ReturnType> {
    docsTaskItem: {
      setTaskItemMetadata: (attrs: DocsTaskItemMetadata) => ReturnType
    }
  }
}

export const DocsTaskItemExtension = TaskItem.extend({
  addAttributes() {
    return {
      ...this.parent?.(),
      assigneeId: {
        default: null,
        parseHTML: (el) => (el as HTMLElement).getAttribute('data-assignee-id'),
        renderHTML: (attrs) => (attrs.assigneeId ? { 'data-assignee-id': attrs.assigneeId } : {}),
      },
      assigneeName: {
        default: null,
        parseHTML: (el) => (el as HTMLElement).getAttribute('data-assignee-name'),
        renderHTML: (attrs) => (attrs.assigneeName ? { 'data-assignee-name': attrs.assigneeName } : {}),
      },
      dueDate: {
        default: null,
        parseHTML: (el) => (el as HTMLElement).getAttribute('data-due-date'),
        renderHTML: (attrs) => (attrs.dueDate ? { 'data-due-date': attrs.dueDate } : {}),
      },
      pmTaskId: {
        default: null,
        parseHTML: (el) => (el as HTMLElement).getAttribute('data-pm-task-id'),
        renderHTML: (attrs) => (attrs.pmTaskId ? { 'data-pm-task-id': attrs.pmTaskId } : {}),
      },
      taskKey: {
        default: null,
        parseHTML: (el) => (el as HTMLElement).getAttribute('data-task-key'),
        renderHTML: (attrs) => (attrs.taskKey ? { 'data-task-key': attrs.taskKey } : {}),
      },
    }
  },

  addCommands() {
    return {
      ...this.parent?.(),
      setTaskItemMetadata:
        (attrs) =>
        ({ state, tr, dispatch }) => {
          const { $from } = state.selection
          for (let depth = $from.depth; depth > 0; depth -= 1) {
            const node = $from.node(depth)
            if (node.type.name !== this.name) continue
            const pos = $from.before(depth)
            const nextAttrs = {
              ...node.attrs,
              assigneeId: attrs.assigneeId === undefined ? node.attrs.assigneeId : attrs.assigneeId || null,
              assigneeName: attrs.assigneeName === undefined ? node.attrs.assigneeName : attrs.assigneeName || null,
              dueDate: attrs.dueDate === undefined ? node.attrs.dueDate : attrs.dueDate || null,
              pmTaskId: attrs.pmTaskId === undefined ? node.attrs.pmTaskId : attrs.pmTaskId || null,
              taskKey: attrs.taskKey === undefined ? node.attrs.taskKey : attrs.taskKey || null,
            }
            if (dispatch) dispatch(tr.setNodeMarkup(pos, undefined, nextAttrs))
            return true
          }
          return false
        },
    }
  },
}).configure({ nested: true })
