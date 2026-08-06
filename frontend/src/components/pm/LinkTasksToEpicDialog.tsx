import { useCallback, useDeferredValue, useEffect, useMemo, useRef, useState } from 'react'

import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { Input } from '@/components/ui/input'
import { Loading01Icon, Search01Icon, Tick01Icon } from '@/lib/icons'
import type { LinkEpicTasksResponse, Task } from '@/lib/pmTypes'
import { pmEpicService } from '@/lib/services/pmEpicService'
import { pmTaskService } from '@/lib/services/pmTaskService'

interface LinkTasksToEpicDialogProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  workspaceId: string
  epicId: string
  epicName: string
  teamId: string
  teamName: string
  onLinked: (result: LinkEpicTasksResponse) => void
}

function pluralizeTasks(count: number) {
  return `${count} ${count === 1 ? 'task' : 'tasks'}`
}

function taskEpicName(task: Task) {
  return task.epic_name?.trim() || 'another epic'
}

export function LinkTasksToEpicDialog({
  open,
  onOpenChange,
  workspaceId,
  epicId,
  epicName,
  teamId,
  teamName,
  onLinked,
}: LinkTasksToEpicDialogProps) {
  const [query, setQuery] = useState('')
  const deferredQuery = useDeferredValue(query.trim())
  const [candidates, setCandidates] = useState<Task[]>([])
  const [selected, setSelected] = useState<Map<string, Task>>(new Map())
  const [page, setPage] = useState(1)
  const [totalPages, setTotalPages] = useState(1)
  const [loading, setLoading] = useState(false)
  const [submitting, setSubmitting] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [reviewing, setReviewing] = useState(false)
  const loadRequestRef = useRef(0)

  const loadPage = useCallback(async (nextPage: number, append: boolean) => {
    if (!open || !teamId) return
    const requestID = ++loadRequestRef.current
    setLoading(true)
    setError(null)
    const response = await pmTaskService.list(workspaceId, {
      page: nextPage,
      per_page: 50,
      search: deferredQuery || undefined,
      team_id: teamId,
      archived: false,
    })
    if (requestID !== loadRequestRef.current) return
    if (response.error || !response.data) {
      setError(response.error ?? 'Unable to load tasks.')
      setLoading(false)
      return
    }
    const available = response.data.data.filter((task) => task.epic_id !== epicId)
    setCandidates((current) => append ? [...current, ...available] : available)
    setPage(nextPage)
    setTotalPages(response.data.total_pages || 1)
    setLoading(false)
  }, [deferredQuery, epicId, open, teamId, workspaceId])

  useEffect(() => {
    if (!open) return
    const loadTimer = window.setTimeout(() => {
      void loadPage(1, false)
    }, 0)
    return () => window.clearTimeout(loadTimer)
  }, [deferredQuery, loadPage, open])

  const handleOpenChange = (nextOpen: boolean) => {
    if (nextOpen) {
      onOpenChange(true)
      return
    }
    loadRequestRef.current += 1
    setQuery('')
    setCandidates([])
    setSelected(new Map())
    setReviewing(false)
    setError(null)
    onOpenChange(false)
  }

  const unassigned = useMemo(() => candidates.filter((task) => !task.epic_id), [candidates])
  const assignedElsewhere = useMemo(() => candidates.filter((task) => Boolean(task.epic_id)), [candidates])
  const selectedTasks = useMemo(() => Array.from(selected.values()), [selected])
  const selectedMoves = useMemo(() => selectedTasks.filter((task) => Boolean(task.epic_id)), [selectedTasks])
  const selectedNew = useMemo(() => selectedTasks.filter((task) => !task.epic_id), [selectedTasks])
  const hasMoves = selectedMoves.length > 0

  const movesByEpic = useMemo(() => {
    const groups = new Map<string, Task[]>()
    for (const task of selectedMoves) {
      const name = taskEpicName(task)
      groups.set(name, [...(groups.get(name) ?? []), task])
    }
    return Array.from(groups.entries())
  }, [selectedMoves])

  const toggleTask = (task: Task) => {
    setSelected((current) => {
      const next = new Map(current)
      if (next.has(task.id)) next.delete(task.id)
      else next.set(task.id, task)
      return next
    })
  }

  const submit = async () => {
    if (selectedTasks.length === 0 || submitting) return
    setSubmitting(true)
    setError(null)
    const response = await pmEpicService.linkTasks(workspaceId, epicId, {
      task_ids: selectedTasks.map((task) => task.id),
    })
    setSubmitting(false)
    if (response.error || !response.data) {
      setError(response.error ?? 'Unable to link the selected tasks.')
      return
    }
    onLinked(response.data)
    handleOpenChange(false)
  }

  const renderTaskGroup = (label: string, tasks: Task[]) => {
    if (tasks.length === 0) return null
    return (
      <section className="border-b border-border/60 last:border-b-0">
        <div className="sticky top-0 z-10 flex items-center justify-between border-b border-border/50 bg-muted/45 px-3 py-1.5 text-[11px] font-medium text-muted-foreground">
          <span>{label}</span>
          <span>{tasks.length}</span>
        </div>
        {tasks.map((task) => {
          const checked = selected.has(task.id)
          return (
            <button
              key={task.id}
              type="button"
              data-task-id={task.id}
              aria-pressed={checked}
              className={`grid w-full grid-cols-[24px_minmax(0,1fr)_110px_120px] items-center gap-2 border-b border-border/40 px-3 py-2.5 text-left text-sm transition-colors last:border-b-0 ${checked ? 'bg-primary/5' : 'hover:bg-muted/35'}`}
              onClick={() => toggleTask(task)}
            >
              <span className={`flex h-4 w-4 items-center justify-center rounded-[5px] border ${checked ? 'border-primary bg-primary text-primary-foreground' : 'border-input bg-background'}`}>
                {checked ? <Tick01Icon className="h-3 w-3" /> : null}
              </span>
              <span className="min-w-0">
                <span className="block truncate font-medium text-foreground">{task.name}</span>
                <span className="mt-0.5 block text-xs text-muted-foreground">{task.task_key}</span>
              </span>
              <span className="truncate text-xs text-muted-foreground">{task.state_name || 'No state'}</span>
              <span className="min-w-0 text-xs">
                <span className="block truncate font-medium text-foreground/80">{task.team_name || teamName}</span>
                {task.epic_id ? <span className="block truncate text-[11px] text-amber-700 dark:text-amber-400">{taskEpicName(task)}</span> : null}
              </span>
            </button>
          )
        })}
      </section>
    )
  }

  const selectionLabel = selectedTasks.length === 0 ? 'Select tasks to link' : `${pluralizeTasks(selectedTasks.length)} selected`

  return (
    <Dialog open={open} onOpenChange={handleOpenChange}>
      <DialogContent className="flex max-h-[min(720px,85vh)] flex-col gap-0 overflow-hidden p-0 sm:max-w-3xl">
        <DialogHeader className="border-b border-border/60 px-5 py-4">
          <DialogTitle>{reviewing ? 'Review task moves' : 'Link tasks'}</DialogTitle>
          <DialogDescription>
            {reviewing ? `Confirm how the selected tasks will join ${epicName}.` : `Only tasks from ${teamName} can be linked to this epic.`}
          </DialogDescription>
        </DialogHeader>

        {reviewing ? (
          <div className="min-h-0 flex-1 overflow-y-auto px-5 py-4">
            {selectedNew.length > 0 ? (
              <div className="mb-4">
                <p className="text-sm font-medium">Link from no epic</p>
                <p className="mt-1 text-sm text-muted-foreground">{pluralizeTasks(selectedNew.length)} will be added to {epicName}.</p>
              </div>
            ) : null}
            {movesByEpic.map(([oldEpicName, tasks]) => (
              <div key={oldEpicName} className="border-t border-border/60 py-3 first:border-t-0 first:pt-0">
                <p className="text-sm font-medium text-amber-700 dark:text-amber-400">Move from {oldEpicName}</p>
                <p className="mt-1 text-sm text-muted-foreground">{pluralizeTasks(tasks.length)} will leave {oldEpicName} and move to {epicName}.</p>
                <div className="mt-2 space-y-1">
                  {tasks.map((task) => <p key={task.id} className="truncate text-xs text-foreground/80">{task.task_key} · {task.name}</p>)}
                </div>
              </div>
            ))}
          </div>
        ) : (
          <>
            <div className="border-b border-border/60 px-4 py-3">
              <div className="relative">
                <Search01Icon className="pointer-events-none absolute left-3 top-2.5 h-4 w-4 text-muted-foreground" />
                <Input value={query} onChange={(event) => setQuery(event.target.value)} placeholder="Search by task name or key" className="pl-9" autoFocus />
              </div>
            </div>
            <div className="grid grid-cols-[24px_minmax(0,1fr)_110px_120px] gap-2 border-b border-border/60 bg-muted/20 px-3 py-1.5 text-[10px] font-medium uppercase tracking-wide text-muted-foreground">
              <span />
              <span>Task</span>
              <span>State</span>
              <span>Team</span>
            </div>
            <div className="min-h-[260px] flex-1 overflow-y-auto">
              {renderTaskGroup('No epic', unassigned)}
              {renderTaskGroup('In another epic', assignedElsewhere)}
              {loading && candidates.length === 0 ? (
                <div className="flex items-center justify-center gap-2 py-16 text-sm text-muted-foreground"><Loading01Icon className="h-4 w-4 animate-spin" />Loading tasks…</div>
              ) : null}
              {!loading && candidates.length === 0 ? <p className="py-16 text-center text-sm text-muted-foreground">No eligible tasks found for {teamName}.</p> : null}
              {page < totalPages ? (
                <div className="flex justify-center border-t border-border/60 p-3">
                  <Button variant="ghost" size="sm" disabled={loading} onClick={() => void loadPage(page + 1, true)}>{loading ? 'Loading…' : 'Load more'}</Button>
                </div>
              ) : null}
            </div>
          </>
        )}

        {error ? <p className="border-t border-destructive/20 bg-destructive/5 px-5 py-2 text-sm text-destructive">{error}</p> : null}
        <DialogFooter className="flex-row items-center border-t border-border/60 px-5 py-3 sm:justify-between">
          <span className="mr-auto text-xs text-muted-foreground">{selectionLabel}</span>
          {reviewing ? <Button variant="ghost" onClick={() => setReviewing(false)} disabled={submitting}>Back</Button> : <Button variant="ghost" onClick={() => handleOpenChange(false)}>Cancel</Button>}
          {reviewing ? (
            <Button onClick={() => void submit()} disabled={submitting}>{submitting ? 'Moving…' : `Move and link ${pluralizeTasks(selectedTasks.length)}`}</Button>
          ) : (
            <Button onClick={() => hasMoves ? setReviewing(true) : void submit()} disabled={selectedTasks.length === 0 || submitting}>
              {submitting ? 'Linking…' : hasMoves ? 'Review changes' : `Link ${pluralizeTasks(selectedTasks.length)}`}
            </Button>
          )}
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
