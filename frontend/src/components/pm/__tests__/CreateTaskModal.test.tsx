// @vitest-environment jsdom
import { act } from 'react'
import { createRoot } from 'react-dom/client'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import type { WorkflowWithStates } from '@/lib/pmTypes'
import { uploadToS3 } from '@/lib/api'
import { pmAttachmentService } from '@/lib/services/pmAttachmentService'
import { pmLabelService } from '@/lib/services/pmLabelService'
import { pmTaskService } from '@/lib/services/pmTaskService'
import { pmTaskTemplateService } from '@/lib/services/pmTaskTemplateService'
import { pmWorkflowService } from '@/lib/services/pmWorkflowService'

const toastSuccess = vi.fn()
const toastError = vi.fn()
const navigate = vi.fn()
const showEntityCreatedToast = vi.fn()

vi.mock('sonner', () => ({
  toast: {
    success: (...args: unknown[]) => toastSuccess(...args),
    error: (...args: unknown[]) => toastError(...args),
  },
}))

vi.mock('@tanstack/react-router', () => ({
  useNavigate: () => navigate,
}))

vi.mock('@/stores/workspaceStore', () => ({
  useWorkspaceStore: () => ({ currentWorkspace: { slug: 'acme' } }),
}))

vi.mock('@/components/ui/entity-created-toast', () => ({
  showEntityCreatedToast: (...args: unknown[]) => showEntityCreatedToast(...args),
  entityCreatedToastIcons: { task: () => null },
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

vi.mock('@/components/ui/popover', async () => {
  const React = await import('react')
  const Context = React.createContext<{ open?: boolean; onOpenChange?: (open: boolean) => void }>({})
  return {
    Popover: ({ children, open, onOpenChange }: { children: React.ReactNode; open?: boolean; onOpenChange?: (open: boolean) => void }) =>
      <Context.Provider value={{ open, onOpenChange }}><div>{children}</div></Context.Provider>,
    PopoverTrigger: ({ children }: { children: React.ReactElement<{ onClick?: (event: React.MouseEvent) => void }> }) => {
      const context = React.useContext(Context)
      return React.cloneElement(children, { onClick: (event: React.MouseEvent) => {
        children.props.onClick?.(event)
        context.onOpenChange?.(!context.open)
      } })
    },
    PopoverContent: ({ children }: { children: React.ReactNode }) => <div>{children}</div>,
  }
})

vi.mock('@/components/ui/tooltip', () => ({
  Tooltip: ({ children }: { children: React.ReactNode }) => <div>{children}</div>,
  TooltipContent: ({ children }: { children: React.ReactNode }) => <div>{children}</div>,
  TooltipProvider: ({ children }: { children: React.ReactNode }) => <div>{children}</div>,
  TooltipTrigger: ({ children }: { children: React.ReactNode }) => <div>{children}</div>,
}))

vi.mock('@/components/ui/command', () => ({
  Command: ({ children }: { children: React.ReactNode }) => <div>{children}</div>,
  CommandEmpty: ({ children }: { children: React.ReactNode }) => <div>{children}</div>,
  CommandGroup: ({ children }: { children: React.ReactNode }) => <div>{children}</div>,
  CommandInput: ({ placeholder }: { placeholder?: string }) => <input placeholder={placeholder} />,
  CommandItem: ({
    children,
    onSelect,
  }: {
    children: React.ReactNode
    onSelect?: () => void
  }) => (
    <button type="button" onClick={() => onSelect?.()}>
      {children}
    </button>
  ),
  CommandSeparator: () => <hr />,
  CommandList: ({ children }: { children: React.ReactNode }) => <div>{children}</div>,
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

vi.mock('@/components/billing/UpgradeRequiredDialog', () => ({
  UpgradeRequiredDialog: () => null,
}))

vi.mock('@/components/pm/LabelPicker', () => ({
  LabelPicker: () => <div />,
}))

vi.mock('@/components/pm/EstimatePicker', () => ({
  EstimatePicker: () => <div />,
}))

vi.mock('@/components/pm/MemberPickerPopover', () => ({
  MemberPickerPopover: ({ renderTrigger }: { renderTrigger: () => React.ReactNode }) => <div>{renderTrigger()}</div>,
  MultiMemberPickerPopover: ({ renderTrigger }: { renderTrigger: () => React.ReactNode }) => <div>{renderTrigger()}</div>,
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
    teams: [
      { id: 'team-1', name: 'Core', default_task_type: 'feature' },
      { id: 'team-2', name: 'Support', default_task_type: 'bug' },
    ],
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
    task_type: true,
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

vi.mock('@/hooks/queries/useAgents', () => ({
  useAgents: () => ({
    data: [
      {
        id: 'agent-scribe',
        workspace_id: 'ws-1',
        is_system: true,
        name: 'Scribe',
        role: 'Task Planner',
        status: 'idle',
        runtime_kind: 'native_sdk',
        skills: [],
        tools: [],
        allowed_tools: [],
        allowed_commands: [],
        allowed_targets: ['task'],
        preset_key: 'task_planner',
        trigger_mode: 'manual',
        tokens_used_this_month: 0,
        approval_mode: 'preset_default',
        max_concurrent_runs: 1,
        default_invocation_mode: 'interactive',
        created_at: '',
        updated_at: '',
      },
    ],
    isLoading: false,
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

vi.mock('@/lib/services/pmTaskTemplateService', () => ({
  pmTaskTemplateService: {
    list: vi.fn(async () => ({ data: [] })),
    create: vi.fn(async () => ({ data: { id: 'template-1' }, error: null })),
    update: vi.fn(async () => ({ data: { id: 'template-1' }, error: null })),
  },
}))

vi.mock('@/lib/services/pmTaskService', () => ({
  pmTaskService: {
    saveAsTemplate: vi.fn(async () => ({ data: { id: 'template-1' }, error: null })),
  },
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
  api: {
    get: vi.fn(async () => ({ data: null, error: null })),
    post: vi.fn(async () => ({ data: null, error: null })),
    put: vi.fn(async () => ({ data: null, error: null })),
    del: vi.fn(async () => ({ data: null, error: null })),
  },
  uploadToS3: vi.fn(async () => ({ ok: true })),
}))

import { buildTaskTemplateSelectGroups, CreateTaskModal } from '../CreateTaskModal'

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
    {
      id: 'state-2',
      workflow_id: 'workflow-1',
      name: 'In Progress',
      state_type: 'started',
      position: 1,
      color: '#3b82f6',
      description: null,
      wip_limit: null,
      is_default: false,
      created_at: '2026-03-24T00:00:00Z',
      updated_at: '2026-03-24T00:00:00Z',
    },
  ],
}

const supportWorkflow: WorkflowWithStates = {
  workflow: {
    id: 'workflow-2',
    workspace_id: 'ws-1',
    team_id: 'team-2',
    name: 'Support workflow',
    description: null,
    default_state_id: 'support-state-new',
    created_at: '2026-03-24T00:00:00Z',
    updated_at: '2026-03-24T00:00:00Z',
  },
  states: [
    {
      id: 'support-state-new',
      workflow_id: 'workflow-2',
      name: 'New',
      state_type: 'backlog',
      position: 0,
      color: '#999999',
      description: null,
      wip_limit: null,
      is_default: true,
      created_at: '2026-03-24T00:00:00Z',
      updated_at: '2026-03-24T00:00:00Z',
    },
    {
      id: 'support-state-triage',
      workflow_id: 'workflow-2',
      name: 'Triage',
      state_type: 'started',
      position: 1,
      color: '#f97316',
      description: null,
      wip_limit: null,
      is_default: false,
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

function findCheckboxByText(container: HTMLElement, text: string) {
  const label = Array.from(container.querySelectorAll('span')).find((node) => node.textContent === text)
  return label?.parentElement?.querySelector('input[type="checkbox"]') as HTMLInputElement | null | undefined
}

async function openTemplateMenu(container: HTMLElement) {
  const applyTemplateButton = Array.from(container.querySelectorAll('button')).find(
    (button) => button.textContent?.includes('Apply template'),
  ) as HTMLButtonElement | undefined
  expect(applyTemplateButton).toBeTruthy()
  await act(async () => {
    applyTemplateButton?.click()
    await Promise.resolve()
    await Promise.resolve()
  })
}

function deferred<T>() {
  let resolve!: (value: T) => void
  let reject!: (reason?: unknown) => void
  const promise = new Promise<T>((promiseResolve, promiseReject) => {
    resolve = promiseResolve
    reject = promiseReject
  })
  return { promise, resolve, reject }
}

describe('CreateTaskModal', () => {
  beforeEach(() => {
    toastSuccess.mockReset()
    toastError.mockReset()
    navigate.mockReset()
    showEntityCreatedToast.mockReset()
    vi.mocked(pmTaskTemplateService.list).mockReset()
    vi.mocked(pmTaskTemplateService.list).mockResolvedValue({ data: [] } as any)
    vi.mocked(pmTaskTemplateService.create).mockResolvedValue({ data: { id: 'template-1' }, error: null } as any)
    vi.mocked(pmTaskTemplateService.update).mockResolvedValue({ data: { id: 'template-1' }, error: null } as any)
    vi.mocked(pmTaskService.saveAsTemplate).mockReset()
    vi.mocked(pmTaskService.saveAsTemplate).mockResolvedValue({ data: { id: 'template-1' }, error: null } as any)
    vi.mocked(pmWorkflowService.resolveTeamWorkflow).mockResolvedValue({ data: null, error: null } as any)
    vi.mocked(pmLabelService.list).mockResolvedValue({ data: [] } as any)
    vi.mocked(pmAttachmentService.initiateUpload).mockResolvedValue({
      data: {
        attachment: { id: 'attachment-1' },
        url: 'https://uploads.test/attachment-1',
      },
    } as any)
    vi.mocked(pmAttachmentService.confirmUpload).mockResolvedValue({} as any)
    vi.mocked(uploadToS3).mockResolvedValue({ ok: true } as any)
  })

  afterEach(() => {
    document.body.innerHTML = ''
  })

  it('groups task templates by teams that have templates', () => {
    const groups = buildTaskTemplateSelectGroups(
      [
        {
          id: 'shared-template',
          workspace_id: 'ws-1',
          name: 'Shared checklist',
          archived: false,
          created_at: '2026-03-24T00:00:00Z',
          updated_at: '2026-03-24T00:00:00Z',
        },
        {
          id: 'support-template',
          workspace_id: 'ws-1',
          team_id: 'team-2',
          name: 'Support intake',
          archived: false,
          created_at: '2026-03-24T00:00:00Z',
          updated_at: '2026-03-24T00:00:00Z',
        },
      ],
      [
        { id: 'team-1', name: 'Core' },
        { id: 'team-2', name: 'Support' },
      ],
    )

    expect(groups).toEqual([
      { options: [{ value: '', label: 'None' }] },
      { label: 'Shared', options: [{ value: 'shared-template', label: 'Shared checklist' }] },
      { label: 'Support', options: [{ value: 'support-template', label: 'Support intake' }] },
    ])
  })

  it('does not group templates from teams the user cannot access', () => {
    const groups = buildTaskTemplateSelectGroups(
      [
        {
          id: 'core-template',
          workspace_id: 'ws-1',
          team_id: 'team-1',
          name: 'Core planning',
          archived: false,
          created_at: '2026-03-24T00:00:00Z',
          updated_at: '2026-03-24T00:00:00Z',
        },
        {
          id: 'support-template',
          workspace_id: 'ws-1',
          team_id: 'team-2',
          name: 'Support intake',
          archived: false,
          created_at: '2026-03-24T00:00:00Z',
          updated_at: '2026-03-24T00:00:00Z',
        },
      ],
      [{ id: 'team-1', name: 'Core' }],
    )

    expect(groups).toEqual([
      { options: [{ value: '', label: 'None' }] },
      { label: 'Core', options: [{ value: 'core-template', label: 'Core planning' }] },
    ])
  })

  it('shows the template entry point and loads the empty state when opened with no task templates', async () => {
    const onCreate = vi.fn(async () => ({ id: 'task-1' }))
    const onOpenChange = vi.fn()
    const container = document.createElement('div')
    document.body.appendChild(container)
    const root = createRoot(container)

    await act(async () => {
      root.render(
        <CreateTaskModal
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

    const applyTemplateButton = Array.from(container.querySelectorAll('button')).find(
      (button) => button.textContent?.includes('Apply template'),
    ) as HTMLButtonElement | undefined
    expect(applyTemplateButton).toBeTruthy()
    expect(pmTaskTemplateService.list).not.toHaveBeenCalled()

    await act(async () => {
      applyTemplateButton?.click()
      await Promise.resolve()
      await Promise.resolve()
    })

    expect(container.textContent).toContain('No task templates yet')
    expect(container.textContent).toContain('Create one with this task by enabling Save as template before saving.')
    expect(findCheckboxByText(container, 'Save as template')).toBeTruthy()

    act(() => {
      root.unmount()
    })
  })

  it('explains the save-as-template footer toggle with a question mark tooltip', async () => {
    const onCreate = vi.fn(async () => ({ id: 'task-1' }))
    const onOpenChange = vi.fn()
    const container = document.createElement('div')
    document.body.appendChild(container)
    const root = createRoot(container)

    await act(async () => {
      root.render(
        <CreateTaskModal
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

    expect(container.textContent).toContain('Also save this task as a reusable template after it is created.')
    expect(container.querySelectorAll('[aria-label="Explain Save as template"]').length).toBe(1)
    expect(container.textContent).toContain('Save & create another')
    expect(container.textContent).not.toContain('Discard')

    act(() => {
      root.unmount()
    })
  })

  it('loads task templates only after opening the template entry point', async () => {
    const templatesRequest = deferred<{ data: [] }>()
    vi.mocked(pmTaskTemplateService.list).mockReturnValue(templatesRequest.promise as any)
    const onCreate = vi.fn(async () => ({ id: 'task-1' }))
    const onOpenChange = vi.fn()
    const container = document.createElement('div')
    document.body.appendChild(container)
    const root = createRoot(container)

    await act(async () => {
      root.render(
        <CreateTaskModal
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
    })

    expect(container.textContent).toContain('Apply template')
    expect(pmTaskTemplateService.list).not.toHaveBeenCalled()

    const applyTemplateButton = Array.from(container.querySelectorAll('button')).find(
      (button) => button.textContent?.includes('Apply template'),
    ) as HTMLButtonElement | undefined

    await act(async () => {
      applyTemplateButton?.click()
      await Promise.resolve()
    })

    expect(pmTaskTemplateService.list).toHaveBeenCalledWith('ws-1', { archived: false })
    expect(container.textContent).toContain('Loading task templates')
    expect(container.textContent).not.toContain('No task templates yet')

    await act(async () => {
      templatesRequest.resolve({ data: [] })
      await Promise.resolve()
    })

    act(() => {
      root.unmount()
    })
  })

  it('creates a task and saves it as a template when requested', async () => {
    const onCreate = vi.fn(async () => ({ id: 'task-1' }))
    const onOpenChange = vi.fn()
    const container = document.createElement('div')
    document.body.appendChild(container)
    const root = createRoot(container)

    await act(async () => {
      root.render(
        <CreateTaskModal
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

    await openTemplateMenu(container)

    const titleInput = container.querySelector('#task-title') as HTMLInputElement | null
    const saveAsTemplateToggle = findCheckboxByText(container, 'Save as template')
    const saveButton = Array.from(container.querySelectorAll('button')).find(
      (button) => button.textContent === 'Save',
    ) as HTMLButtonElement | undefined

    expect(titleInput).toBeTruthy()
    expect(saveAsTemplateToggle).toBeTruthy()
    expect(saveButton).toBeTruthy()

    await act(async () => {
      setInputValue(titleInput!, 'New task')
      saveAsTemplateToggle!.click()
      await Promise.resolve()
    })

    await act(async () => {
      saveButton?.click()
      await Promise.resolve()
      await Promise.resolve()
    })

    expect(onCreate).toHaveBeenCalledWith(
      expect.objectContaining({
        name: 'New task',
        workspace_id: 'ws-1',
      }),
    )
    expect(pmTaskService.saveAsTemplate).toHaveBeenCalledWith('ws-1', 'task-1', { name: 'New task' })
    expect(showEntityCreatedToast).toHaveBeenCalledWith(
      expect.objectContaining({
        entityLabel: 'Task',
        title: 'New task',
        subtitle: 'Template saved.',
      }),
    )

    act(() => {
      root.unmount()
    })
  })

  it('does not assign or run an agent by default', async () => {
    const onCreate = vi.fn(async () => ({ id: 'task-1' }))
    const onOpenChange = vi.fn()
    const container = document.createElement('div')
    document.body.appendChild(container)
    const root = createRoot(container)

    await act(async () => {
      root.render(
        <CreateTaskModal
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

    const titleInput = container.querySelector('#task-title') as HTMLInputElement | null
    const saveButton = Array.from(container.querySelectorAll('button')).find(
      (button) => button.textContent === 'Save',
    ) as HTMLButtonElement | undefined

    expect(titleInput).toBeTruthy()
    expect(saveButton).toBeTruthy()
    expect(container.textContent).toContain('No agent')

    await act(async () => {
      setInputValue(titleInput!, 'Manual task')
      await Promise.resolve()
    })

    await act(async () => {
      saveButton?.click()
      await Promise.resolve()
      await Promise.resolve()
    })

    expect(onCreate).toHaveBeenCalledWith(
      expect.not.objectContaining({
        assigned_agent_id: expect.any(String),
        run_on_create: true,
      }),
    )

    act(() => {
      root.unmount()
    })
  })

  it('shows a minimal success toast before resetting the form when saving and creating another task', async () => {
    const onCreate = vi.fn(async () => ({ id: 'task-1' }))
    const onOpenChange = vi.fn()
    const container = document.createElement('div')
    document.body.appendChild(container)
    const root = createRoot(container)

    await act(async () => {
      root.render(
        <CreateTaskModal
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

    const titleInput = container.querySelector('#task-title') as HTMLInputElement | null
    const saveAndCreateAnotherButton = Array.from(container.querySelectorAll('button')).find(
      (button) => button.textContent === 'Save & create another',
    ) as HTMLButtonElement | undefined

    expect(titleInput).toBeTruthy()
    expect(saveAndCreateAnotherButton).toBeTruthy()

    await act(async () => {
      setInputValue(titleInput!, 'New task')
      await Promise.resolve()
    })

    await act(async () => {
      saveAndCreateAnotherButton?.click()
      await Promise.resolve()
      await Promise.resolve()
    })

    expect(onCreate).toHaveBeenCalledWith(
      expect.objectContaining({
        name: 'New task',
        workspace_id: 'ws-1',
        workflow_id: 'workflow-1',
        workflow_state_id: 'state-1',
      }),
    )
    expect(showEntityCreatedToast).not.toHaveBeenCalled()
    expect(toastSuccess).toHaveBeenCalledWith('Task created. Ready for the next one.')
    expect(pmTaskService.saveAsTemplate).not.toHaveBeenCalled()

    act(() => {
      root.unmount()
    })
  })

  it('submits converted html when saving from markdown mode', async () => {
    const onCreate = vi.fn(async () => ({ id: 'task-2' }))
    const onOpenChange = vi.fn()
    const container = document.createElement('div')
    document.body.appendChild(container)
    const root = createRoot(container)

    await act(async () => {
      root.render(
        <CreateTaskModal
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

    const titleInput = container.querySelector('#task-title') as HTMLInputElement | null
    const markdownButton = Array.from(container.querySelectorAll('button')).find(
      (button) => button.textContent?.includes('Markdown'),
    ) as HTMLButtonElement | undefined

    expect(titleInput).toBeTruthy()
    expect(markdownButton).toBeTruthy()

    await act(async () => {
      setInputValue(titleInput!, 'Markdown task')
      markdownButton?.click()
      await Promise.resolve()
    })

    const markdownTextarea = container.querySelector('textarea') as HTMLTextAreaElement | null
    const saveButton = Array.from(container.querySelectorAll('button')).find(
      (button) => button.textContent === 'Save',
    ) as HTMLButtonElement | undefined

    expect(markdownTextarea).toBeTruthy()
    expect(saveButton).toBeTruthy()

    await act(async () => {
      const descriptor = Object.getOwnPropertyDescriptor(HTMLTextAreaElement.prototype, 'value')
      descriptor?.set?.call(markdownTextarea, '# Problem\n\n- first')
      markdownTextarea?.dispatchEvent(new Event('input', { bubbles: true }))
      await Promise.resolve()
    })

    await act(async () => {
      saveButton?.click()
      await Promise.resolve()
      await Promise.resolve()
    })

    expect(onCreate).toHaveBeenCalledWith(
      expect.objectContaining({
        name: 'Markdown task',
        description: expect.stringContaining('<h1>Problem</h1>'),
      }),
    )
    expect(onCreate).toHaveBeenCalledWith(
      expect.objectContaining({
        description: expect.stringContaining('<ul'),
      }),
    )

    act(() => {
      root.unmount()
    })
  })

  it('saves the selected team workflow state on task templates', async () => {
    vi.mocked(pmWorkflowService.resolveTeamWorkflow).mockResolvedValue({ data: workflow, error: null } as any)
    const onOpenChange = vi.fn()
    const container = document.createElement('div')
    document.body.appendChild(container)
    const root = createRoot(container)

    await act(async () => {
      root.render(
        <CreateTaskModal
          open
          onOpenChange={onOpenChange}
          workspaceId="ws-1"
          mode="template"
          initialTeamId="team-1"
        />,
      )
      await Promise.resolve()
      await Promise.resolve()
    })

    const titleInput = container.querySelector('#task-title') as HTMLInputElement | null
    const inProgressButton = Array.from(container.querySelectorAll('button')).find(
      (button) => button.textContent?.includes('In Progress'),
    ) as HTMLButtonElement | undefined
    const saveButton = Array.from(container.querySelectorAll('button')).find(
      (button) => button.textContent === 'Save',
    ) as HTMLButtonElement | undefined

    expect(titleInput).toBeTruthy()
    expect(inProgressButton).toBeTruthy()
    expect(saveButton).toBeTruthy()

    await act(async () => {
      setInputValue(titleInput!, 'Bug intake')
      inProgressButton?.click()
      await Promise.resolve()
    })

    await act(async () => {
      saveButton?.click()
      await Promise.resolve()
      await Promise.resolve()
    })

    expect(pmTaskTemplateService.create).toHaveBeenCalledWith(
      expect.objectContaining({
        workspace_id: 'ws-1',
        name: 'Bug intake',
        workflow_state_id: 'state-2',
      }),
    )

    act(() => {
      root.unmount()
    })
  })

  it('uploads pending files as task template attachments after saving a template', async () => {
    vi.mocked(pmWorkflowService.resolveTeamWorkflow).mockResolvedValue({ data: workflow, error: null } as any)
    const onOpenChange = vi.fn()
    const container = document.createElement('div')
    document.body.appendChild(container)
    const root = createRoot(container)

    await act(async () => {
      root.render(
        <CreateTaskModal
          open
          onOpenChange={onOpenChange}
          workspaceId="ws-1"
          mode="template"
          initialTeamId="team-1"
        />,
      )
      await Promise.resolve()
      await Promise.resolve()
    })

    const titleInput = container.querySelector('#task-title') as HTMLInputElement | null
    const attachButton = Array.from(container.querySelectorAll('button')).find(
      (button) => button.textContent?.includes('Attach Files'),
    ) as HTMLButtonElement | undefined

    expect(titleInput).toBeTruthy()
    expect(attachButton).toBeTruthy()

    await act(async () => {
      setInputValue(titleInput!, 'Template with file')
      attachButton?.click()
      await Promise.resolve()
    })

    const fileInput = container.querySelector('input[type="file"]') as HTMLInputElement | null
    const saveButton = Array.from(container.querySelectorAll('button')).find(
      (button) => button.textContent === 'Save',
    ) as HTMLButtonElement | undefined

    expect(fileInput).toBeTruthy()
    expect(saveButton).toBeTruthy()

    await act(async () => {
      Object.defineProperty(fileInput!, 'files', {
        value: [new File(['brief'], 'brief.pdf', { type: 'application/pdf' })],
        configurable: true,
      })
      fileInput?.dispatchEvent(new Event('change', { bubbles: true }))
      await Promise.resolve()
    })

    await act(async () => {
      saveButton?.click()
      await Promise.resolve()
      await Promise.resolve()
    })

    expect(pmAttachmentService.initiateUpload).toHaveBeenCalledWith(
      'ws-1',
      expect.objectContaining({
        entity_type: 'task_template',
        entity_id: 'template-1',
        file_name: 'brief.pdf',
      }),
    )
    expect(uploadToS3).toHaveBeenCalledWith(
      'https://uploads.test/attachment-1',
      expect.any(File),
      undefined,
      { 'x-amz-acl': 'public-read' },
    )
    expect(pmAttachmentService.confirmUpload).toHaveBeenCalledWith('ws-1', 'attachment-1')

    act(() => {
      root.unmount()
    })
  })

  it('applies a task template workflow state to the created task', async () => {
    vi.mocked(pmTaskTemplateService.list).mockResolvedValue({
      data: [
        {
          id: 'template-1',
          workspace_id: 'ws-1',
          team_id: 'team-1',
          name: 'Bug intake',
          workflow_state_id: 'state-2',
          created_at: '2026-03-24T00:00:00Z',
          updated_at: '2026-03-24T00:00:00Z',
        },
      ],
    } as any)
    const onCreate = vi.fn(async () => ({ id: 'task-3' }))
    const onOpenChange = vi.fn()
    const container = document.createElement('div')
    document.body.appendChild(container)
    const root = createRoot(container)

    await act(async () => {
      root.render(
        <CreateTaskModal
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

    const titleInput = container.querySelector('#task-title') as HTMLInputElement | null
    await openTemplateMenu(container)

    const templateButton = Array.from(container.querySelectorAll('button')).find(
      (button) => button.textContent?.includes('Bug intake'),
    ) as HTMLButtonElement | undefined
    const saveButton = Array.from(container.querySelectorAll('button')).find(
      (button) => button.textContent === 'Save',
    ) as HTMLButtonElement | undefined

    expect(titleInput).toBeTruthy()
    expect(templateButton).toBeTruthy()
    expect(saveButton).toBeTruthy()

    await act(async () => {
      setInputValue(titleInput!, 'Task from template')
      templateButton?.click()
      await Promise.resolve()
    })

    await act(async () => {
      saveButton?.click()
      await Promise.resolve()
      await Promise.resolve()
    })

    expect(onCreate).toHaveBeenCalledWith(
      expect.objectContaining({
        name: 'Task from template',
        workflow_id: 'workflow-1',
        workflow_state_id: 'state-2',
        template_id: 'template-1',
      }),
    )

    act(() => {
      root.unmount()
    })
  })

  it('uses the template name as the default task title when the title is empty', async () => {
    vi.mocked(pmTaskTemplateService.list).mockResolvedValue({
      data: [
        {
          id: 'template-1',
          workspace_id: 'ws-1',
          team_id: 'team-1',
          name: 'Bug intake',
          workflow_state_id: 'state-2',
          created_at: '2026-03-24T00:00:00Z',
          updated_at: '2026-03-24T00:00:00Z',
        },
      ],
    } as any)
    const onCreate = vi.fn(async () => ({ id: 'task-4' }))
    const onOpenChange = vi.fn()
    const container = document.createElement('div')
    document.body.appendChild(container)
    const root = createRoot(container)

    await act(async () => {
      root.render(
        <CreateTaskModal
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

    await openTemplateMenu(container)

    const templateButton = Array.from(container.querySelectorAll('button')).find(
      (button) => button.textContent?.includes('Bug intake'),
    ) as HTMLButtonElement | undefined
    const saveButton = Array.from(container.querySelectorAll('button')).find(
      (button) => button.textContent === 'Save',
    ) as HTMLButtonElement | undefined

    expect(templateButton).toBeTruthy()
    expect(saveButton).toBeTruthy()

    await act(async () => {
      templateButton?.click()
      await Promise.resolve()
    })

    await act(async () => {
      saveButton?.click()
      await Promise.resolve()
      await Promise.resolve()
    })

    expect(onCreate).toHaveBeenCalledWith(
      expect.objectContaining({
        name: 'Bug intake',
        template_id: 'template-1',
      }),
    )

    act(() => {
      root.unmount()
    })
  })

  it('applies a cross-team template placement and saved task fields', async () => {
    vi.mocked(pmLabelService.list).mockResolvedValue({
      data: [
        { id: 'label-1', workspace_id: 'ws-1', team_id: 'team-2', name: 'Escalated', color: '#ef4444', archived: false },
        { id: 'label-2', workspace_id: 'ws-1', name: 'Customer', color: '#3b82f6', archived: false },
      ],
    } as any)
    vi.mocked(pmTaskTemplateService.list).mockResolvedValue({
      data: [
        {
          id: 'template-support',
          workspace_id: 'ws-1',
          team_id: 'team-2',
          name: 'Support intake',
          description: '<p>Support handoff</p>',
          task_type: 'bug',
          priority: 'high',
          severity: 'major',
          estimate: 3,
          label_ids: JSON.stringify(['label-1', 'label-2']),
          owner_member_id: 'owner-1',
          epic_id: 'epic-1',
          sprint_id: 'sprint-1',
          workflow_state_id: 'support-state-triage',
          deadline: '2026-06-01',
          checklist_items: JSON.stringify([{ text: 'Check logs', position: 0 }]),
          external_links: JSON.stringify([{ url: 'https://example.com/ticket' }]),
          created_at: '2026-03-24T00:00:00Z',
          updated_at: '2026-03-24T00:00:00Z',
        },
      ],
    } as any)
    vi.mocked(pmWorkflowService.resolveTeamWorkflow).mockImplementation(async (_workspaceId, teamId) => {
      if (teamId === 'team-2') return { data: supportWorkflow, error: null } as any
      return { data: workflow, error: null } as any
    })
    const onCreate = vi.fn(async () => ({ id: 'task-5' }))
    const onOpenChange = vi.fn()
    const container = document.createElement('div')
    document.body.appendChild(container)
    const root = createRoot(container)

    await act(async () => {
      root.render(
        <CreateTaskModal
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

    await openTemplateMenu(container)

    const templateButton = Array.from(container.querySelectorAll('button')).find(
      (button) => button.textContent?.includes('Support intake'),
    ) as HTMLButtonElement | undefined
    const saveButton = Array.from(container.querySelectorAll('button')).find(
      (button) => button.textContent === 'Save',
    ) as HTMLButtonElement | undefined

    expect(templateButton).toBeTruthy()
    expect(saveButton).toBeTruthy()

    await act(async () => {
      templateButton?.click()
      await Promise.resolve()
    })

    await act(async () => {
      saveButton?.click()
      await Promise.resolve()
      await Promise.resolve()
    })

    expect(onCreate).toHaveBeenCalledWith(
      expect.objectContaining({
        name: 'Support intake',
        description: '<p>Support handoff</p>',
        task_type: 'bug',
        workflow_id: 'workflow-2',
        workflow_state_id: 'support-state-triage',
        template_id: 'template-support',
        priority: 'high',
        severity: 'major',
        estimate: 3,
        epic_id: 'epic-1',
        sprint_id: 'sprint-1',
        team_id: 'team-2',
        owner_member_ids: ['owner-1'],
        deadline: '2026-06-01',
        label_ids: ['label-1', 'label-2'],
        checklist_items: [{ text: 'Check logs', position: 0 }],
        external_links: [{ url: 'https://example.com/ticket' }],
      }),
    )

    act(() => {
      root.unmount()
    })
  })

  it('shows a cross-team template workflow state after applying the template', async () => {
    vi.mocked(pmTaskTemplateService.list).mockResolvedValue({
      data: [
        {
          id: 'template-support',
          workspace_id: 'ws-1',
          team_id: 'team-2',
          name: 'Support intake',
          workflow_state_id: 'support-state-triage',
          archived: false,
          created_at: '2026-03-24T00:00:00Z',
          updated_at: '2026-03-24T00:00:00Z',
        },
      ],
    } as any)
    vi.mocked(pmWorkflowService.resolveTeamWorkflow).mockImplementation(async (_workspaceId, teamId) => {
      if (teamId === 'team-2') return { data: supportWorkflow, error: null } as any
      return { data: workflow, error: null } as any
    })
    const onCreate = vi.fn(async () => ({ id: 'task-6' }))
    const onOpenChange = vi.fn()
    const container = document.createElement('div')
    document.body.appendChild(container)
    const root = createRoot(container)

    await act(async () => {
      root.render(
        <CreateTaskModal
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

    await openTemplateMenu(container)

    const templateButton = Array.from(container.querySelectorAll('button')).find(
      (button) => button.textContent?.includes('Support intake'),
    ) as HTMLButtonElement | undefined

    expect(templateButton).toBeTruthy()

    await act(async () => {
      templateButton?.click()
      await Promise.resolve()
      await Promise.resolve()
      await Promise.resolve()
    })

    expect(Array.from(container.querySelectorAll('button')).some((button) => button.textContent?.includes('Triage'))).toBe(true)

    act(() => {
      root.unmount()
    })
  })

  it('keeps a cross-team template task type instead of replacing it with the team default', async () => {
    vi.mocked(pmTaskTemplateService.list).mockResolvedValue({
      data: [
        {
          id: 'template-support',
          workspace_id: 'ws-1',
          team_id: 'team-2',
          name: 'Support chore',
          task_type: 'chore',
          workflow_state_id: 'support-state-triage',
          archived: false,
          created_at: '2026-03-24T00:00:00Z',
          updated_at: '2026-03-24T00:00:00Z',
        },
      ],
    } as any)
    vi.mocked(pmWorkflowService.resolveTeamWorkflow).mockImplementation(async (_workspaceId, teamId) => {
      if (teamId === 'team-2') return { data: supportWorkflow, error: null } as any
      return { data: workflow, error: null } as any
    })
    const onCreate = vi.fn(async () => ({ id: 'task-7' }))
    const onOpenChange = vi.fn()
    const container = document.createElement('div')
    document.body.appendChild(container)
    const root = createRoot(container)

    await act(async () => {
      root.render(
        <CreateTaskModal
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

    await openTemplateMenu(container)

    const templateButton = Array.from(container.querySelectorAll('button')).find(
      (button) => button.textContent?.includes('Support chore'),
    ) as HTMLButtonElement | undefined
    const saveButton = Array.from(container.querySelectorAll('button')).find(
      (button) => button.textContent === 'Save',
    ) as HTMLButtonElement | undefined

    expect(templateButton).toBeTruthy()
    expect(saveButton).toBeTruthy()

    await act(async () => {
      templateButton?.click()
      await Promise.resolve()
      await Promise.resolve()
      await Promise.resolve()
    })

    await act(async () => {
      saveButton?.click()
      await Promise.resolve()
      await Promise.resolve()
    })

    expect(onCreate).toHaveBeenCalledWith(
      expect.objectContaining({
        task_type: 'chore',
      }),
    )

    act(() => {
      root.unmount()
    })
  })
})
