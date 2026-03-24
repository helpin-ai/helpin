// @vitest-environment jsdom
import { act } from 'react'
import { createRoot } from 'react-dom/client'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import type { WorkflowWithStates } from '@/lib/pmTypes'

const toastSuccess = vi.fn()
const toastError = vi.fn()

vi.mock('sonner', () => ({
  toast: {
    success: (...args: unknown[]) => toastSuccess(...args),
    error: (...args: unknown[]) => toastError(...args),
  },
}))

vi.mock('@/components/ui/button', () => ({
  Button: ({ children, ...props }: React.ButtonHTMLAttributes<HTMLButtonElement>) => (
    <button {...props}>{children}</button>
  ),
}))

vi.mock('@/components/ui/input', () => ({
  Input: (props: React.InputHTMLAttributes<HTMLInputElement>) => <input {...props} />,
}))

vi.mock('@/components/ui/switch', () => ({
  Switch: ({
    checked,
    onCheckedChange,
  }: {
    checked?: boolean
    onCheckedChange?: (checked: boolean) => void
  }) => (
    <input
      type="checkbox"
      checked={checked}
      onChange={(event) => onCheckedChange?.(event.target.checked)}
    />
  ),
}))

vi.mock('@/components/ui/dialog', () => ({
  Dialog: ({ open, children }: { open?: boolean; children: React.ReactNode }) => (open ? <div>{children}</div> : null),
  DialogContent: ({ children }: { children: React.ReactNode }) => <div>{children}</div>,
  DialogDescription: ({ children }: { children: React.ReactNode }) => <div>{children}</div>,
  DialogHeader: ({ children }: { children: React.ReactNode }) => <div>{children}</div>,
  DialogTitle: ({ children }: { children: React.ReactNode }) => <div>{children}</div>,
}))

vi.mock('@/components/ui/popover', () => ({
  Popover: ({ children }: { children: React.ReactNode }) => <div>{children}</div>,
  PopoverTrigger: ({ children }: { children: React.ReactNode }) => <div>{children}</div>,
  PopoverContent: ({ children }: { children: React.ReactNode }) => <div>{children}</div>,
}))

vi.mock('@/components/ui/tiptap-editor', () => ({
  TiptapEditor: ({ content, onChange }: { content: string; onChange: (value: string) => void }) => (
    <textarea value={content} onChange={(event) => onChange(event.target.value)} />
  ),
}))

vi.mock('@/components/ui/date-picker', () => ({
  DatePicker: ({ value = '', onChange }: { value?: string; onChange: (value: string) => void }) => (
    <input value={value} onChange={(event) => onChange(event.target.value)} />
  ),
}))

vi.mock('@/components/pm/LabelPicker', () => ({
  LabelPicker: () => <div />,
}))

vi.mock('@/components/pm/EstimatePicker', () => ({
  EstimatePicker: () => <div />,
}))

vi.mock('@/components/pm/MemberPickerPopover', () => ({
  MemberPickerPopover: ({ renderTrigger }: { renderTrigger: () => React.ReactNode }) => <div>{renderTrigger()}</div>,
}))

vi.mock('@/components/pm/UserAvatar', () => ({
  UserAvatar: () => <div />,
}))

vi.mock('@/components/pm/RecurringTemplateForm', () => ({
  RecurringTemplateForm: () => <div />,
}))

vi.mock('@/components/pm/RecurringTemplateBadge', () => ({
  RecurringTemplateBadge: () => <div />,
}))

vi.mock('@/hooks/useAccessibleTeams', () => ({
  useAccessibleTeams: () => ({
    teams: [{ id: 'team-1', name: 'Core', default_story_type: 'feature' }],
  }),
}))

vi.mock('@/hooks/useAssignableWorkspaceMembers', () => ({
  useAssignableWorkspaceMembers: () => ({
    members: [],
  }),
}))

vi.mock('@/hooks/queries/useSettings', () => ({
  useTeamFieldVisibilityForTeam: () => ({
    priority: true,
    story_type: true,
    severity: true,
    labels: true,
    epic: true,
    sprint: true,
    estimate: true,
    due_date: true,
    blocked: true,
    delivery: true,
    dev_history: true,
  }),
}))

vi.mock('@/hooks/queries/useSession', () => ({
  useSession: () => ({
    data: { id: 'member-1' },
  }),
}))

vi.mock('@/lib/services/pmEpicService', () => ({
  pmEpicService: { list: vi.fn(async () => ({ data: [] })) },
}))

vi.mock('@/lib/services/pmSprintService', () => ({
  pmSprintService: { list: vi.fn(async () => ({ data: [] })) },
}))

