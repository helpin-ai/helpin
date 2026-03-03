import { createFileRoute } from '@tanstack/react-router'
import { FileText } from 'lucide-react'

export const Route = createFileRoute('/_authenticated/w/$slug/docs')({
  component: Docs,
})

function Docs() {
  return (
    <div className="flex flex-col items-center justify-center py-24 text-center">
      <FileText className="h-12 w-12 text-muted-foreground/40" />
      <h2 className="mt-4 text-xl font-semibold">Documentation</h2>
      <p className="mt-2 text-sm text-muted-foreground">
        Coming Soon — Documentation features are being built.
      </p>
    </div>
  )
}
