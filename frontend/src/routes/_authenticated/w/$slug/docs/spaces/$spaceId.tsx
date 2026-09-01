import { createFileRoute } from '@tanstack/react-router'
import { z } from 'zod'
import { DocsSpaceDetail } from '@/pages/docs/DocsSpaceDetail'

// `collection` drives which node the space-detail page renders:
// - omitted → space root view
// - '__uncollected__' → dedicated uncategorized view
// - any other string → collection UUID (unknown IDs fall back to
//   space root via resolveView + the deep-link guard in the page).
const spaceDetailSearchSchema = z.object({
  collection: z.string().optional(),
})

export const Route = createFileRoute('/_authenticated/w/$slug/docs/spaces/$spaceId')({
  validateSearch: spaceDetailSearchSchema,
  component: DocsSpaceDetail,
})
