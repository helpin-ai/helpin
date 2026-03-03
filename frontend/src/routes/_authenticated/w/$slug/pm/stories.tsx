import { createFileRoute } from '@tanstack/react-router'
import { LayoutList } from 'lucide-react'

export const Route = createFileRoute('/_authenticated/w/$slug/pm/stories')({
  component: Stories,
})

function Stories() {
  return (
    <div className="flex flex-col items-center justify-center py-24 text-center">
      <LayoutList className="h-12 w-12 text-muted-foreground/40" />
      <h2 className="mt-4 text-xl font-semibold">Stories</h2>
      <p className="mt-2 text-sm text-muted-foreground">
        Coming Soon — Story tracking and management features are being built.
      </p>
    </div>
  )
}
