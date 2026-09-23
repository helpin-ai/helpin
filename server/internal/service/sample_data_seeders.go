package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
	"github.com/helpin-ai/helpin/server/internal/tiptap"
)

// DefaultSampleDataSeeders returns the built-in seeders in dependency order:
// CRM first so conversations can link contacts, PM before support so a
// conversation can link its task.
func DefaultSampleDataSeeders(docsUseSortKey bool) []SampleDataSeeder {
	return []SampleDataSeeder{
		crmSampleSeeder{},
		pmSampleSeeder{},
		docsSampleSeeder{useSortKey: docsUseSortKey},
		supportSampleSeeder{},
		automationSampleSeeder{},
	}
}

// Custom agents and meetings are deliberately not seeded: agents have no
// disabled state, so a sample agent would be a live, runnable executor, and a
// useful sample meeting needs capture and processing output that only the
// capture pipeline writes.

// ── CRM ──────────────────────────────────────────────────────────────────

type crmSampleSeeder struct{}

func (crmSampleSeeder) Module() model.ModuleID { return model.ModuleCRM }

// Seed creates companies, contacts, their company associations, and open deals
// through the CRM services. The services are built without summary refresh,
// signal-motion, enrichment, or analytics dependencies.
func (crmSampleSeeder) Seed(ctx context.Context, env *SampleDataEnv) error {
	companyRepo := repository.NewCRMCompanyRepository(env.Tx)
	contactRepo := repository.NewCRMContactRepository(env.Tx)
	dealRepo := repository.NewCRMDealRepository(env.Tx)
	assocRepo := repository.NewCRMAssociationRepository(env.Tx)
	companies := NewCRMCompanyService(companyRepo)
	contacts := NewCRMContactService(contactRepo)
	associations := NewCRMAssociationService(assocRepo)
	deals := NewCRMDealService(dealRepo, assocRepo).
		SetActivityService(NewPMActivityService(repository.NewPMActivityRepository(env.Tx)))
	owner := env.ActorMemberID

	for _, fixture := range sampleCompanies {
		company, err := companies.Create(ctx, model.CreateCRMCompanyRequest{
			WorkspaceID:   env.WorkspaceID,
			Name:          fixture.name,
			Domain:        optionalText(fixture.domain),
			Industry:      optionalText(fixture.industry),
			EmployeeCount: optionalInt(fixture.employees),
			Description:   optionalText(fixture.description),
			Headquarters:  optionalText(fixture.headquarters),
			OwnerMemberID: &owner,
		})
		if err != nil {
			return fmt.Errorf("create company %q: %w", fixture.name, err)
		}
		env.CompanyIDs[fixture.key] = company.ID
		env.Track(model.SampleEntityCRMCompany, company.ID)
	}
	for _, fixture := range sampleContacts {
		lifecycle := fixture.lifecycle
		contact, err := contacts.Create(ctx, model.CreateCRMContactRequest{
			WorkspaceID:     env.WorkspaceID,
			FirstName:       fixture.firstName,
			LastName:        optionalText(fixture.lastName),
			Email:           optionalText(fixture.email),
			JobTitle:        optionalText(fixture.jobTitle),
			PrimaryLocation: optionalText(fixture.location),
			LifecycleStage:  &lifecycle,
			OwnerMemberID:   &owner,
		})
		if err != nil {
			return fmt.Errorf("create contact %q: %w", fixture.email, err)
		}
		env.ContactIDs[fixture.key] = contact.ID
		env.Track(model.SampleEntityCRMContact, contact.ID)
		if fixture.companyKey == "" {
			continue
		}
		if _, err := associations.Create(ctx, model.CreateCRMAssociationRequest{
			WorkspaceID:    env.WorkspaceID,
			FromObjectType: model.CRMObjectContact,
			FromObjectID:   contact.ID,
			ToObjectType:   model.CRMObjectCompany,
			ToObjectID:     env.CompanyIDs[fixture.companyKey],
		}); err != nil {
			return fmt.Errorf("associate contact %q: %w", fixture.email, err)
		}
	}
	pipeline, err := sampleSalesPipeline(ctx, env, dealRepo)
	if err != nil {
		return err
	}
	for _, fixture := range sampleDeals {
		stageID := samplePipelineStage(pipeline, fixture.stageName)
		if stageID == "" {
			return fmt.Errorf("pipeline %q has no open stage", pipeline.Name)
		}
		amount := fixture.amount
		currency := "USD"
		closeDate := env.Now.AddDate(0, 0, fixture.closeInDays)
		deal, err := deals.CreateWithActor(ctx, model.CreateCRMDealRequest{
			WorkspaceID:   env.WorkspaceID,
			Name:          fixture.name,
			ContactID:     env.ContactIDs[fixture.contactKey],
			CompanyID:     env.CompanyIDs[fixture.companyKey],
			PipelineID:    pipeline.ID,
			StageID:       stageID,
			Amount:        &amount,
			Currency:      &currency,
			CloseDate:     &closeDate,
			OwnerMemberID: &owner,
		}, env.ActorID)
		if err != nil {
			return fmt.Errorf("create deal %q: %w", fixture.name, err)
		}
		env.Track(model.SampleEntityCRMDeal, deal.ID)
	}
	return nil
}

