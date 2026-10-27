package service

import (
	"context"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"math"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

// ErrSupportReportInvalidRange means the requested period is empty, reversed
// or longer than model.SupportReportMaxRange.
var ErrSupportReportInvalidRange = errors.New("choose a report period of up to one year")

const supportReportRecentComments = 20

// SupportReportService builds support metrics from conversations, messages
// and CSAT ratings. Every query respects the caller's inbox access.
type SupportReportService struct {
	conversations *repository.SupportConversationRepository
}

func NewSupportReportService(conversations *repository.SupportConversationRepository) *SupportReportService {
	return &SupportReportService{conversations: conversations}
}

type supportReportPeriod struct {
	from, to time.Time
}

func (p supportReportPeriod) contains(t *time.Time) bool {
	return t != nil && !t.Before(p.from) && t.Before(p.to)
}

func (s *SupportReportService) scope(ctx context.Context, req model.SupportReportRequest, from, to time.Time) repository.SupportReportScope {
	scope := repository.SupportReportScope{
		WorkspaceID: req.WorkspaceID, From: from, To: to,
		MailboxIDs: req.MailboxIDs, Channels: req.Channels,
	}
	if actor := supportActorFromContext(ctx, req.WorkspaceID); actor != nil {
		scope.WorkspaceMemberID, scope.Role = actor.WorkspaceMemberID, actor.Role
	}
	return scope
}

func validateSupportReportRange(req model.SupportReportRequest) (*time.Location, error) {
	if req.WorkspaceID == "" || req.From.IsZero() || req.To.IsZero() || !req.From.Before(req.To) ||
		req.To.Sub(req.From) > model.SupportReportMaxRange {
		return nil, ErrSupportReportInvalidRange
	}
	location := time.UTC
	if tz := strings.TrimSpace(req.Timezone); tz != "" {
		loaded, err := time.LoadLocation(tz)
		if err != nil {
			return nil, fmt.Errorf("unknown timezone %q", tz)
		}
		location = loaded
	}
	return location, nil
}

// Overview returns the report for [req.From, req.To) compared with the
// previous period of the same length.
func (s *SupportReportService) Overview(ctx context.Context, req model.SupportReportRequest) (*model.SupportReportOverview, error) {
	location, err := validateSupportReportRange(req)
	if err != nil {
		return nil, err
	}
	current := supportReportPeriod{from: req.From.UTC(), to: req.To.UTC()}
	previous := supportReportPeriod{from: current.from.Add(-current.to.Sub(current.from)), to: current.from}
	scope := s.scope(ctx, req, previous.from, current.to)

	facts, err := s.conversations.ListReportFacts(ctx, scope)
	if err != nil {
		return nil, err
	}
	ratings, err := s.conversations.ListReportCSAT(ctx, scope, previous.from, current.to)
	if err != nil {
		return nil, err
	}
	replies, err := s.conversations.ListReportReplyCounts(ctx, scope, current.from, current.to)
	if err != nil {
		return nil, err
	}
	tags, err := s.conversations.ListReportTags(ctx, scope)
	if err != nil {
		return nil, err
	}

	currentRatings, previousRatings := splitSupportReportRatings(ratings, current)
	overview := &model.SupportReportOverview{
		Range: model.SupportReportRange{
			From: current.from, To: current.to, PreviousFrom: previous.from, PreviousTo: previous.to,
			Timezone: location.String(),
		},
		Summary:  summarizeSupportReport(facts, currentRatings, current),
		Previous: summarizeSupportReport(facts, previousRatings, previous),
		Daily:    supportReportDaily(facts, current, location),
		Topics:   supportReportTopics(facts, tags, currentRatings, current, previous),
		Channels: supportReportChannels(facts, current),
		CSAT:     supportReportCSATDetail(currentRatings),
	}
	teammates, err := s.teammates(ctx, facts, currentRatings, replies, current)
	if err != nil {
		return nil, err
	}
	overview.Teammates = teammates
	slaRows, err := s.conversations.ListReportSLA(ctx, scope, current.from, current.to)
	if err != nil {
		return nil, err
	}
	overview.SLA = summarizeSupportReportSLA(slaRows)
	// Requests have no inbox while they are written, so intake is reported
	// only for the whole workspace.
	if len(req.MailboxIDs) == 0 && (len(req.Channels) == 0 || slices.Contains(req.Channels, "portal")) {
		intake, err := s.conversations.ReportPortalIntake(ctx, req.WorkspaceID, current.from, current.to, time.Now().UTC())
		if err != nil {
			return nil, err
		}
		overview.PortalIntake = intake
	}
	return overview, nil
}

// summarizeSupportReportSLA counts finished targets. A next-response
// conversation counts as met when it finished cycles without a miss.
func summarizeSupportReportSLA(rows []repository.SupportReportSLARow) model.SupportReportSLA {
	summary := model.SupportReportSLA{Conversations: len(rows)}
	count := func(target *model.SupportReportSLATarget, state string) {
		switch state {
		case model.SupportSLAStateAchieved:
			target.Met++
		case model.SupportSLAStateBreached:
			target.Missed++
		}
	}
	for _, row := range rows {
		count(&summary.FirstResponse, row.FirstResponseState)
		count(&summary.Resolution, row.ResolutionState)
		if row.NextResponseBreaches > 0 {
			summary.NextResponse.Missed++
		} else if row.NextResponseState == model.SupportSLAStateAchieved {
			summary.NextResponse.Met++
		}
	}
	for _, target := range []*model.SupportReportSLATarget{&summary.FirstResponse, &summary.NextResponse, &summary.Resolution} {
		target.HitRate = supportReportRate(target.Met, target.Met+target.Missed)
	}
	return summary
}

func splitSupportReportRatings(rows []repository.SupportReportCSATRow, current supportReportPeriod) (cur, prev []repository.SupportReportCSATRow) {
	for _, row := range rows {
		submitted := row.SubmittedAt
		if current.contains(&submitted) {
			cur = append(cur, row)
		} else {
			prev = append(prev, row)
		}
	}
	return cur, prev
}

func supportReportResolvedByAI(fact repository.SupportReportConversationFact) bool {
	if fact.HumanTakeover != nil && *fact.HumanTakeover {
		return false
	}
	return fact.FlowState != nil && *fact.FlowState == model.SupportConversationFlowStateResolvedByAI
}

func supportReportResolvedIn(fact repository.SupportReportConversationFact, period supportReportPeriod) bool {
	return fact.Status == model.SupportConversationStatusResolved && period.contains(fact.ResolvedAt)
}

// supportReportCustomerStarted reports whether the customer wrote before
// anyone replied; only then is a first response time meaningful.
func supportReportCustomerStarted(fact repository.SupportReportConversationFact) bool {
	return fact.FirstCustomerAt != nil && (fact.FirstResponseAt == nil || !fact.FirstResponseAt.Before(*fact.FirstCustomerAt))
}

func supportReportFirstHumanResponse(fact repository.SupportReportConversationFact) (time.Duration, bool) {
	if !supportReportCustomerStarted(fact) || fact.FirstHumanReplyAt == nil {
		return 0, false
	}
	return clampSupportReportDuration(fact.FirstHumanReplyAt.Sub(*fact.FirstCustomerAt)), true
}

func supportReportFirstResponse(fact repository.SupportReportConversationFact) (time.Duration, bool) {
	if !supportReportCustomerStarted(fact) || fact.FirstResponseAt == nil {
		return 0, false
	}
	return clampSupportReportDuration(fact.FirstResponseAt.Sub(*fact.FirstCustomerAt)), true
}

func supportReportHandedOff(fact repository.SupportReportConversationFact) bool {
	return fact.AIEscalatedAt != nil || fact.CustomerRequestedHumanAt != nil ||
		(fact.AIState != nil && *fact.AIState == "escalated")
}

func clampSupportReportDuration(d time.Duration) time.Duration {
	if d < 0 {
		return 0
	}
	return d
}

func summarizeSupportReport(facts []repository.SupportReportConversationFact, ratings []repository.SupportReportCSATRow, period supportReportPeriod) model.SupportReportSummary {
	var summary model.SupportReportSummary
	var firstResponses, firstHumanResponses, resolutions []time.Duration
	for _, fact := range facts {
		created := fact.CreatedAt
		if period.contains(&created) {
			summary.ConversationsCreated++
			if d, ok := supportReportFirstResponse(fact); ok {
				firstResponses = append(firstResponses, d)
			}
			if d, ok := supportReportFirstHumanResponse(fact); ok {
				firstHumanResponses = append(firstHumanResponses, d)
			}
			if fact.AIReplyCount > 0 {
				summary.AIInvolved++
				if fact.Status == model.SupportConversationStatusResolved && supportReportResolvedByAI(fact) {
					summary.AIResolved++
					switch derefString(fact.AIResolutionType) {
					case "confirmed":
						summary.AIResolvedConfirm++
					case "assumed":
						summary.AIResolvedAssumed++
					}
				}
				if supportReportHandedOff(fact) {
					summary.AIHandoffs++
				}
			}
		}
		if supportReportResolvedIn(fact, period) {
			summary.ConversationsResolved++
			if supportReportResolvedByAI(fact) {
				summary.ResolvedByAI++
			} else {
				summary.ResolvedByHuman++
			}
			resolutions = append(resolutions, clampSupportReportDuration(fact.ResolvedAt.Sub(fact.CreatedAt)))
		}
	}
	summary.FirstResponse = summarizeSupportReportDurations(firstResponses)
	summary.FirstHumanResponse = summarizeSupportReportDurations(firstHumanResponses)
	summary.Resolution = summarizeSupportReportDurations(resolutions)
	summary.AIInvolvementRate = supportReportRate(summary.AIInvolved, summary.ConversationsCreated)
	summary.AIResolutionRate = supportReportRate(summary.AIResolved, summary.AIInvolved)
	summary.AIHandoffRate = supportReportRate(summary.AIHandoffs, summary.AIInvolved)

	positive, total := 0, 0
	for _, rating := range ratings {
		total += rating.Rating
		if rating.Rating >= 4 {
			positive++
		}
	}
	summary.CSATResponses = len(ratings)
	summary.CSATAverage = supportReportAverage(total, len(ratings))
	summary.CSATPositiveRate = supportReportRate(positive, len(ratings))
	return summary
}

func supportReportRate(numerator, denominator int) *float64 {
	if denominator == 0 {
		return nil
	}
	value := math.Round(float64(numerator)/float64(denominator)*10000) / 10000
	return &value
}

func supportReportAverage(total, count int) *float64 {
	if count == 0 {
		return nil
	}
	value := math.Round(float64(total)/float64(count)*100) / 100
	return &value
}

// summarizeSupportReportDurations returns the median and the 90th percentile
// (nearest rank) in whole seconds.
func summarizeSupportReportDurations(values []time.Duration) model.SupportReportDuration {
	result := model.SupportReportDuration{Count: len(values)}
	if len(values) == 0 {
		return result
	}
	sorted := append([]time.Duration(nil), values...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i] < sorted[j] })
	middle := len(sorted) / 2
	median := sorted[middle]
	if len(sorted)%2 == 0 {
		median = (sorted[middle-1] + sorted[middle]) / 2
	}
	rank := int(math.Ceil(0.9*float64(len(sorted)))) - 1
	medianSeconds := int64(median.Round(time.Second) / time.Second)
	p90Seconds := int64(sorted[rank].Round(time.Second) / time.Second)
	result.MedianSeconds, result.P90Seconds = &medianSeconds, &p90Seconds
	return result
}

