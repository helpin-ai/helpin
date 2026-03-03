import { useForm } from 'react-hook-form'
import { standardSchemaResolver } from '@hookform/resolvers/standard-schema'
import { z } from 'zod'
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogFooter,
} from '@/components/ui/dialog'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Textarea } from '@/components/ui/textarea'
import { Label } from '@/components/ui/label'
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
  type StoryStatus,
  type StoryPriority,
} from '@/lib/types/pm'

const schema = z.object({
  title: z.string().min(1, 'Title is required'),
  status: z.enum(['backlog', 'todo', 'in_progress', 'done']),
  priority: z.enum(['urgent', 'high', 'medium', 'low', 'none']),
  description: z.string().optional(),
})

type FormValues = z.infer<typeof schema>

interface CreateStoryDialogProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  defaultStatus: StoryStatus
  nextIdentifier: string
  onCreateStory: (story: Story) => void
}

export function CreateStoryDialog({
  open,
  onOpenChange,
  defaultStatus,
  nextIdentifier,
  onCreateStory,
}: CreateStoryDialogProps) {
  const form = useForm<FormValues>({
    resolver: standardSchemaResolver(schema),
    defaultValues: {
      title: '',
      status: defaultStatus,
      priority: 'none',
      description: '',
    },
  })

  // Reset form when dialog opens with new default status
  const handleOpenChange = (nextOpen: boolean) => {
    if (nextOpen) {
      form.reset({
        title: '',
        status: defaultStatus,
        priority: 'none',
        description: '',
      })
    }
    onOpenChange(nextOpen)
  }

  const onSubmit = (values: FormValues) => {
    const story: Story = {
      id: crypto.randomUUID(),
      identifier: nextIdentifier,
      title: values.title,
      description: values.description || undefined,
      status: values.status as StoryStatus,
      priority: values.priority as StoryPriority,
      labels: [],
      created_at: new Date().toISOString(),
      sort_order: 0,
    }
    onCreateStory(story)
    onOpenChange(false)
  }

  return (
    <Dialog open={open} onOpenChange={handleOpenChange}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>Create Story</DialogTitle>
        </DialogHeader>

        <form onSubmit={form.handleSubmit(onSubmit)} className="space-y-4">
          {/* Title */}
          <div className="space-y-2">
            <Label htmlFor="title">Title</Label>
            <Input
              id="title"
              placeholder="Story title..."
              {...form.register('title')}
            />
            {form.formState.errors.title && (
              <p className="text-xs text-destructive">
                {form.formState.errors.title.message}
              </p>
            )}
          </div>

          {/* Status */}
          <div className="space-y-2">
            <Label>Status</Label>
            <Select
              value={form.watch('status')}
              onValueChange={(val) =>
                form.setValue('status', val as StoryStatus)
              }
            >
              <SelectTrigger>
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

          {/* Priority */}
          <div className="space-y-2">
            <Label>Priority</Label>
            <Select
              value={form.watch('priority')}
              onValueChange={(val) =>
                form.setValue('priority', val as StoryPriority)
              }
            >
              <SelectTrigger>
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

          {/* Description */}
          <div className="space-y-2">
            <Label htmlFor="description">Description (optional)</Label>
            <Textarea
              id="description"
              placeholder="Brief description..."
              rows={3}
              {...form.register('description')}
            />
          </div>

          <DialogFooter>
            <Button
              type="button"
              variant="outline"
              onClick={() => onOpenChange(false)}
            >
              Cancel
            </Button>
            <Button type="submit">Create</Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  )
}
