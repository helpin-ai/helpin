import { QuietMetricBlock, QuietMetricGrid, QuietSection } from '@/components/design-system/quiet'
import type { SupportReportPortalIntake } from '@/lib/supportReportTypes'

const plural = (count: number, one: string, many: string) => `${count} ${count === 1 ? one : many}`

/**
 * ReportPortalIntake shows what customers did with the help center articles
 * suggested while they wrote portal requests: how often an article likely
 * answered them, and how often they found nothing and asked the team.
 */
export function ReportPortalIntake({ intake, workspaceSlug }: { intake: SupportReportPortalIntake; workspaceSlug: string }) {
  return (
    <QuietSection title="Portal self-service">
      <QuietMetricGrid>
        <QuietMetricBlock
          className="items-start"
          label="Likely answered by an article"
          value={String(intake.likely_deflected)}
          description="Opened a suggested article and sent no request within 30 minutes"
        />
        <QuietMetricBlock
          className="items-start"
          label="Opened an article"
          value={String(intake.opened)}
          description={`Of ${plural(intake.with_suggestions, 'form', 'forms')} that showed suggestions`}
        />
        <QuietMetricBlock
          className="items-start"
          label="Asked after reading"
          value={String(intake.submitted_after_opening)}
          description="Sent a request after opening an article"
        />
        <QuietMetricBlock
          className="items-start"
          label="Nothing to suggest"
          value={String(intake.submitted_without_suggestions)}
          description={
            <>
              Requests sent when no article matched.{' '}
              <a className="underline hover:text-quiet-text-primary" href={`/w/${workspaceSlug}/support/coverage`}>Coverage gaps</a> collects them.
            </>
          }
        />
      </QuietMetricGrid>
    </QuietSection>
  )
}
