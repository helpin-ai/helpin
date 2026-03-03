import { createFileRoute } from '@tanstack/react-router'
import Workspaces from '@/pages/Workspaces'

export const Route = createFileRoute('/_authenticated/workspaces')({
  component: Workspaces,
})
