import { createFileRoute } from '@tanstack/react-router'
import { BarChart3 } from 'lucide-react'

export const Route = createFileRoute('/_authenticated/w/$slug/pm/reports')({
  component: Reports,
})

function Reports() {
  return (
    <div className="flex flex-col items-center justify-center py-24 text-center">
      <BarChart3 className="h-12 w-12 text-muted-foreground/40" />
      <h2 className="mt-4 text-xl font-semibold">Reports</h2>
      <p className="mt-2 text-sm text-muted-foreground">
        Coming Soon — Project reporting and analytics features are being built.
      </p>
    </div>
  )
}
