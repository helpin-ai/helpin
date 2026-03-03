import { createFileRoute } from '@tanstack/react-router'
import { Target } from 'lucide-react'

export const Route = createFileRoute('/_authenticated/w/$slug/pm/objectives')({
  component: Objectives,
})

function Objectives() {
  return (
    <div className="flex flex-col items-center justify-center py-24 text-center">
      <Target className="h-12 w-12 text-muted-foreground/40" />
      <h2 className="mt-4 text-xl font-semibold">Objectives</h2>
      <p className="mt-2 text-sm text-muted-foreground">
        Coming Soon — OKR and objective tracking features are being built.
      </p>
    </div>
  )
}
