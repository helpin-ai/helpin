import { createFileRoute } from '@tanstack/react-router'
import { RefreshCw } from 'lucide-react'

export const Route = createFileRoute('/_authenticated/w/$slug/pm/iterations')({
  component: Iterations,
})

function Iterations() {
  return (
    <div className="flex flex-col items-center justify-center py-24 text-center">
      <RefreshCw className="h-12 w-12 text-muted-foreground/40" />
      <h2 className="mt-4 text-xl font-semibold">Iterations</h2>
      <p className="mt-2 text-sm text-muted-foreground">
        Coming Soon — Iteration planning and sprint cycles are being built.
      </p>
    </div>
  )
}