// sampleSalesPipeline returns the workspace's default pipeline, creating (and
// tracking) the standard one when the workspace has none.
func sampleSalesPipeline(ctx context.Context, env *SampleDataEnv, dealRepo *repository.CRMDealRepository) (*model.CRMPipeline, error) {
	pipelines, err := dealRepo.ListPipelines(ctx, env.WorkspaceID)
	if err != nil {
		return nil, err
	}
	created := false
	if len(pipelines) == 0 {
		if err := dealRepo.SeedDefaultPipeline(ctx, env.WorkspaceID); err != nil {
			return nil, err
		}
		if pipelines, err = dealRepo.ListPipelines(ctx, env.WorkspaceID); err != nil {
			return nil, err
		}
		created = true
	}
	if len(pipelines) == 0 {
		return nil, fmt.Errorf("no sales pipeline available")
	}
	pipeline := pipelines[0]
	for _, candidate := range pipelines {
		if candidate.IsDefault {
			pipeline = candidate
			break
		}
	}
	if created {
		env.Track(model.SampleEntityCRMPipeline, pipeline.ID)
	}
	return &pipeline, nil
}

func samplePipelineStage(pipeline *model.CRMPipeline, name string) string {
	firstOpen := ""
	for _, stage := range pipeline.Stages {
		if stage.StageType != "open" {
			continue
		}
		if strings.EqualFold(stage.Name, name) {
			return stage.ID
		}
		if firstOpen == "" {
			firstOpen = stage.ID
		}
	}
	return firstOpen
}

// ── Projects ─────────────────────────────────────────────────────────────

type pmSampleSeeder struct{}

func (pmSampleSeeder) Module() model.ModuleID { return model.ModulePM }

// Seed creates one project (epic) and tasks across workflow states through the
// PM services, built without websocket, notification, follower, automation,
// rule-engine, triage, agent, or git dependencies.
func (pmSampleSeeder) Seed(ctx context.Context, env *SampleDataEnv) error {
	workspaceRepo := repository.NewWorkspaceRepository(env.Tx)
	workflowRepo := repository.NewPMWorkflowRepository(env.Tx)
	taskRepo := repository.NewPMTaskRepository(env.Tx)
	epicRepo := repository.NewPMEpicRepository(env.Tx)
	labelRepo := repository.NewPMLabelRepository(env.Tx)
	activity := NewPMActivityService(repository.NewPMActivityRepository(env.Tx))
	tasks := NewPMTaskService(taskRepo, workspaceRepo, workflowRepo, epicRepo,
		repository.NewPMSprintRepository(env.Tx), labelRepo,
		repository.NewPMChecklistItemRepository(env.Tx), repository.NewPMExternalLinkRepository(env.Tx),
		repository.NewPMAttachmentRepository(env.Tx), activity, nil, nil, nil, nil)
	epics := NewPMEpicService(epicRepo, taskRepo, labelRepo, nil, nil, workspaceRepo, activity, nil, nil)

	teamID, err := sampleTeam(ctx, env)
	if err != nil {
		return err
	}
	workflow, err := workflowRepo.GetDefaultWorkflow(ctx, env.WorkspaceID)
	if err != nil {
		return err
	}
	if workflow == nil || len(workflow.States) == 0 {
		if workflow, err = workflowRepo.SeedDefaultWorkflow(ctx, env.WorkspaceID); err != nil {
			return err
		}
	}
	epicStates, err := workflowRepo.SeedDefaultEpicStates(ctx, env.WorkspaceID)
	if err != nil {
		return err
	}
	epicReq := model.CreateEpicRequest{
		WorkspaceID:      env.WorkspaceID,
		Name:             sampleEpicName,
		Description:      optionalText(renderTaskDescriptionRichText(sampleEpicDescription)),
		EpicStateID:      sampleEpicState(epicStates),
		OwnerMemberID:    &env.ActorMemberID,
		TeamID:           &teamID,
		PlannedStartDate: timePtr(env.Now.AddDate(0, 0, -7)),
		Deadline:         timePtr(env.Now.AddDate(0, 0, 30)),
		Health:           optionalText(model.PMEpicHealthOnTrack),
	}
	epic, err := epics.Create(ctx, epicReq, env.ActorID)
	if err != nil {
		return fmt.Errorf("create project: %w", err)
	}
	env.Track(model.SampleEntityPMEpic, epic.Epic.ID)

	for _, fixture := range sampleTasks {
		req := model.CreateTaskRequest{
			WorkspaceID:     env.WorkspaceID,
			Name:            fixture.name,
			Description:     optionalText(renderTaskDescriptionRichText(fixture.description)),
			TaskType:        fixture.taskType,
			WorkflowID:      workflow.Workflow.ID,
			WorkflowStateID: sampleWorkflowState(workflow, fixture.stateType, fixture.stateName),
			TeamID:          &teamID,
			OwnerMemberIDs:  []string{env.ActorMemberID},
			Priority:        optionalText(fixture.priority),
		}
		if fixture.inEpic {
			req.EpicID = &epic.Epic.ID
		}
		for position, text := range fixture.checklist {
			item := position
			req.ChecklistItems = append(req.ChecklistItems, model.CreateChecklistItemRequest{Text: text, Position: &item})
		}
		task, err := tasks.Create(ctx, req, env.ActorID)
		if err != nil {
			return fmt.Errorf("create task %q: %w", fixture.name, err)
		}
		env.TaskIDs[fixture.key] = task.Task.ID
		env.Track(model.SampleEntityPMTask, task.Task.ID)
	}
	return epics.syncProgress(ctx, epic.Epic.ID)
}

