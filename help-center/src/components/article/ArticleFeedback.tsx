import { useState, useCallback } from 'react'
import { ThumbsUp, ThumbsDown, Check } from 'lucide-react'
import { helpCenterService } from '@/lib/services'
import { useDocsContext } from '@/contexts/DocsContext'

interface ArticleFeedbackProps {
  locale: string
  collectionSlug?: string | null
  articleSlug: string
}

export function ArticleFeedback({
  locale,
  collectionSlug,
  articleSlug,
}: ArticleFeedbackProps) {
  const { subdomain } = useDocsContext()
  const [submitted, setSubmitted] = useState<boolean | null>(null)

  const handleFeedback = useCallback(
    async (isHelpful: boolean) => {
      if (submitted !== null) return
      if (!collectionSlug) return
      setSubmitted(isHelpful)
      try {
        await helpCenterService.submitFeedback(
          subdomain,
          locale,
          collectionSlug,
          articleSlug,
          {
            is_helpful: isHelpful,
          },
        )
      } catch {
        // Feedback is best-effort
      }
    },
    [subdomain, locale, collectionSlug, articleSlug, submitted],
  )

  return (
    <div className="mt-12 pt-6 border-t border-border">
      {submitted !== null ? (
        <div className="flex items-center gap-2 text-sm text-muted-foreground">
          <Check size={16} className="text-green-600" />
          Thanks for your feedback!
        </div>
      ) : (
        <>
          <p className="text-[13px] text-muted-foreground mb-3">
            Was this article helpful?
          </p>
          <div className="flex gap-2">
            <button
              onClick={() => handleFeedback(true)}
              disabled={!collectionSlug}
              className="inline-flex items-center gap-1.5 rounded-md border border-border px-3 py-1.5 text-[13px] text-muted-foreground hover:text-foreground hover:bg-muted/60 transition-colors"
            >
              <ThumbsUp size={13} />
              Yes
            </button>
            <button
              onClick={() => handleFeedback(false)}
              disabled={!collectionSlug}
              className="inline-flex items-center gap-1.5 rounded-md border border-border px-3 py-1.5 text-[13px] text-muted-foreground hover:text-foreground hover:bg-muted/60 transition-colors"
            >
              <ThumbsDown size={13} />
              No
            </button>
          </div>
        </>
      )}
    </div>
  )
}
