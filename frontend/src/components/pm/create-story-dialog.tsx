import { useState } from 'react'
import { useForm } from 'react-hook-form'
import { standardSchemaResolver } from '@hookform/resolvers/standard-schema'
import { CircleDashed, Clock3, Plus, Tags, UserRound } from 'lucide-react'
import { z } from 'zod'
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Textarea } from '@/components/ui/textarea'
import { Badge } from '@/components/ui/badge'
import { Label } from '@/components/ui/label'
import { Switch } from '@/components/ui/switch'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import {
  STATUS_COLUMNS,
  PRIORITY_CONFIG,
  type Story,
  type StoryPriority,
  type StoryStatus,
} from '@/lib/types/pm'

const schema = z.object({
  title: z.string().trim().min(1, 'Title is required'),
  description: z.string().optional(),
  status: z.enum(['backlog', 'todo', 'in_progress', 'done']),
  priority: z.enum(['urgent', 'high', 'medium', 'low', 'none']),
  assignee: z.string().optional(),
  labels: z.string().optional(),
})

type FormValues = z.infer<typeof schema>

interface CreateStoryDialogProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  defaultStatus: StoryStatus
  nextIdentifier: string
  workspaceName?: string
  onCreateStory: (story: Story) => void
}

export function CreateStoryDialog({
  open,
  onOpenChange,
  defaultStatus,
  nextIdentifier,
  workspaceName,
  onCreateStory,
}: CreateStoryDialogProps) {
  const [createMore, setCreateMore] = useState(false)
  const form = useForm<FormValues>({
    resolver: standardSchemaResolver(schema),
    defaultValues: {
      title: '',
      description: '',
      status: defaultStatus,
      priority: 'none',
      assignee: '',
      labels: '',
    },
  })

  const resetForm = () => {
    form.reset({
      title: '',
      description: '',
      status: defaultStatus,
      priority: 'none',
      assignee: '',
      labels: '',
    })
  }

  const handleOpenChange = (nextOpen: boolean) => {
    if (nextOpen) resetForm()
    onOpenChange(nextOpen)
  }

  const onSubmit = (values: FormValues) => {
    const labels = (values.labels ?? '')
      .split(',')
      .map((label) => label.trim())
      .filter(Boolean)

    const story: Story = {
      id: crypto.randomUUID(),
      identifier: nextIdentifier,
      title: values.title.trim(),
      description: values.description?.trim() || undefined,
      status: values.status as StoryStatus,
      priority: values.priority as StoryPriority,
      assignee: values.assignee?.trim()
        ? {
            name: values.assignee.trim(),
          }
        : undefined,
      labels,
      created_at: new Date().toISOString(),
      sort_order: 0,
    }

    onCreateStory(story)
    if (createMore) {
      resetForm()
      return
    }
    onOpenChange(false)
  }

  return (
    <Dialog open={open} onOpenChange={handleOpenChange}>
      <DialogContent className="max-h-[86vh] max-w-[980px] gap-0 overflow-hidden rounded-lg p-0" showCloseButton={false}>
        <DialogHeader className="border-b border-border/70 px-5 py-4">
          <DialogTitle className="text-4xl font-semibold tracking-tight">
            Create new work item
          </DialogTitle>
        </DialogHeader>

        <form onSubmit={form.handleSubmit(onSubmit)} className="flex flex-col overflow-y-auto">
          <div className="space-y-4 px-5 py-4">
            <Badge variant="outline" className="h-7 rounded-sm px-2 text-sm font-medium">
              <CircleDashed className="h-3.5 w-3.5 text-amber-500" />
              {workspaceName || 'Workspace'}
            </Badge>

            <div className="space-y-2">
              <Input
                placeholder="Title"
                className="h-11 rounded-sm text-base"
                {...form.register('title')}
              />
              {form.formState.errors.title && (
                <p className="text-xs text-destructive">{form.formState.errors.title.message}</p>
              )}
            </div>

            <Textarea
              placeholder="Press '/' for commands"
              className="min-h-[220px] rounded-sm text-base"
              {...form.register('description')}
            />

            <div className="flex flex-wrap items-center gap-2 border-t border-border/70 pt-3">
              <div className="flex items-center gap-2">
                <Label className="text-xs text-muted-foreground">Status</Label>
                <Select
                  value={form.watch('status')}
                  onValueChange={(value) => form.setValue('status', value as StoryStatus)}
                >
                  <SelectTrigger className="h-8 w-[136px] rounded-sm">
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    {STATUS_COLUMNS.map((col) => (
                      <SelectItem key={col.id} value={col.id}>
                        {col.label}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              </div>

              <div className="flex items-center gap-2">
                <Label className="text-xs text-muted-foreground">Priority</Label>
                <Select
                  value={form.watch('priority')}
                  onValueChange={(value) => form.setValue('priority', value as StoryPriority)}
                >
                  <SelectTrigger className="h-8 w-[136px] rounded-sm">
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    {Object.entries(PRIORITY_CONFIG).map(([key, config]) => (
                      <SelectItem key={key} value={key}>
                        {config.label}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              </div>

              <div className="flex items-center gap-2">
                <UserRound className="h-3.5 w-3.5 text-muted-foreground" />
                <Input
                  placeholder="Assignee"
                  className="h-8 w-[150px] rounded-sm"
                  {...form.register('assignee')}
                />
              </div>

              <div className="flex items-center gap-2">
                <Tags className="h-3.5 w-3.5 text-muted-foreground" />
                <Input
                  placeholder="labels, comma separated"
                  className="h-8 w-[220px] rounded-sm"
                  {...form.register('labels')}
                />
              </div>

              <Button type="button" variant="outline" size="sm" className="ml-auto rounded-sm">
                <Clock3 className="h-3.5 w-3.5" />
                Start date
              </Button>
            </div>
          </div>

          <div className="flex items-center justify-between border-t border-border/70 px-5 py-3">
            <label className="flex items-center gap-2 text-sm text-muted-foreground">
              <Switch checked={createMore} onCheckedChange={setCreateMore} />
              Create more
            </label>

            <div className="flex items-center gap-2">
              <Button
                type="button"
                variant="ghost"
                className="rounded-sm"
                onClick={() => onOpenChange(false)}
              >
                Discard
              </Button>
              <Button type="submit" className="rounded-sm">
                <Plus className="h-3.5 w-3.5" />
                Save
              </Button>
            </div>
          </div>
        </form>
      </DialogContent>
    </Dialog>
  )
}