// sampleTeam returns the team sample work belongs to, creating and tracking a
// team only when the workspace has none.
func sampleTeam(ctx context.Context, env *SampleDataEnv) (string, error) {
	teamID, err := env.Repo.PreferredTeamID(ctx, env.WorkspaceID, env.ActorMemberID)
	if err != nil || teamID != "" {
		return teamID, err
	}
	team := &model.WorkspaceTeam{WorkspaceID: env.WorkspaceID, Name: sampleTeamName, TeamType: "engineering", DefaultTaskType: model.PMTaskTypeFeature, SprintsEnabled: true}
	if err := env.Repo.CreateTeam(ctx, team, env.ActorMemberID); err != nil {
		return "", err
	}
	env.Track(model.SampleEntityWorkspaceTeam, team.ID)
	return team.ID, nil
}

func sampleWorkflowState(workflow *model.WorkflowWithStates, stateType, name string) string {
	byType := ""
	for _, state := range workflow.States {
		if state.StateType != stateType {
			continue
		}
		if strings.EqualFold(state.Name, name) {
			return state.ID
		}
		if byType == "" {
			byType = state.ID
		}
	}
	if byType != "" {
		return byType
	}
	if workflow.Workflow.DefaultStateID != nil {
		return *workflow.Workflow.DefaultStateID
	}
	return workflow.States[0].ID
}

func sampleEpicState(states []model.PMEpicWorkflowState) *string {
	for _, state := range states {
		if state.StateType == model.PMStateTypeStarted {
			id := state.ID
			return &id
		}
	}
	return nil
}

// ── Docs ─────────────────────────────────────────────────────────────────

type docsSampleSeeder struct {
	useSortKey bool
}

func (docsSampleSeeder) Module() model.ModuleID { return model.ModuleDocs }

// Seed creates a help-center-capable space with draft articles. Nothing is
// published, and the services are built without translation, mention,
// embedding, entitlement, or websocket dependencies.
func (s docsSampleSeeder) Seed(ctx context.Context, env *SampleDataEnv) error {
	spaceRepo := repository.NewDocsSpaceRepository(env.Tx)
	docRepo := repository.NewDocsDocumentRepository(env.Tx, s.useSortKey)
	spaces := NewDocsSpaceService(spaceRepo, nil)
	documents := NewDocsDocumentService(docRepo, spaceRepo, nil, s.useSortKey)
	contents := NewDocsContentService(repository.NewDocsContentRepository(env.Tx), docRepo, nil)

	slug, err := sampleSpaceSlug(ctx, env, spaceRepo)
	if err != nil {
		return err
	}
	space, err := spaces.Create(ctx, env.WorkspaceID, model.CreateDocsSpaceRequest{
		Name: sampleHelpSpaceName,
		Slug: slug,
		Type: model.SpaceTypeExternalCapable,
	}, env.ActorID)
	if err != nil {
		return fmt.Errorf("create help center space: %w", err)
	}
	env.Track(model.SampleEntityDocsSpace, space.ID)
	for _, article := range sampleArticles {
		doc, err := documents.Create(ctx, env.WorkspaceID, model.CreateDocsDocumentRequest{
			SpaceID: space.ID,
			Title:   article.title,
			OwnerID: &env.ActorID,
		}, env.ActorID)
		if err != nil {
			return fmt.Errorf("create article %q: %w", article.title, err)
		}
		env.Track(model.SampleEntityDocsDocument, doc.ID)
		if _, err := contents.Save(ctx, doc.ID, sampleArticleContent(article), env.ActorID); err != nil {
			return fmt.Errorf("save article %q: %w", article.title, err)
		}
	}
	return nil
}

