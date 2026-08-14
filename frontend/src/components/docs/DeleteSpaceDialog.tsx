import type { ReactNode } from 'react'
import { useDocsSpaceDeleteImpact } from '@/hooks/queries'
import { TypedConfirmDialog } from '@/components/docs/TypedConfirmDialog'

interface DeleteSpaceDialogProps {
  wsId: string
  space: { id: string; name: string } | null
  open: boolean
  onOpenChange: (open: boolean) => void
  onConfirm: () => Promise<void> | void
}

/**
 * Unified confirmation dialog for deleting a docs space. Used from
 * both the sidebar and the collection page. Loads the cascade impact
 * (collection + document counts + public-article count) and presents
 * a clear summary of everything that will be removed. Confirmation
 * requires typing the space name.
 *
 * Works for both internal and external spaces: internal spaces will
 * always have `public_document_count = 0`, so the "published to the
 * help center" callout is only rendered when relevant.
 */
export function DeleteSpaceDialog({
  wsId,
  space,
  open,
  onOpenChange,
  onConfirm,
}: DeleteSpaceDialogProps) {
  const { data: impact, isLoading, isError } = useDocsSpaceDeleteImpact(wsId, space?.id)

  const description: ReactNode = isLoading
    ? 'Calculating what will be deleted...'
    : isError
    ? 'Could not calculate the delete impact. Close this dialog and try again.'
    : !impact
    ? (
        <>
          <strong>"{space?.name ?? 'this space'}"</strong> and everything inside it
          will be permanently deleted. <strong>This cannot be undone.</strong>
        </>
      )
    : renderImpactDescription(space?.name ?? 'this space', impact)

  return (
    <TypedConfirmDialog
      open={open}
      onOpenChange={onOpenChange}
      title="Delete space"
      description={description}
      confirmText={space?.name ?? ''}
      disabled={isLoading || isError}
      onConfirm={onConfirm}
    />
  )
}

function renderImpactDescription(
  name: string,
  impact: {
    collection_count: number
    document_count: number
    public_document_count: number
    api_reference_count: number
  },
): ReactNode {
  const { collection_count, document_count, public_document_count, api_reference_count } = impact
  const hasCollections = collection_count > 0
  const hasDocs = document_count > 0
  const hasPublic = public_document_count > 0
  const hasAPIReferences = api_reference_count > 0

  if (!hasCollections && !hasDocs && !hasAPIReferences) {
    return (
      <>
        <strong>"{name}"</strong> will be permanently deleted.{' '}
        <strong>This cannot be undone.</strong>
      </>
    )
  }

  const parts: ReactNode[] = []
  if (hasCollections) {
    parts.push(
      <strong>
        {collection_count} collection{collection_count === 1 ? '' : 's'}
      </strong>,
    )
  }
  if (hasDocs) {
    parts.push(
      <>
        <strong>
          {document_count} document{document_count === 1 ? '' : 's'}
        </strong>
        {hasPublic && (
          <>
            {' '}
            <span className="text-amber-700">
              ({public_document_count} published to the help center)
            </span>
          </>
        )}
      </>,
    )
  }
  if (hasAPIReferences) {
    parts.push(
      <strong>
        {api_reference_count} API reference{api_reference_count === 1 ? '' : 's'}
      </strong>,
    )
  }

  return (
    <>
      <strong>"{name}"</strong> will be permanently deleted, along with{' '}
      {joinParts(parts)}. <strong>This cannot be undone.</strong>
    </>
  )
}

function joinParts(nodes: ReactNode[]): ReactNode {
  if (nodes.length === 0) return null
  if (nodes.length === 1) return nodes[0]
  if (nodes.length === 3) {
    return (
      <>
        {nodes[0]}, {nodes[1]}, and {nodes[2]}
      </>
    )
  }
  return (
    <>
      {nodes[0]} and {nodes[1]}
    </>
  )
}
