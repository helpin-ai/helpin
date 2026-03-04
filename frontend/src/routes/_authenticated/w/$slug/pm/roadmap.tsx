import { createFileRoute } from '@tanstack/react-router'
import { GanttChart } from 'lucide-react'
import { useTitle } from '@/hooks/useTitle'

export const Route = createFileRoute('/_authenticated/w/$slug/pm/roadmap')({
  component: Roadmap,
})

function Roadmap() {
  useTitle('Roadmap')
  return (
    <div className="flex h-full flex-col items-center justify-center p-4 text-center md:p-6">
      <GanttChart className="h-12 w-12 text-muted-foreground/40" />
      <h2 className="mt-4 text-xl font-semibold">Roadmap</h2>
      <p className="mt-2 text-sm text-muted-foreground">
        Coming Soon — Roadmap visualization and timeline features are being built.
      </p>
    </div>
  )
}