func sampleArticleContent(article sampleArticle) json.RawMessage {
	return tiptap.MarkdownToJSON(article.markdown)
}

// sampleSpaceSlug avoids colliding with a space the workspace already uses.
func sampleSpaceSlug(ctx context.Context, env *SampleDataEnv, spaceRepo *repository.DocsSpaceRepository) (string, error) {
	slug := sampleHelpSpaceSlug
	for attempt := 2; attempt < 50; attempt++ {
		existing, err := spaceRepo.GetBySlug(ctx, env.WorkspaceID, slug)
		if err != nil {
			return "", err
		}
		if existing == nil {
			return slug, nil
		}
		slug = fmt.Sprintf("%s-%d", sampleHelpSpaceSlug, attempt)
	}
	return "", fmt.Errorf("no free slug for the sample help center space")
}

// ── Support ──────────────────────────────────────────────────────────────

type supportSampleSeeder struct{}

func (supportSampleSeeder) Module() model.ModuleID { return model.ModuleSupport }

// Seed creates inbox conversations through the support repositories. The
// service send paths are deliberately bypassed because they deliver email,
// run AI replies, triage, and live translation; queued live-translation work
// for sample messages is purged inside the load transaction.
func (supportSampleSeeder) Seed(ctx context.Context, env *SampleDataEnv) error {
	conversations := repository.NewSupportConversationRepository(env.Tx)
	messages := repository.NewSupportMessageRepository(env.Tx)
	for _, fixture := range sampleConversations {
		conversation, err := sampleConversationRecord(env, fixture)
		if err != nil {
			return err
		}
		if err := conversations.Create(ctx, conversation); err != nil {
			return fmt.Errorf("create conversation %q: %w", fixture.subject, err)
		}
		env.Track(model.SampleEntitySupportConversation, conversation.ID)
		for _, message := range fixture.messages {
			if err := messages.Create(ctx, sampleMessageRecord(env, conversation, fixture, message)); err != nil {
				return fmt.Errorf("create message in %q: %w", fixture.subject, err)
			}
		}
		if err := env.Repo.PurgeLiveTranslationQueue(ctx, conversation.ID); err != nil {
			return err
		}
		if err := env.Repo.SetConversationState(ctx, env.WorkspaceID, conversation.ID, sampleConversationState(env, fixture)); err != nil {
			return err
		}
	}
	return nil
}

func sampleConversationRecord(env *SampleDataEnv, fixture sampleConversation) (*model.SupportConversation, error) {
	if len(fixture.messages) == 0 {
		return nil, fmt.Errorf("sample conversation %q has no messages", fixture.subject)
	}
	flowState := model.SupportConversationFlowStateAssignedToHuman
	createdAt := env.Now.Add(-time.Duration(fixture.messages[0].minutesAgo) * time.Minute)
	conversation := &model.SupportConversation{
		WorkspaceID:    env.WorkspaceID,
		Subject:        fixture.subject,
		Status:         model.SupportConversationStatusOpen,
		FlowState:      &flowState,
		Priority:       fixture.priority,
		Channel:        fixture.channel,
		Source:         fixture.channel,
		CustomerName:   optionalText(fixture.customerName),
		CustomerEmail:  optionalText(fixture.customerEmail),
		AssignedUserID: &env.ActorID,
		CreatedAt:      createdAt,
	}
	if id := env.ContactIDs[fixture.contactKey]; id != "" {
		conversation.CRMContactID = &id
	}
	if id := env.CompanyIDs[fixture.companyKey]; id != "" {
		conversation.CRMCompanyID = &id
	}
	if id := env.TaskIDs[fixture.linkedTaskKey]; id != "" {
		conversation.LinkedTaskID = &id
	}
	return conversation, nil
}