vi.mock('@/lib/services/pmLabelService', () => ({
  pmLabelService: { list: vi.fn(async () => ({ data: [] })) },
}))

vi.mock('@/lib/services/pmStoryTemplateService', () => ({
  pmStoryTemplateService: { list: vi.fn(async () => ({ data: [] })) },
}))

vi.mock('@/lib/services/pmWorkflowService', () => ({
  pmWorkflowService: {
    resolveTeamWorkflow: vi.fn(async () => ({ data: null, error: null })),
  },
}))

vi.mock('@/lib/services/pmAttachmentService', () => ({
  pmAttachmentService: {
    remove: vi.fn(async () => undefined),
    initiateUpload: vi.fn(async () => ({ data: null })),
    confirmUpload: vi.fn(async () => undefined),
  },
}))

vi.mock('@/lib/services/pmRecurringTemplateService', () => ({
  pmRecurringTemplateService: {
    create: vi.fn(async () => ({ error: null })),
  },
}))

vi.mock('@/lib/api', () => ({
  uploadToS3: vi.fn(async () => ({ ok: true })),
}))

import { CreateStoryModal } from '../CreateStoryModal'

(globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true

const workflow: WorkflowWithStates = {
  workflow: {
    id: 'workflow-1',
    workspace_id: 'ws-1',
    team_id: 'team-1',
    name: 'Default workflow',
    description: null,
    default_state_id: 'state-1',
    created_at: '2026-03-24T00:00:00Z',
    updated_at: '2026-03-24T00:00:00Z',
  },
  states: [
    {
      id: 'state-1',
      workflow_id: 'workflow-1',
      name: 'Backlog',
      state_type: 'backlog',
      position: 0,
      color: '#999999',
      description: null,
      wip_limit: null,
      is_default: true,
      created_at: '2026-03-24T00:00:00Z',
      updated_at: '2026-03-24T00:00:00Z',
    },
  ],
}

function setInputValue(input: HTMLInputElement, value: string) {
  const descriptor = Object.getOwnPropertyDescriptor(HTMLInputElement.prototype, 'value')
  descriptor?.set?.call(input, value)
  input.dispatchEvent(new Event('input', { bubbles: true }))
}

function setChecked(input: HTMLInputElement, checked: boolean) {
  const descriptor = Object.getOwnPropertyDescriptor(HTMLInputElement.prototype, 'checked')
  descriptor?.set?.call(input, checked)
  input.dispatchEvent(new Event('change', { bubbles: true }))
}

describe('CreateStoryModal', () => {
  beforeEach(() => {
    toastSuccess.mockReset()
    toastError.mockReset()
  })

  afterEach(() => {
    document.body.innerHTML = ''
  })

  it('shows a success toast for plain story creation before resetting the form when create more is enabled', async () => {
    const onCreate = vi.fn(async () => ({ id: 'story-1' }))
    const onOpenChange = vi.fn()
    const container = document.createElement('div')
    document.body.appendChild(container)
    const root = createRoot(container)

    await act(async () => {
      root.render(
        <CreateStoryModal
          open
          onOpenChange={onOpenChange}
          workspaceId="ws-1"
          workflow={workflow}
          initialStateId="state-1"
          initialTeamId="team-1"
          onCreate={onCreate}
        />,
      )
      await Promise.resolve()
      await Promise.resolve()
    })

    const titleInput = container.querySelector('#story-title') as HTMLInputElement | null
    const createMoreToggle = Array.from(container.querySelectorAll('input')).find(
      (input) => (input as HTMLInputElement).type === 'checkbox',
    ) as HTMLInputElement | undefined
    const saveButton = Array.from(container.querySelectorAll('button')).find(
      (button) => button.textContent === 'Save',
    ) as HTMLButtonElement | undefined

    expect(titleInput).toBeTruthy()
    expect(createMoreToggle).toBeTruthy()
    expect(saveButton).toBeTruthy()

    await act(async () => {
      setInputValue(titleInput!, 'New story')
      setChecked(createMoreToggle!, true)
      await Promise.resolve()
    })

    await act(async () => {
      saveButton?.click()
      await Promise.resolve()
      await Promise.resolve()
    })

    expect(onCreate).toHaveBeenCalledWith(
      expect.objectContaining({
        name: 'New story',
        workspace_id: 'ws-1',
        workflow_id: 'workflow-1',
        workflow_state_id: 'state-1',
      }),
    )
    expect(toastSuccess).toHaveBeenCalledWith('Story created')

    act(() => {
      root.unmount()
    })
  })
})
