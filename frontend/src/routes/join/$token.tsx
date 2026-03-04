import { createFileRoute } from '@tanstack/react-router'
import JoinWorkspace from '@/pages/JoinWorkspace'

export const Route = createFileRoute('/join/$token')({
  component: JoinWorkspace,
})
