import type { ReactNode } from 'react'
import { useDocsCollectionDeleteImpact } from '@/hooks/queries'
import { TypedConfirmDialog } from '@/components/docs/TypedConfirmDialog'

interface DeleteCollectionDialogProps {
  wsId: string
  collection: { id: string; name: string; space_id: string } | null
  open: boolean
  onOpenChange: (open: boolean) => void
  onConfirm: () => Promise<void> | void
}

/**
 * Unified confirmation dialog for deleting a docs collection. Used from
 * both the sidebar and the collection page. Loads the cascade impact
 * (sub-collection + document counts) via GetCollectionDeleteImpact and
 * presents a clear summary of what will be deleted. Confirmation
 * requires typing the collection name.
 *
 * The backend's `collection_count` is the size of the subtree INCLUDING
 * the target collection itself, so we subtract one to show the
 * sub-collection count in the copy.
 */
export function DeleteCollectionDialog({
  wsId,
  collection,
  open,
  onOpenChange,
  onConfirm,
}: DeleteCollectionDialogProps) {
  const { data: impact, isLoading, isError } = useDocsCollectionDeleteImpact(wsId, collection?.id)

  const description: ReactNode = isLoading
    ? 'Calculating what will be deleted...'
    : isError
    ? 'Could not calculate the delete impact. Close this dialog and try again.'
    : !impact
    ? (
        <>
          <strong>"{collection?.name ?? 'this collection'}"</strong> and everything inside it
          will be permanently deleted. <strong>This cannot be undone.</strong>
        </>
      )
    : renderImpactDescription(collection?.name ?? 'this collection', impact)

  return (
    <TypedConfirmDialog
      open={open}
      onOpenChange={onOpenChange}
      title="Delete collection"
      description={description}
      confirmText={collection?.name ?? ''}
      disabled={isLoading || isError}
      onConfirm={onConfirm}
    />
  )
}

/**
 * Render the impact summary. `collection_count` includes the target,
 * so sub-collections = collection_count - 1.
 */
function renderImpactDescription(
  name: string,
  impact: {
    collection_count: number
    document_count: number
    public_document_count: number
  },
): ReactNode {
  const subCollectionCount = Math.max(0, impact.collection_count - 1)
  const subLabel = subCollectionCount === 1 ? 'sub-collection' : 'sub-collections'
  const docLabel = impact.document_count === 1 ? 'document' : 'documents'

  const hasSubs = subCollectionCount > 0
  const hasDocs = impact.document_count > 0
  const hasPublic = impact.public_document_count > 0

  // Simplest case: an empty collection.
  if (!hasSubs && !hasDocs) {
    return (
      <>
        <strong>"{name}"</strong> will be permanently deleted.{' '}
        <strong>This cannot be undone.</strong>
      </>
    )
  }

  const contents: ReactNode[] = []
  if (hasSubs) {
    contents.push(
      <>
        <strong>
          {subCollectionCount} {subLabel}
        </strong>
      </>,
    )
  }
  if (hasDocs) {
    contents.push(
      <>
        <strong>
          {impact.document_count} {docLabel}
        </strong>
        {hasPublic && (
          <>
            {' '}
            <span className="text-amber-700">
              ({impact.public_document_count} published to the help center)
            </span>
          </>
        )}
      </>,
    )
  }

  return (
    <>
      <strong>"{name}"</strong> will be permanently deleted, along with{' '}
      {joinContents(contents)}. <strong>This cannot be undone.</strong>
    </>
  )
}

function joinContents(nodes: ReactNode[]): ReactNode {
  if (nodes.length === 0) return null
  if (nodes.length === 1) return nodes[0]
  if (nodes.length === 2) return (
    <>
      {nodes[0]} and {nodes[1]}
    </>
  )
  // 3+ not expected, but handle gracefully.
  return nodes.map((n, i) => (
    <span key={i}>
      {n}
      {i < nodes.length - 1 ? (i === nodes.length - 2 ? ', and ' : ', ') : ''}
    </span>
  ))
}
