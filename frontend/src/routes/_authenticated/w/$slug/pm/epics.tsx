import { createFileRoute } from '@tanstack/react-router'
import { Layers } from 'lucide-react'

export const Route = createFileRoute('/_authenticated/w/$slug/pm/epics')({
  component: Epics,
})

function Epics() {
  return (
    <div className="flex flex-col items-center justify-center py-24 text-center">
      <Layers className="h-12 w-12 text-muted-foreground/40" />
      <h2 className="mt-4 text-xl font-semibold">Epics</h2>
      <p className="mt-2 text-sm text-muted-foreground">
        Coming Soon — Epic planning and hierarchy features are being built.
      </p>
    </div>
  )
}
