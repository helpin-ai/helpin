import { createFileRoute } from '@tanstack/react-router'
import { ChartColumnIcon } from '@/lib/icons'
import { useTitle } from '@/hooks/useTitle'

export const Route = createFileRoute('/_authenticated/w/$slug/pm/reports')({
  component: Reports,
})

function Reports() {
  useTitle('Reports')
  return (
    <div className="flex h-full flex-col items-center justify-center p-4 text-center md:p-6">
      <ChartColumnIcon className="h-12 w-12 text-muted-foreground/40" />
      <h2 className="mt-4 text-xl font-semibold">Reports</h2>
      <p className="mt-2 text-sm text-muted-foreground">
        Coming Soon — Project reporting and analytics features are being built.
      </p>
    </div>
  )
}
