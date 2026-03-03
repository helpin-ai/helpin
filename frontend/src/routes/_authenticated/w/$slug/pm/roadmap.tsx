import { createFileRoute } from '@tanstack/react-router'
import { GanttChart } from 'lucide-react'

export const Route = createFileRoute('/_authenticated/w/$slug/pm/roadmap')({
  component: Roadmap,
})

function Roadmap() {
  return (
    <div className="flex flex-col items-center justify-center py-24 text-center">
      <GanttChart className="h-12 w-12 text-muted-foreground/40" />
      <h2 className="mt-4 text-xl font-semibold">Roadmap</h2>
      <p className="mt-2 text-sm text-muted-foreground">
        Coming Soon — Roadmap visualization and timeline features are being built.
      </p>
    </div>
  )
}