func supportReportDaily(facts []repository.SupportReportConversationFact, period supportReportPeriod, location *time.Location) []model.SupportReportDay {
	start := period.from.In(location)
	start = time.Date(start.Year(), start.Month(), start.Day(), 0, 0, 0, 0, location)
	var days []model.SupportReportDay
	index := map[string]int{}
	firstResponses := map[string][]time.Duration{}
	for day := start; day.Before(period.to); day = day.AddDate(0, 0, 1) {
		key := day.Format("2006-01-02")
		index[key] = len(days)
		days = append(days, model.SupportReportDay{Date: key})
	}
	for _, fact := range facts {
		created := fact.CreatedAt
		if period.contains(&created) {
			key := created.In(location).Format("2006-01-02")
			if i, ok := index[key]; ok {
				days[i].ConversationsCreated++
				if d, ok := supportReportFirstHumanResponse(fact); ok {
					firstResponses[key] = append(firstResponses[key], d)
				}
			}
		}
		if supportReportResolvedIn(fact, period) {
			key := fact.ResolvedAt.In(location).Format("2006-01-02")
			if i, ok := index[key]; ok {
				days[i].ConversationsResolved++
				if supportReportResolvedByAI(fact) {
					days[i].ResolvedByAI++
				}
			}
		}
	}
	for key, values := range firstResponses {
		days[index[key]].MedianFirstHumanResponseS = summarizeSupportReportDurations(values).MedianSeconds
	}
	return days
}

