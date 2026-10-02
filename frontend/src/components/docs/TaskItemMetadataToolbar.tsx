import { TeamLabel } from '@/components/workspace/TeamLabel';
import { useEffect, useState } from 'react'
import type { Editor } from '@tiptap/core'
import { useLocation, useNavigate } from '@tanstack/react-router'
import { toast } from 'sonner'
import { Calendar03Icon, LinkSquare01Icon, Loading01Icon, UserAdd01Icon } from '@/lib/icons'
import { openTaskRoute } from '@/components/pm/task-detail/taskRouteNavigation'
import { useAccessibleTeams } from '@/hooks/useAccessibleTeams'
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
  const { teams, loading: teamsLoading } = useAccessibleTeams(workspace?.id || '')
  const { members } = useAssignableWorkspaceMembers(workspace?.id)
  const [metadata, setMetadata] = useState<TaskMetadata | null>(() => currentTaskMetadata(editor))
  const [teamId, setTeamId] = useState('')
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

  useEffect(() => {
    if (teams.some((team) => team.id === teamId)) return
    setTeamId(teams[0]?.id || '')
  }, [teamId, teams])

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
    if (!teamId) {
      toast.error('Select a team before creating the task')
      return
    }
    const name = metadata.text || 'Untitled task'
    setCreating(true)
    const { data, error } = await pmTaskService.create({
      workspace_id: workspace.id,
      name,
      team_id: teamId,
      owner_member_ids: metadata.assigneeId ? [metadata.assigneeId] : undefined,
      deadline: metadata.dueDate || undefined,
    })
    setCreating(false)
    if (error || !data) {
      toast.error(error || 'Failed to create task')
      return
    }
    const createdTask = data.task.task
    const next = {
      ...metadata,
      pmTaskId: createdTask.id,
      taskKey: createdTask.task_key || `#${createdTask.display_id}`,
    }
    updateTask(next)
    toast.success('Task created')
    openTaskRoute(navigate as never, location as never, workspace.slug, createdTask.id)
  }

  return (
    <div className="mx-6 mt-2 flex flex-wrap items-center gap-2 rounded-md border border-border bg-popover px-2 py-1.5 text-xs text-popover-foreground shadow-sm">
      {!metadata.pmTaskId && (
        <label className="flex min-w-[140px] items-center gap-1.5">
          <span className="text-muted-foreground">Team</span>
          <select
            value={teamId}
            onChange={(event) => setTeamId(event.target.value)}
            disabled={teamsLoading}
            className="h-7 min-w-0 flex-1 bg-transparent outline-none"
          >
            <option value="">Select team</option>
            {teams.map((team) => (
              <option key={team.id} value={team.id}>
                <TeamLabel team={team} />
              </option>
            ))}
          </select>
        </label>
      )}
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
        disabled={creating || !workspace?.id || (!metadata.pmTaskId && !teamId)}
        className="ml-auto inline-flex h-7 items-center gap-1.5 rounded border border-border bg-background px-2 font-medium text-foreground hover:bg-muted disabled:cursor-not-allowed disabled:opacity-50"
      >
        {creating ? <Loading01Icon className="h-3.5 w-3.5 animate-spin" /> : <LinkSquare01Icon className="h-3.5 w-3.5" />}
        {metadata.pmTaskId ? (metadata.taskKey || 'Open task') : 'Create task'}
      </button>
    </div>
  )
}
