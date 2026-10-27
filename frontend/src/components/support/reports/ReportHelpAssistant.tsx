import { QuietMetricBlock, QuietMetricGrid, QuietSection } from '@/components/design-system/quiet'
import type { SupportReportHelpAssistant } from '@/lib/supportReportTypes'
import { timeAgo } from '@/lib/utils'

const plural = (count: number, one: string, many: string) => `${count} ${count === 1 ? one : many}`
const percent = (part: number, whole: number) => (whole > 0 ? `${Math.round((part / whole) * 100)}%` : '—')

/**
 * ReportHelpAssistant shows how the Ask AI help assistant did: how many
 * questions the help articles answered, how many needed the team, and which
 * questions went unanswered, so the articles can be improved.
 */
export function ReportHelpAssistant({ assistant, workspaceSlug }: { assistant: SupportReportHelpAssistant; workspaceSlug: string }) {
  const votes = assistant.helpful + assistant.not_helpful
  return (
    <QuietSection title="Help assistant" count={plural(assistant.conversations, 'conversation', 'conversations')}>
      <QuietMetricGrid>
        <QuietMetricBlock
          className="items-start"
          label="Answered from the articles"
          value={percent(assistant.answered, assistant.questions)}
          description={`${assistant.answered} of ${plural(assistant.questions, 'question', 'questions')}`}
        />
        <QuietMetricBlock
          className="items-start"
          label="Offered support"
          value={String(assistant.handed_off)}
          description={
            <>
              The articles did not answer.{' '}
              <a className="underline hover:text-quiet-text-primary" href={`/w/${workspaceSlug}/support/coverage`}>Coverage gaps</a> collects them.
            </>
          }
        />
        <QuietMetricBlock
          className="items-start"
          label="Rated helpful"
          value={percent(assistant.helpful, votes)}
          description={votes ? `${plural(votes, 'rating', 'ratings')}` : 'No ratings yet'}
        />
        <QuietMetricBlock
          className="items-start"
          label="Could not answer"
          value={String(assistant.failed)}
          description="Questions the assistant failed to finish"
        />
      </QuietMetricGrid>
      {assistant.unanswered.length > 0 ? (
        <div className="mt-4">
          <p className="text-[12px] font-medium text-quiet-text-tertiary">Recent questions the articles did not answer</p>
          <ul className="mt-2 divide-y divide-quiet-divider-light border-y border-quiet-divider-light">
            {assistant.unanswered.map((item) => (
              <li key={`${item.asked_at}-${item.question}`} className="flex items-baseline justify-between gap-4 py-2 text-sm">
                <span className="min-w-0 break-words">{item.question}</span>
                <span className="flex-none text-[12px] text-quiet-text-tertiary">{timeAgo(item.asked_at)}</span>
              </li>
            ))}
          </ul>
        </div>
      ) : null}
    </QuietSection>
  )
}