func supportReportChannels(facts []repository.SupportReportConversationFact, period supportReportPeriod) []model.SupportReportChannel {
	byChannel := map[string]*model.SupportReportChannel{}
	durations := map[string][]time.Duration{}
	get := func(channel string) *model.SupportReportChannel {
		if channel == "" {
			channel = "unknown"
		}
		if existing, ok := byChannel[channel]; ok {
			return existing
		}
		created := &model.SupportReportChannel{Channel: channel}
		byChannel[channel] = created
		return created
	}
	for _, fact := range facts {
		created := fact.CreatedAt
		if period.contains(&created) {
			entry := get(fact.Channel)
			entry.ConversationsCreated++
			if d, ok := supportReportFirstHumanResponse(fact); ok {
				durations[entry.Channel] = append(durations[entry.Channel], d)
			}
		}
		if supportReportResolvedIn(fact, period) {
			get(fact.Channel).ConversationsResolved++
		}
	}
	result := make([]model.SupportReportChannel, 0, len(byChannel))
	for channel, entry := range byChannel {
		entry.FirstHumanResponse = summarizeSupportReportDurations(durations[channel])
		result = append(result, *entry)
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].ConversationsCreated != result[j].ConversationsCreated {
			return result[i].ConversationsCreated > result[j].ConversationsCreated
		}
		return result[i].Channel < result[j].Channel
	})
	return result
}