func sampleMessageRecord(env *SampleDataEnv, conversation *model.SupportConversation, fixture sampleConversation, message sampleMessage) *model.SupportMessage {
	record := &model.SupportMessage{
		WorkspaceID:    env.WorkspaceID,
		ConversationID: conversation.ID,
		MessageType:    "reply",
		Content:        message.body,
		Metadata:       "{}",
		CreatedAt:      env.Now.Add(-time.Duration(message.minutesAgo) * time.Minute),
	}
	switch message.sender {
	case "customer":
		record.SenderType = "customer"
		record.SenderDisplayName = optionalText(fixture.customerName)
	default:
		record.SenderType = "user"
		record.SenderUserID = &env.ActorID
		record.SenderDisplayName = optionalText(env.ActorName)
		record.IsInternal = message.sender == "note"
		// Sample replies were never emailed; marking them handled keeps the
		// email fallback from ever picking them up.
		notifiedAt := record.CreatedAt
		record.EmailNotifiedAt = &notifiedAt
	}
	return record
}

func sampleConversationState(env *SampleDataEnv, fixture sampleConversation) map[string]any {
	last := fixture.messages[len(fixture.messages)-1]
	lastAt := env.Now.Add(-time.Duration(last.minutesAgo) * time.Minute)
	fields := map[string]any{"status": fixture.status, "updated_at": lastAt}
	switch fixture.status {
	case model.SupportConversationStatusResolved:
		fields["resolved_at"] = lastAt
		fields["flow_state"] = model.SupportConversationFlowStateResolvedByHuman
		fields["team_last_seen_at"] = lastAt
	case model.SupportConversationStatusWaitingOnCustomer:
		fields["team_last_seen_at"] = lastAt
	}
	return fields
}

// ── Automation ───────────────────────────────────────────────────────────

type automationSampleSeeder struct{}

func (automationSampleSeeder) Module() model.ModuleID { return model.ModuleAutomation }

func (automationSampleSeeder) sampleDataCompanion() {}

// Seed creates example Flows on the default task workflow. Every Flow is
// inserted disabled through the repository, bypassing the rule engine: no cron
// schedule is registered, no websocket event is published, and the engine
// only matches enabled rules, so nothing fires until someone turns a Flow on.
// Without a task workflow there is nothing for the Flows to act on, so none
// are created.
func (automationSampleSeeder) Seed(ctx context.Context, env *SampleDataEnv) error {
	workflow, err := repository.NewPMWorkflowRepository(env.Tx).GetDefaultWorkflow(ctx, env.WorkspaceID)
	if err != nil {
		return err
	}
	if workflow == nil || len(workflow.States) == 0 {
		return nil
	}
	for position, fixture := range sampleFlows {
		rule, err := sampleFlowRecord(env, workflow, fixture, position)
		if err != nil {
			return err
		}
		if err := env.Repo.CreateDisabledAutomationRule(ctx, rule); err != nil {
			return fmt.Errorf("create Flow %q: %w", fixture.name, err)
		}
		env.Track(model.SampleEntityAutomationRule, rule.ID)
	}
	return nil
}

func sampleFlowRecord(env *SampleDataEnv, workflow *model.WorkflowWithStates, fixture sampleFlow, position int) (*model.AutomationRule, error) {
	trigger, err := json.Marshal(model.TriggerConfigGitHubPullRequest{BaseBranch: fixture.baseBranch})
	if err != nil {
		return nil, err
	}
	action, err := json.Marshal(model.ActionConfigMoveToState{
		TargetStateID: sampleWorkflowState(workflow, fixture.targetStateType, fixture.targetStateName),
	})
	if err != nil {
		return nil, err
	}
	workflowID := workflow.Workflow.ID
	rule := &model.AutomationRule{
		ID:            uuid.NewString(),
		WorkspaceID:   env.WorkspaceID,
		Name:          fixture.name,
		Description:   optionalText(fixture.description),
		WorkflowID:    &workflowID,
		TriggerType:   fixture.triggerType,
		TriggerConfig: trigger,
		ActionType:    model.ActionMoveToState,
		ActionConfig:  action,
		Position:      position,
		CreatedAt:     env.Now,
		UpdatedAt:     env.Now,
	}
	if env.ActorID != "" {
		actorID := env.ActorID
		rule.CreatedBy = &actorID
	}
	return rule, nil
}

func optionalText(value string) *string {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	return &value
}

func optionalInt(value int) *int {
	if value == 0 {
		return nil
	}
	return &value
}
