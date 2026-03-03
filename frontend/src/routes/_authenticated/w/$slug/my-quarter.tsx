import { createFileRoute } from '@tanstack/react-router'
import MyQuarter from '@/pages/MyQuarter'

export const Route = createFileRoute('/_authenticated/w/$slug/my-quarter')({
  component: MyQuarter,
})