func supportReportCSATDetail(ratings []repository.SupportReportCSATRow) model.SupportReportCSAT {
	detail := model.SupportReportCSAT{RecentComments: []model.SupportReportCSATComment{}}
	aiTotal, humanTotal := 0, 0
	for _, rating := range ratings {
		if rating.Rating >= 1 && rating.Rating <= 5 {
			detail.Distribution[rating.Rating-1]++
		}
		if rating.ResolvedBy == model.SupportCSATResolvedByAI {
			detail.AIResponses++
			aiTotal += rating.Rating
		} else {
			detail.HumanResponses++
			humanTotal += rating.Rating
		}
		if rating.Comment != nil && strings.TrimSpace(*rating.Comment) != "" && len(detail.RecentComments) < supportReportRecentComments {
			detail.RecentComments = append(detail.RecentComments, model.SupportReportCSATComment{
				ConversationID: rating.ConversationID, DisplayID: rating.DisplayID, Subject: rating.Subject,
				Rating: rating.Rating, Comment: *rating.Comment, Channel: rating.Channel,
				ResolvedBy: rating.ResolvedBy, SubmittedAt: rating.SubmittedAt,
			})
		}
	}
	detail.AIAverage = supportReportAverage(aiTotal, detail.AIResponses)
	detail.HumanAverage = supportReportAverage(humanTotal, detail.HumanResponses)
	return detail
}

