import { useEffect, useState } from 'react'
import type { Editor } from '@tiptap/core'
import { useLocation, useNavigate } from '@tanstack/react-router'
import { toast } from 'sonner'
import { Calendar03Icon, LinkSquare01Icon, Loading01Icon, UserAdd01Icon } from '@/lib/icons'
import { openTaskRoute } from '@/components/pm/task-detail/taskRouteNavigation'
import { useAssignableWorkspaceMembers } from '@/hooks/useAssignableWorkspaceMembers'
import { pmTaskService } from '@/lib/services/pmTaskService'
import { useWorkspaceStore } from '@/stores/workspaceStore'

interface TaskMetadata {
  assigneeId: string
  assigneeName: string
  dueDate: string
  pmTaskId: string
  taskKey: string
  text: string
}

function currentTaskMetadata(editor: Editor): TaskMetadata | null {
  const { $from } = editor.state.selection
  for (let depth = $from.depth; depth > 0; depth -= 1) {
    const node = $from.node(depth)
    if (node.type.name !== 'taskItem') continue
    return {
      assigneeId: typeof node.attrs.assigneeId === 'string' ? node.attrs.assigneeId : '',
      assigneeName: typeof node.attrs.assigneeName === 'string' ? node.attrs.assigneeName : '',
      dueDate: typeof node.attrs.dueDate === 'string' ? node.attrs.dueDate : '',
      pmTaskId: typeof node.attrs.pmTaskId === 'string' ? node.attrs.pmTaskId : '',
      taskKey: typeof node.attrs.taskKey === 'string' ? node.attrs.taskKey : '',
      text: node.textContent.trim(),
    }
  }
  return null
}

export function TaskItemMetadataToolbar({ editor }: { editor: Editor }) {
  const navigate = useNavigate()
  const location = useLocation()
  const workspace = useWorkspaceStore((s) => s.currentWorkspace)
  const { members } = useAssignableWorkspaceMembers(workspace?.id)
  const [metadata, setMetadata] = useState<TaskMetadata | null>(() => currentTaskMetadata(editor))
  const [creating, setCreating] = useState(false)

  useEffect(() => {
    const update = () => setMetadata(currentTaskMetadata(editor))
    editor.on('selectionUpdate', update)
    editor.on('transaction', update)
    return () => {
      editor.off('selectionUpdate', update)
      editor.off('transaction', update)
    }
  }, [editor])

  if (!editor.isEditable || !metadata) return null

  const updateTask = (next: TaskMetadata) => {
    setMetadata(next)
    editor.chain().focus().setTaskItemMetadata({
      assigneeName: next.assigneeName.trim() || null,
      assigneeId: next.assigneeId || null,
      dueDate: next.dueDate || null,
      pmTaskId: next.pmTaskId || null,
      taskKey: next.taskKey || null,
    }).run()
  }

  const createOrOpenTask = async () => {
    if (!metadata || !workspace?.id || !workspace.slug) return
    if (metadata.pmTaskId) {
      openTaskRoute(navigate as never, location as never, workspace.slug, metadata.pmTaskId)
      return
    }
    const name = metadata.text || 'Untitled task'
    setCreating(true)
    const { data, error } = await pmTaskService.create({
      workspace_id: workspace.id,
      name,
      owner_member_ids: metadata.assigneeId ? [metadata.assigneeId] : undefined,
      deadline: metadata.dueDate || undefined,
    })
    setCreating(false)
    if (error || !data) {
      toast.error(error || 'Failed to create task')
      return
    }
    const next = {
      ...metadata,
      pmTaskId: data.task.id,
      taskKey: data.task.task_key || `#${data.task.display_id}`,
    }
    updateTask(next)
    toast.success('Task created')
    openTaskRoute(navigate as never, location as never, workspace.slug, data.task.id)
  }

  return (
    <div className="mx-6 mt-2 flex flex-wrap items-center gap-2 rounded-md border border-border bg-popover px-2 py-1.5 text-xs text-popover-foreground shadow-sm">
      <label className="flex min-w-[160px] items-center gap-1.5">
        <UserAdd01Icon className="h-3.5 w-3.5 text-muted-foreground" />
        <select
          value={metadata.assigneeId}
          onChange={(event) => {
            const member = members.find((candidate) => candidate.id === event.target.value)
            updateTask({
              ...metadata,
              assigneeId: event.target.value,
              assigneeName: member?.display_name || '',
            })
          }}
          className="h-7 min-w-0 flex-1 bg-transparent outline-none"
        >
          <option value="">Unassigned</option>
          {members.map((member) => (
            <option key={member.id} value={member.id}>
              {member.display_name}
            </option>
          ))}
        </select>
      </label>
      <label className="flex items-center gap-1.5">
        <Calendar03Icon className="h-3.5 w-3.5 text-muted-foreground" />
        <input
          type="date"
          value={metadata.dueDate}
          onChange={(event) => updateTask({ ...metadata, dueDate: event.target.value })}
          className="h-7 bg-transparent outline-none"
        />
      </label>
      <button
        type="button"
        onClick={() => void createOrOpenTask()}
        disabled={creating || !workspace?.id}
        className="ml-auto inline-flex h-7 items-center gap-1.5 rounded border border-border bg-background px-2 font-medium text-foreground hover:bg-muted disabled:cursor-not-allowed disabled:opacity-50"
      >
        {creating ? <Loading01Icon className="h-3.5 w-3.5 animate-spin" /> : <LinkSquare01Icon className="h-3.5 w-3.5" />}
        {metadata.pmTaskId ? (metadata.taskKey || 'Open task') : 'Create task'}
      </button>
    </div>
  )
}
