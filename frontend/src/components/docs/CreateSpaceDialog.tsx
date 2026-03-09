import { SpaceDialog } from './SpaceDialog'

interface CreateSpaceDialogProps {
  wsId: string
  open: boolean
  onOpenChange: (open: boolean) => void
}

export function CreateSpaceDialog({ wsId, open, onOpenChange }: CreateSpaceDialogProps) {
  return <SpaceDialog wsId={wsId} open={open} onOpenChange={onOpenChange} />
}