func (s *SupportReportService) teammates(
	ctx context.Context,
	facts []repository.SupportReportConversationFact,
	ratings []repository.SupportReportCSATRow,
	replies []repository.SupportReportReplyCount,
	period supportReportPeriod,
) ([]model.SupportReportTeammate, error) {
	type accumulator struct {
		entry       model.SupportReportTeammate
		durations   []time.Duration
		ratingTotal int
	}
	byUser := map[string]*accumulator{}
	get := func(userID string) *accumulator {
		if existing, ok := byUser[userID]; ok {
			return existing
		}
		created := &accumulator{entry: model.SupportReportTeammate{UserID: userID}}
		byUser[userID] = created
		return created
	}
	for _, fact := range facts {
		created := fact.CreatedAt
		if period.contains(&created) && fact.FirstHumanReplyUserID != nil {
			if d, ok := supportReportFirstHumanResponse(fact); ok {
				acc := get(*fact.FirstHumanReplyUserID)
				acc.durations = append(acc.durations, d)
			}
		}
		if supportReportResolvedIn(fact, period) && !supportReportResolvedByAI(fact) {
			// Credit whoever resolved it; older conversations without a
			// resolve event fall back to the assignee.
			if resolver := firstNonEmptyPointer(fact.ResolvedByUserID, fact.AssignedUserID); resolver != "" {
				get(resolver).entry.Resolved++
			}
		}
	}
	for _, reply := range replies {
		get(reply.UserID).entry.RepliesSent += reply.Replies
	}
	for _, rating := range ratings {
		if rating.AssignedUserID == nil {
			continue
		}
		acc := get(*rating.AssignedUserID)
		acc.entry.CSATResponses++
		acc.ratingTotal += rating.Rating
	}
	ids := make([]string, 0, len(byUser))
	for id := range byUser {
		ids = append(ids, id)
	}
	users, err := s.conversations.ListReportUsers(ctx, ids)
	if err != nil {
		return nil, err
	}
	names := make(map[string]repository.SupportReportUser, len(users))
	for _, user := range users {
		names[user.ID] = user
	}
	result := make([]model.SupportReportTeammate, 0, len(byUser))
	for id, acc := range byUser {
		user, known := names[id]
		if !known {
			// Deleted users keep their history out of the teammate table.
			continue
		}
		acc.entry.Name = user.FullName
		acc.entry.AvatarURL = user.AvatarURL
		acc.entry.FirstHumanResponse = summarizeSupportReportDurations(acc.durations)
		acc.entry.CSATAverage = supportReportAverage(acc.ratingTotal, acc.entry.CSATResponses)
		result = append(result, acc.entry)
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].Resolved != result[j].Resolved {
			return result[i].Resolved > result[j].Resolved
		}
		if result[i].RepliesSent != result[j].RepliesSent {
			return result[i].RepliesSent > result[j].RepliesSent
		}
		return result[i].Name < result[j].Name
	})
	return result, nil
}

var supportReportCSVHeader = []string{
	"conversation_number", "subject", "channel", "inbox", "status", "priority", "assignee",
	"customer_name", "customer_email", "created_at", "first_response_at", "first_response_seconds",
	"first_human_response_at", "first_human_response_seconds", "resolved_at", "resolution_seconds",
	"resolved_by", "ai_involved", "ai_resolution_type", "handed_off", "csat_rating", "csat_comment", "tags",
}

