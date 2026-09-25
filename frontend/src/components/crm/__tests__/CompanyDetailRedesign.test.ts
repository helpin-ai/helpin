import { readFileSync } from 'node:fs'
import { resolve } from 'node:path'
import { describe, expect, it } from 'vitest'

const companyDetailSource = readFileSync(resolve(__dirname, '../../../pages/crm/CompanyDetail.tsx'), 'utf8')
const createDealSource = readFileSync(resolve(__dirname, '../CreateDealDialog.tsx'), 'utf8')
const enrichmentSource = readFileSync(resolve(__dirname, '../contact-detail/EnrichmentRailCard.tsx'), 'utf8')
const linkedTasksSource = readFileSync(resolve(__dirname, '../LinkedTasksPanel.tsx'), 'utf8')
const companyTasksWorkspaceSource = readFileSync(resolve(__dirname, '../CompanyTasksWorkspace.tsx'), 'utf8')
const companyCollectionsSource = readFileSync(resolve(__dirname, '../CompanyDetailCollections.tsx'), 'utf8')
const emailTimelineSource = readFileSync(resolve(__dirname, '../EmailTimeline.tsx'), 'utf8')

describe('Company detail divider redesign', () => {
  it('uses the task-detail rail scale and scoped borderless surfaces', () => {
    expect(companyDetailSource).toContain('lg:grid-cols-[minmax(0,1fr)_300px]')
    expect(companyDetailSource).toContain('grid-cols-[16px_72px_1fr]')
    expect(companyDetailSource).toContain('<RichTextMentionContent')
    expect(companyDetailSource).toContain('<DetailDescriptionEditorActions')
    expect(companyDetailSource).toContain('<DetailDescriptionEditButton')
    expect(companyDetailSource).toContain('presentation="borderless"')
  })

  it('retains the existing association rail presentation', () => {
    const associationUsage = companyDetailSource.slice(
      companyDetailSource.indexOf('<AssociationsList'),
      companyDetailSource.indexOf('/>', companyDetailSource.indexOf('<AssociationsList')) + 2,
    )

    expect(associationUsage).not.toContain('presentation=')
    expect(associationUsage).toContain('currentObjectType="company"')
  })

  it('uses the unified cursor-paginated company activity timeline', () => {
    expect(companyDetailSource).toContain('useCompanyTimeline')
    expect(companyDetailSource).toContain('timeline.data?.pages.flatMap')
    expect(companyDetailSource).toContain('onTimelineFilterChange={setActivityFilter}')
    expect(companyDetailSource).toContain('onLoadMore={() => void timeline.fetchNextPage()}')
    expect(companyDetailSource).toContain('filterControl="dropdown"')
  })

  it('uses dedicated company tabs with a collapsible desktop detail rail', () => {
    for (const tab of ['Overview', 'Notes', 'Emails', 'Calls', 'Meetings', 'Tasks', 'Deals', 'Support']) {
      expect(companyDetailSource).toContain(`label: '${tab}'`)
    }
    expect(companyDetailSource).toContain('<CompanyContactsView')
    expect(companyDetailSource).toContain('<CompanyDealsView')
    expect(companyDetailSource).toContain('<CompanyMeetingsView')
    expect(companyDetailSource).toContain('<CompanySupportView')
    expect(companyDetailSource).toContain('<DesktopDetailRail')
    expect(companyDetailSource).toContain("setDesktopDetailsCollapsed(activeTab !== 'overview')")
    expect(companyDetailSource).toContain('lg:grid-cols-[minmax(0,1fr)_40px]')
    expect(companyDetailSource).not.toContain("value: 'contacts'")
    expect(companyDetailSource.indexOf('<CompanyContactsView')).toBeGreaterThan(
      companyDetailSource.indexOf('<DetailDescriptionEditButton'),
    )
    expect(companyDetailSource.indexOf('<CompanyContactsView')).toBeLessThan(
      companyDetailSource.indexOf('<ActivityTimeline'),
    )
    expect(companyDetailSource.indexOf("value: 'support'")).toBeLessThan(
      companyDetailSource.indexOf("value: 'notes'"),
    )
  })

  it('uses the overview action treatment for every tab-header action', () => {
    expect(companyCollectionsSource).toContain(
      "getOptionalSectionActionClass(linkOpen ? 'open' : 'available', 'borderless')",
    )
    expect(companyCollectionsSource).toContain(
      "getOptionalSectionActionClass(createOpen ? 'open' : 'available', 'borderless')",
    )
    expect(companyCollectionsSource).toContain('<Link01Icon className="h-[15px] w-[15px]" />')
    expect(companyCollectionsSource).toContain('<PlusSignIcon className="h-[15px] w-[15px]" />')
    expect(companyCollectionsSource).toContain('gap-[18px]')
    expect(emailTimelineSource).toContain('showComposeAction ? <Button size="sm"')
    expect(emailTimelineSource).toContain('<PlusSignIcon className="h-3.5 w-3.5" /> Compose')
  })

	it('creates company-context deals with the company as the atomic customer', () => {
		expect(companyDetailSource).toContain('companyContext={{ id: companyId')
		expect(createDealSource).toContain("company_id: company?.id")
		expect(createDealSource).not.toContain("from_object_type: 'deal'")
		expect(createDealSource).not.toContain('Deal created, but it could not be linked')
	})

  it('keeps the new enrichment treatment opt-in', () => {
    expect(enrichmentSource).toContain("presentation?: 'default' | 'borderless'")
    expect(enrichmentSource).toContain('presentation={presentation}')
  })

  it('always chooses a team from an add-task action matching link existing', () => {
    expect(linkedTasksSource).toContain('CheckListIcon')
    expect(linkedTasksSource).not.toContain('CheckmarkSquare02Icon')
    expect(linkedTasksSource).toContain('<SidebarPopoverSelect')
    expect(linkedTasksSource).not.toContain('triggerVariant="underline"')
    expect(linkedTasksSource).toContain('showChevron')
    expect(linkedTasksSource).toContain('<PlusSignIcon')
    expect(linkedTasksSource).toContain('handleStartCreate(teamId)')
    expect(linkedTasksSource).toContain("canEdit ? 'available' : 'locked'")
    expect(linkedTasksSource).toContain('canCreateTask && !teamsLoading && !openingCreate')
    expect(linkedTasksSource).not.toContain('<DropdownMenu>')
  })

  it('uses full-page task scrolling in tabs while retaining bounded embedded views', () => {
    expect(linkedTasksSource).toContain('<CRMTasksWorkspace')
    expect(linkedTasksSource).toContain("associationTarget.type === 'company'")
    expect(companyTasksWorkspaceSource).toContain("objectType === 'company'")
    expect(companyTasksWorkspaceSource).toContain('max-h-[470px]')
    expect(companyTasksWorkspaceSource).toContain('fitContent={!fullHeight}')
    expect(companyTasksWorkspaceSource).toContain("fullHeight ? 'flex-1' : 'max-h-[410px]'")
    expect(companyTasksWorkspaceSource).toContain("fullHeight ? 'flex-1' : 'max-h-[342px]'")
    expect(companyTasksWorkspaceSource).toContain("fullHeight ? 'h-full min-h-0' : 'max-h-[398px]'")
    expect(companyTasksWorkspaceSource).toContain("useState<'list' | 'board'>('board')")
    expect(companyTasksWorkspaceSource).toContain('<TaskListView')
    expect(companyTasksWorkspaceSource).toContain('<CompanyTaskBoard')
    expect(companyTasksWorkspaceSource).toContain("stateType: 'backlog'")
    expect(companyTasksWorkspaceSource).toContain("stateType: 'done'")
    expect(companyTasksWorkspaceSource).not.toContain('rounded-md border border-border/60 bg-muted/20')
  })

  it('inherits the detail-page surface across task, email, and meeting tabs', () => {
    expect(companyTasksWorkspaceSource).toContain(
      "fullHeight ? 'flex-1 bg-transparent' : 'max-h-[470px] bg-background'",
    )
    expect(emailTimelineSource).toContain(
      '@container/email flex h-full min-h-0 overflow-hidden bg-transparent',
    )
    expect(emailTimelineSource).toContain(
      'border-t border-border/60 bg-transparent px-5',
    )
    expect(companyCollectionsSource.match(/flex min-h-0 flex-1 flex-col bg-transparent/g)).toHaveLength(2)
  })
})
