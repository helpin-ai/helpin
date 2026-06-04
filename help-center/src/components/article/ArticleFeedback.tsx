import { useState, useCallback } from 'react'
import { ThumbsUp, ThumbsDown, Check } from 'lucide-react'
import { helpCenterService } from '@/lib/services'
import { useDocsContext } from '@/contexts/DocsContext'
import { buildArticleKey } from '@/lib/articleKey'

interface ArticleFeedbackProps {
  locale: string
  articleSlug: string
  articlePublicId: string
  multilingualEnabled: boolean
}

export function ArticleFeedback({
  locale,
  articleSlug,
  articlePublicId,
  multilingualEnabled,
}: ArticleFeedbackProps) {
  const { subdomain } = useDocsContext()
  const [submitted, setSubmitted] = useState<boolean | null>(null)

  const handleFeedback = useCallback(
    async (isHelpful: boolean) => {
      if (submitted !== null) return
      if (!articleSlug || !articlePublicId) return
      setSubmitted(isHelpful)
      try {
        await helpCenterService.submitFeedback(
          subdomain,
          locale,
          buildArticleKey(articleSlug, articlePublicId),
          multilingualEnabled,
          {
            is_helpful: isHelpful,
          },
        )
      } catch {
        // Feedback is best-effort
      }
    },
    [subdomain, locale, articleSlug, articlePublicId, multilingualEnabled, submitted],
  )

  return (
    <div className="mt-12 border-t border-border pt-6 text-center">
      {submitted !== null ? (
        <div className="flex items-center justify-center gap-2 text-sm text-muted-foreground">
          <Check size={16} className="text-green-600" />
          Thanks for your feedback!
        </div>
      ) : (
        <>
          <p className="text-[13px] text-muted-foreground mb-3">
            Was this article helpful?
          </p>
          <div className="flex justify-center gap-2">
            <button
              onClick={() => handleFeedback(true)}
              disabled={!articleSlug || !articlePublicId}
              className="inline-flex items-center gap-1.5 rounded-md border border-border px-3 py-1.5 text-[13px] text-muted-foreground hover:text-foreground hover:bg-muted/60 transition-colors"
            >
              <ThumbsUp size={13} />
              Yes
            </button>
            <button
              onClick={() => handleFeedback(false)}
              disabled={!articleSlug || !articlePublicId}
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