// ExportConversationsCSV writes one row per conversation created in the period.
func (s *SupportReportService) ExportConversationsCSV(ctx context.Context, req model.SupportReportRequest, w io.Writer) error {
	if _, err := validateSupportReportRange(req); err != nil {
		return err
	}
	period := supportReportPeriod{from: req.From.UTC(), to: req.To.UTC()}
	scope := s.scope(ctx, req, period.from, period.to)
	facts, err := s.conversations.ListReportFacts(ctx, scope)
	if err != nil {
		return err
	}
	ratings, err := s.conversations.ListReportCSAT(ctx, scope, period.from, time.Now().UTC().Add(time.Minute))
	if err != nil {
		return err
	}
	ratingByConversation := make(map[string]repository.SupportReportCSATRow, len(ratings))
	for _, rating := range ratings {
		ratingByConversation[rating.ConversationID] = rating
	}
	tags, err := s.conversations.ListReportTags(ctx, scope)
	if err != nil {
		return err
	}
	tagNames := supportReportTagNames(tags)
	userIDs := []string{}
	for _, fact := range facts {
		if fact.AssignedUserID != nil {
			userIDs = append(userIDs, *fact.AssignedUserID)
		}
	}
	users, err := s.conversations.ListReportUsers(ctx, userIDs)
	if err != nil {
		return err
	}
	names := make(map[string]string, len(users))
	for _, user := range users {
		names[user.ID] = user.FullName
	}

	writer := csv.NewWriter(w)
	if err := writer.Write(supportReportCSVHeader); err != nil {
		return err
	}
	for _, fact := range facts {
		created := fact.CreatedAt
		if !period.contains(&created) {
			continue
		}
		row := make([]string, 0, len(supportReportCSVHeader))
		resolvedBy := ""
		if fact.Status == model.SupportConversationStatusResolved {
			resolvedBy = model.SupportCSATResolvedByHuman
			if supportReportResolvedByAI(fact) {
				resolvedBy = model.SupportCSATResolvedByAI
			}
		}
		firstResponseAt, firstResponseSeconds := "", ""
		if d, ok := supportReportFirstResponse(fact); ok {
			firstResponseAt, firstResponseSeconds = supportReportCSVTime(fact.FirstResponseAt), strconv.FormatInt(int64(d/time.Second), 10)
		}
		firstHumanAt, firstHumanSeconds := "", ""
		if d, ok := supportReportFirstHumanResponse(fact); ok {
			firstHumanAt, firstHumanSeconds = supportReportCSVTime(fact.FirstHumanReplyAt), strconv.FormatInt(int64(d/time.Second), 10)
		}
		resolvedAt, resolutionSeconds := "", ""
		if fact.Status == model.SupportConversationStatusResolved && fact.ResolvedAt != nil {
			resolvedAt = supportReportCSVTime(fact.ResolvedAt)
			resolutionSeconds = strconv.FormatInt(int64(clampSupportReportDuration(fact.ResolvedAt.Sub(fact.CreatedAt))/time.Second), 10)
		}
		csatRating, csatComment := "", ""
		if rating, ok := ratingByConversation[fact.ID]; ok {
			csatRating = strconv.Itoa(rating.Rating)
			csatComment = derefString(rating.Comment)
		}
		assignee := ""
		if fact.AssignedUserID != nil {
			assignee = names[*fact.AssignedUserID]
		}
		row = append(row,
			strconv.Itoa(fact.DisplayID), fact.Subject, fact.Channel, derefString(fact.MailboxName), fact.Status, fact.Priority, assignee,
			derefString(fact.CustomerName), derefString(fact.CustomerEmail), supportReportCSVTime(&created), firstResponseAt, firstResponseSeconds,
			firstHumanAt, firstHumanSeconds, resolvedAt, resolutionSeconds,
			resolvedBy, strconv.FormatBool(fact.AIReplyCount > 0), derefString(fact.AIResolutionType),
			strconv.FormatBool(fact.AIReplyCount > 0 && supportReportHandedOff(fact)), csatRating, csatComment,
			tagNames[fact.ID],
		)
		for i := range row {
			row[i] = supportReportCSVCell(row[i])
		}
		if err := writer.Write(row); err != nil {
			return err
		}
	}
	writer.Flush()
	return writer.Error()
}

func supportReportCSVTime(t *time.Time) string {
	if t == nil {
		return ""
	}
	return t.UTC().Format(time.RFC3339)
}

// supportReportCSVCell stops spreadsheet apps from evaluating customer text
// as a formula.
func supportReportCSVCell(value string) string {
	if value == "" {
		return value
	}
	switch value[0] {
	case '=', '+', '-', '@', '\t', '\r':
		return "'" + value
	}
	return value
}

func firstNonEmptyPointer(values ...*string) string {
	for _, value := range values {
		if value != nil && strings.TrimSpace(*value) != "" {
			return strings.TrimSpace(*value)
		}
	}
	return ""
}
