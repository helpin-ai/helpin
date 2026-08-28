package service

import (
	"context"
	"crypto/sha256"
	"fmt"
	"log/slog"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/helpin-ai/helpin/server/internal/model"
)

type signalScoringProfile struct {
	version                int
	heuristic              bool
	signalWeights          map[string]float64
	halfLives              map[string]float64
	domainWeights          map[string]float64
	identityTrust          map[string]float64
	compoundWindowDays     float64
	compoundBoostPerDomain float64
	maxCompoundBoost       float64
	rules                  map[string]model.CRMSignalRuleConfig
}

func defaultSignalScoringProfile() signalScoringProfile {
	return signalScoringProfile{
		version: 2, heuristic: true,
		signalWeights: map[string]float64{
			model.CRMSignalBuyingIntent: 15, model.CRMSignalBudgetSignal: 12,
			model.CRMSignalTimelineSignal: 10, model.CRMSignalChampionSignal: 12,
			model.CRMSignalObjection: 12, model.CRMSignalCompetitorMention: 8,
			model.CRMSignalRiskSignal: 20,
		},
		halfLives: map[string]float64{
			model.CRMSignalBuyingIntent: 7, model.CRMSignalBudgetSignal: 21,
			model.CRMSignalTimelineSignal: 10, model.CRMSignalChampionSignal: 180,
			model.CRMSignalObjection: 30, model.CRMSignalCompetitorMention: 45,
			model.CRMSignalRiskSignal: 30,
		},
		domainWeights: map[string]float64{
			model.CRMSignalDomainConversation: 1, model.CRMSignalDomainWebBehavior: 1.1,
			model.CRMSignalDomainProductUsage: 1.1, model.CRMSignalDomainSupport: 1.15,
			model.CRMSignalDomainDelivery: 1, model.CRMSignalDomainRelationship: 1.1,
			model.CRMSignalDomainMarket: 0.7,
		},
		identityTrust: map[string]float64{
			"verified": 1, "probabilistic": 0.65, "untrusted": 0.35, "unknown": 0.5,
		},
		compoundWindowDays: 14, compoundBoostPerDomain: 0.15, maxCompoundBoost: 0.45,
		rules: map[string]model.CRMSignalRuleConfig{},
	}
}

func (s *CRMSignalService) loadSignalScoringProfile(ctx context.Context, workspaceID string) signalScoringProfile {
	profile := defaultSignalScoringProfile()
	config, err := s.signalRepo.GetLatestSignalScoringConfig(ctx, workspaceID)
	if err != nil {
		slog.WarnContext(ctx, "using default CRM signal scoring profile", "error", err, "workspace_id", workspaceID)
	} else {
		profile.version, profile.heuristic = max(profile.version, config.Version), config.Heuristic
		mergeFloatMap(profile.signalWeights, config.Parameters["signal_weights"])
		mergeFloatMap(profile.halfLives, config.Parameters["half_lives_days"])
		mergeFloatMap(profile.domainWeights, config.Parameters["domain_weights"])
		mergeFloatMap(profile.identityTrust, config.Parameters["identity_trust"])
		profile.compoundWindowDays = jsonFloat(config.Parameters["compound_window_days"], profile.compoundWindowDays)
		profile.compoundBoostPerDomain = jsonFloat(config.Parameters["compound_boost_per_domain"], profile.compoundBoostPerDomain)
		profile.maxCompoundBoost = jsonFloat(config.Parameters["max_compound_boost"], profile.maxCompoundBoost)
	}
	if rules, ruleErr := s.signalRepo.ListLatestRuleScoringConfigs(ctx, workspaceID); ruleErr == nil {
		profile.rules = rules
	} else {
		slog.WarnContext(ctx, "CRM rule scoring overrides unavailable", "error", ruleErr, "workspace_id", workspaceID)
	}
	return profile
}

func mergeFloatMap(target map[string]float64, raw interface{}) {
	values, ok := raw.(map[string]interface{})
	if !ok {
		return
	}
	for key, value := range values {
		if number := jsonFloat(value, 0); number > 0 {
			target[key] = number
		}
	}
}

func jsonFloat(value interface{}, fallback float64) float64 {
	switch number := value.(type) {
	case float64:
		return number
	case float32:
		return float64(number)
	case int:
		return float64(number)
	case int64:
		return float64(number)
	default:
		return fallback
	}
}

func (profile signalScoringProfile) scoreSignal(signal *model.CRMBuyerSignal, now time.Time) {
	weight, halfLife := profile.signalWeights[signal.SignalType], profile.halfLives[signal.SignalType]
	if signal.BusinessWeightSnapshot > 0 {
		weight = signal.BusinessWeightSnapshot
	} else if signal.RuleKey != nil {
		if rule, ok := profile.rules[*signal.RuleKey]; ok {
			weight = rule.BusinessWeight
		}
	}
	if signal.HalfLifeDaysSnapshot > 0 {
		halfLife = signal.HalfLifeDaysSnapshot
	} else if signal.RuleKey != nil {
		if rule, ok := profile.rules[*signal.RuleKey]; ok {
			halfLife = rule.HalfLifeDays
		}
	}
	if weight <= 0 {
		weight = 8
	}
	if halfLife <= 0 {
		halfLife = 30
	}
	domainFactor := profile.domainWeights[signal.SignalDomain]
	if domainFactor <= 0 {
		domainFactor = 1
	}
	trustFactor := profile.identityTrust[strings.ToLower(signal.EvidenceIdentityTrust)]
	if trustFactor <= 0 {
		trustFactor = profile.identityTrust["unknown"]
	}
	confidence := math.Max(0, math.Min(1, signal.Confidence))
	if signal.SourceType == model.CRMSignalSourceManual && confidence == 0 {
		confidence = 1
	}
	ageDays := math.Max(0, now.Sub(signal.DetectedAt).Hours()/24)
	recencyFactor := math.Pow(0.5, ageDays/halfLife)
	entityFactor := signalEntityMultiplier(signal)
	direction := signalDirection(signal)
	priority := math.Min(100, weight*domainFactor*recencyFactor*entityFactor*trustFactor*confidence)
	signal.BusinessPriority = roundScore(priority)
	signal.SignedImpact = roundScore(priority * direction)
	signal.Severity = signalSeverity(priority)
	signal.ScoreVersion = profile.version
	signal.ScoreFactors = model.JSONB{
		"business_weight": weight, "extraction_confidence": roundScore(confidence),
		"domain_weight": roundScore(domainFactor), "identity_trust": roundScore(trustFactor),
		"recency_factor": roundScore(recencyFactor), "half_life_days": halfLife,
		"entity_multiplier": roundScore(entityFactor), "polarity_direction": direction,
		"heuristic": profile.heuristic,
	}
}

func signalDirection(signal *model.CRMBuyerSignal) float64 {
	switch signal.Polarity {
	case model.CRMSignalPolarityNegative:
		return -1
	case model.CRMSignalPolarityPositive:
		return 1
	case model.CRMSignalPolarityNeutral:
		return 0
	}
	switch signal.SignalType {
	case model.CRMSignalObjection, model.CRMSignalCompetitorMention, model.CRMSignalRiskSignal:
		return -1
	default:
		return 1
	}
}

func signalEntityMultiplier(signal *model.CRMBuyerSignal) float64 {
	multiplier := 1.0
	if signal.DealStageProbability != nil {
		multiplier *= 0.8 + math.Max(0, math.Min(100, float64(*signal.DealStageProbability)))/250
	}
	if signal.DealAmount != nil && *signal.DealAmount > 0 {
		multiplier *= 1 + math.Min(0.25, math.Log10(*signal.DealAmount+1)/25)
	}
	return multiplier
}

func signalSeverity(priority float64) string {
	switch {
	case priority >= 15:
		return "high"
	case priority >= 8:
		return "medium"
	default:
		return "low"
	}
}

func roundScore(value float64) float64 { return math.Round(value*100) / 100 }

// ListWorkspaceSignalFeed returns ranked account stories with their source evidence.
func (s *CRMSignalService) ListWorkspaceSignalFeed(ctx context.Context, workspaceID string, filters model.CRMBuyerSignalListFilters, pagination model.PMPagination) (*model.CRMSignalWorkspaceFeed, error) {
	return s.listWorkspaceSignalFeed(ctx, workspaceID, filters, pagination, nil, false)
}

// ListWorkspaceSignalShadowPreview exposes shadow composition only to the
// admin-only preview route.
func (s *CRMSignalService) ListWorkspaceSignalShadowPreview(ctx context.Context, workspaceID string, filters model.CRMBuyerSignalListFilters, pagination model.PMPagination, lanePages map[string]int) (*model.CRMSignalWorkspaceFeed, error) {
	return s.listWorkspaceSignalFeed(ctx, workspaceID, filters, pagination, lanePages, true)
}

// ListWorkspaceSignalLanes returns independently paginated motion queues.
func (s *CRMSignalService) ListWorkspaceSignalLanes(ctx context.Context, workspaceID string, filters model.CRMBuyerSignalListFilters, pagination model.PMPagination, lanePages map[string]int) (*model.CRMSignalWorkspaceFeed, error) {
	return s.listWorkspaceSignalFeed(ctx, workspaceID, filters, pagination, lanePages, false)
}

func (s *CRMSignalService) listWorkspaceSignalFeed(ctx context.Context, workspaceID string, filters model.CRMBuyerSignalListFilters, pagination model.PMPagination, lanePages map[string]int, includeShadow bool) (*model.CRMSignalWorkspaceFeed, error) {
	if strings.TrimSpace(workspaceID) == "" {
		return nil, fmt.Errorf("workspace_id is required")
	}
	rollout, err := s.signalRepo.GetSignalRolloutSettings(ctx, workspaceID)
	if err != nil {
		return nil, err
	}
	if !includeShadow && rollout.Mode != model.CRMSignalRolloutLive {
		return &model.CRMSignalWorkspaceFeed{Data: []model.CRMSignalAccountStory{}, Lanes: []model.CRMSignalLane{}, RolloutMode: rollout.Mode}, nil
	}
	now := time.Now().UTC()
	signals, err := s.signalRepo.ListWorkspaceSignalCandidates(ctx, workspaceID, filters, now, 0)
	if err != nil {
		return nil, err
	}
	profile := s.loadSignalScoringProfile(ctx, workspaceID)
	for index := range signals {
		profile.scoreSignal(&signals[index], now)
	}
	stories := composeSignalStories(signals, profile, now)
	routingSettings, err := s.signalRepo.GetSignalRoutingSettings(ctx, workspaceID)
	if err != nil {
		return nil, err
	}
	if routingSettings.MinimumLanePriority > 0 {
		filtered := stories[:0]
		for _, story := range stories {
			if story.Priority >= routingSettings.MinimumLanePriority {
				filtered = append(filtered, story)
			}
		}
		stories = filtered
	}
	if filters.Severity != nil && *filters.Severity != "" {
		filtered := stories[:0]
		for _, story := range stories {
			if story.Severity == *filters.Severity {
				filtered = append(filtered, story)
			}
		}
		stories = filtered
	}
	page, perPage := pagination.Page, pagination.PerPage
	if page < 1 {
		page = 1
	}
	if perPage < 1 || perPage > 100 {
		perPage = 25
	}
	lanes, flattened := paginateSignalLanes(stories, page, perPage, lanePages)
	return &model.CRMSignalWorkspaceFeed{
		Data: flattened, Lanes: lanes, Total: len(stories), Page: page, ScoreVersion: profile.version,
		Heuristic: profile.heuristic, MinimumLanePriority: routingSettings.MinimumLanePriority, RolloutMode: rollout.Mode,
	}, nil
}

func paginateSignalLanes(stories []model.CRMSignalAccountStory, page, perPage int, lanePages map[string]int) ([]model.CRMSignalLane, []model.CRMSignalAccountStory) {
	motionOrder := []string{
		model.CRMCommercialMotionProspecting, model.CRMCommercialMotionConversion,
		model.CRMCommercialMotionOnboarding, model.CRMCommercialMotionAdoption,
		model.CRMCommercialMotionExpansion, model.CRMCommercialMotionRenewal,
		model.CRMCommercialMotionRetention,
	}
	byMotion := make(map[string][]model.CRMSignalAccountStory, len(motionOrder))
	for _, story := range stories {
		byMotion[story.CommercialMotion] = append(byMotion[story.CommercialMotion], story)
	}
	lanes := make([]model.CRMSignalLane, 0, len(byMotion))
	flattened := make([]model.CRMSignalAccountStory, 0, len(stories))
	for _, motion := range motionOrder {
		laneStories := byMotion[motion]
		if len(laneStories) == 0 {
			continue
		}
		lanePage := page
		if requested := lanePages[motion]; requested > 0 {
			lanePage = requested
		}
		laneStart := min((lanePage-1)*perPage, len(laneStories))
		laneEnd := min(laneStart+perPage, len(laneStories))
		data := laneStories[laneStart:laneEnd]
		flattened = append(flattened, data...)
		lanes = append(lanes, model.CRMSignalLane{
			CommercialMotion: motion, Data: data, Total: len(laneStories),
			Page: lanePage, PerPage: perPage,
		})
	}
	return lanes, flattened
}

func composeSignalStories(signals []model.CRMBuyerSignal, profile signalScoringProfile, now time.Time) []model.CRMSignalAccountStory {
	grouped := map[string]*model.CRMSignalAccountStory{}
	changedAfter := now.Add(-7 * 24 * time.Hour)
	compoundAfter := now.Add(-time.Duration(profile.compoundWindowDays*24) * time.Hour)
	for _, signal := range signals {
		entityType, entityID, accountName := signalStoryEntity(signal)
		motion := signal.CommercialMotion
		if motion == "" {
			motion = model.CRMCommercialMotionConversion
		}
		key := entityType + ":" + entityID + ":" + motion
		story := grouped[key]
		if story == nil {
			sum := sha256.Sum256([]byte(key))
			story = &model.CRMSignalAccountStory{ID: fmt.Sprintf("%x", sum[:12]), EntityType: entityType, EntityID: entityID, AccountName: accountName, AccountDomain: signal.AccountDomain, OwnerMemberID: signal.OwnerMemberID, CommercialMotion: motion, LatestDetectedAt: signal.DetectedAt, ScoreVersion: profile.version, Signals: []model.CRMBuyerSignal{}}
			grouped[key] = story
		}
		story.Signals = append(story.Signals, signal)
		if signal.DetectedAt.After(story.LatestDetectedAt) {
			story.LatestDetectedAt = signal.DetectedAt
		}
		if signal.DetectedAt.After(changedAfter) {
			story.ChangedSince++
		}
	}
	stories := make([]model.CRMSignalAccountStory, 0, len(grouped))
	for _, story := range grouped {
		observedDomains := map[string]struct{}{}
		sourceGroups := map[string][]model.CRMBuyerSignal{}
		for _, signal := range story.Signals {
			sourceKey := signalEvidenceSourceKey(signal)
			sourceGroups[sourceKey] = append(sourceGroups[sourceKey], signal)
			if !signal.DetectedAt.Before(compoundAfter) {
				observedDomains[signal.SignalDomain] = struct{}{}
			}
		}
		story.EvidenceSourceCount = len(sourceGroups)
		independentDomains := map[string]struct{}{}
		for _, sourceSignals := range sourceGroups {
			priority, signedImpact, representativeDomain, changed := signalSourceContribution(sourceSignals, compoundAfter, changedAfter)
			story.Priority += priority
			story.SignedImpact += signedImpact
			if representativeDomain != "" {
				independentDomains[representativeDomain] = struct{}{}
			}
			if changed {
				story.ChangedEvidenceSourceCount++
			}
		}
		// Priority remains source-correlated, but judgment must retain every
		// directional signal. Collapsing a source to one representative would
		// erase opposing product evidence before the ambiguity band can see it.
		for _, signal := range story.Signals {
			if signal.SignedImpact > 0 {
				story.PositiveStrength += math.Abs(signal.SignedImpact)
			} else if signal.SignedImpact < 0 {
				story.NegativeStrength += math.Abs(signal.SignedImpact)
			}
		}
		story.Domains = make([]string, 0, len(observedDomains))
		for domain := range observedDomains {
			story.Domains = append(story.Domains, domain)
		}
		sort.Strings(story.Domains)
		boost := math.Min(profile.maxCompoundBoost, math.Max(0, float64(len(independentDomains)-1))*profile.compoundBoostPerDomain)
		story.Priority = roundScore(math.Min(100, story.Priority*(1+boost)))
		story.PositiveStrength = roundScore(story.PositiveStrength * (1 + boost))
		story.NegativeStrength = roundScore(story.NegativeStrength * (1 + boost))
		story.SignedImpact = roundScore(story.PositiveStrength - story.NegativeStrength)
		story.Severity = signalSeverity(story.Priority)
		story.Polarity = model.CRMSignalPolarityNeutral
		stronger := math.Max(story.PositiveStrength, story.NegativeStrength)
		weaker := math.Min(story.PositiveStrength, story.NegativeStrength)
		story.NeedsJudgment = stronger > 0 && weaker/stronger >= 0.75
		if !story.NeedsJudgment && story.PositiveStrength > story.NegativeStrength {
			story.Polarity = model.CRMSignalPolarityPositive
		} else if !story.NeedsJudgment && story.NegativeStrength > story.PositiveStrength {
			story.Polarity = model.CRMSignalPolarityNegative
		}
		story.ScoreFactors = model.JSONB{
			"independent_domains": len(independentDomains), "observed_domains": len(observedDomains),
			"compound_boost": roundScore(boost), "signal_count": len(story.Signals),
			"evidence_source_count":   story.EvidenceSourceCount,
			"correlated_signal_count": len(story.Signals) - story.EvidenceSourceCount,
			"source_correlation":      "canonical_source_v1", "heuristic": profile.heuristic,
		}
		story.ChangeSummary = signalStoryChangeSummary(*story)
		sort.SliceStable(story.Signals, func(i, j int) bool { return story.Signals[i].BusinessPriority > story.Signals[j].BusinessPriority })
		if !story.NeedsJudgment {
			for _, signal := range story.Signals {
				if signal.RecommendedActionKey == nil {
					continue
				}
				if story.Polarity == model.CRMSignalPolarityPositive && signal.SignedImpact < 0 ||
					story.Polarity == model.CRMSignalPolarityNegative && signal.SignedImpact > 0 {
					continue
				}
				story.RecommendedActionKey = signal.RecommendedActionKey
				story.RecommendedActionLabel = signal.RecommendedActionLabel
				break
			}
		}
		for _, signal := range story.Signals {
			if signal.DirectionChangedBySupersession {
				story.DirectionChangedBySupersession = true
				break
			}
		}
		stories = append(stories, *story)
	}
	activeMotions := make(map[string][]string)
	for _, story := range stories {
		key := story.EntityType + ":" + story.EntityID
		activeMotions[key] = append(activeMotions[key], story.CommercialMotion)
	}
	for index := range stories {
		key := stories[index].EntityType + ":" + stories[index].EntityID
		for _, motion := range activeMotions[key] {
			if motion != stories[index].CommercialMotion {
				stories[index].OtherActiveMotions = append(stories[index].OtherActiveMotions, motion)
			}
		}
		sort.Strings(stories[index].OtherActiveMotions)
	}
	sort.SliceStable(stories, func(i, j int) bool {
		if stories[i].Priority == stories[j].Priority {
			return stories[i].LatestDetectedAt.After(stories[j].LatestDetectedAt)
		}
		return stories[i].Priority > stories[j].Priority
	})
	return stories
}

func signalEvidenceSourceKey(signal model.CRMBuyerSignal) string {
	sourceType := strings.ToLower(strings.TrimSpace(signal.SourceType))
	if sourceType == "" {
		sourceType = "unknown"
	}
	if signal.SourceThreadID != nil && strings.TrimSpace(*signal.SourceThreadID) != "" {
		return sourceType + ":thread:" + strings.TrimSpace(*signal.SourceThreadID)
	}
	if signal.SourceID != nil && strings.TrimSpace(*signal.SourceID) != "" {
		kind := "source"
		if sourceType == model.CRMSignalSourceSupport {
			kind = "thread"
		}
		return sourceType + ":" + kind + ":" + strings.TrimSpace(*signal.SourceID)
	}
	if strings.TrimSpace(signal.EvidenceFingerprint) != "" {
		return sourceType + ":evidence:" + strings.TrimSpace(signal.EvidenceFingerprint)
	}
	return "signal:" + signal.ID
}

func signalSourceContribution(signals []model.CRMBuyerSignal, compoundAfter, changedAfter time.Time) (float64, float64, string, bool) {
	var prioritySignal, directionalSignal *model.CRMBuyerSignal
	var recentPrioritySignal, recentDirectionalSignal *model.CRMBuyerSignal
	changed := false
	for index := range signals {
		signal := &signals[index]
		if prioritySignal == nil || signal.BusinessPriority > prioritySignal.BusinessPriority ||
			(signal.BusinessPriority == prioritySignal.BusinessPriority && signal.DetectedAt.After(prioritySignal.DetectedAt)) {
			prioritySignal = signal
		}
		if math.Abs(signal.SignedImpact) > 0.01 && (directionalSignal == nil || math.Abs(signal.SignedImpact) > math.Abs(directionalSignal.SignedImpact) ||
			(math.Abs(signal.SignedImpact) == math.Abs(directionalSignal.SignedImpact) && signal.DetectedAt.After(directionalSignal.DetectedAt))) {
			directionalSignal = signal
		}
		if !signal.DetectedAt.Before(compoundAfter) {
			if recentPrioritySignal == nil || signal.BusinessPriority > recentPrioritySignal.BusinessPriority ||
				(signal.BusinessPriority == recentPrioritySignal.BusinessPriority && signal.DetectedAt.After(recentPrioritySignal.DetectedAt)) {
				recentPrioritySignal = signal
			}
			if math.Abs(signal.SignedImpact) > 0.01 && (recentDirectionalSignal == nil || math.Abs(signal.SignedImpact) > math.Abs(recentDirectionalSignal.SignedImpact) ||
				(math.Abs(signal.SignedImpact) == math.Abs(recentDirectionalSignal.SignedImpact) && signal.DetectedAt.After(recentDirectionalSignal.DetectedAt))) {
				recentDirectionalSignal = signal
			}
		}
		if signal.DetectedAt.After(changedAfter) {
			changed = true
		}
	}
	if prioritySignal == nil {
		return 0, 0, "", changed
	}
	priority := prioritySignal.BusinessPriority
	signedImpact := 0.0
	if directionalSignal != nil {
		signedImpact = math.Max(-priority, math.Min(priority, directionalSignal.SignedImpact))
	}
	representative := recentDirectionalSignal
	if representative == nil {
		representative = recentPrioritySignal
	}
	domain := ""
	if representative != nil {
		domain = representative.SignalDomain
	}
	return priority, signedImpact, domain, changed
}

func signalStoryEntity(signal model.CRMBuyerSignal) (string, string, string) {
	if signal.CompanyID != nil && *signal.CompanyID != "" {
		return "company", *signal.CompanyID, firstSignalValue(signal.AccountName, "Unnamed account")
	}
	if signal.DealID != nil && *signal.DealID != "" {
		return "deal", *signal.DealID, firstSignalValue(signal.DealName, "Unnamed deal")
	}
	if signal.ContactID != nil && *signal.ContactID != "" {
		return "contact", *signal.ContactID, firstSignalValue(signal.ContactName, "Unnamed contact")
	}
	return "unresolved", signal.ID, "Unresolved account"
}

func signalStoryChangeSummary(story model.CRMSignalAccountStory) string {
	if story.ChangedEvidenceSourceCount == 0 {
		return "No new evidence in the last 7 days"
	}
	domainLabel := "domain"
	if len(story.Domains) != 1 {
		domainLabel = "domains"
	}
	sourceLabel := "evidence source"
	if signalStoryUsesSupportConversations(story.Signals) {
		sourceLabel = "conversation"
	}
	return fmt.Sprintf("%d new %s%s · %d signal%s across %d %s", story.ChangedEvidenceSourceCount,
		sourceLabel, plural(story.ChangedEvidenceSourceCount), story.ChangedSince, plural(story.ChangedSince), len(story.Domains), domainLabel)
}

func signalStoryUsesSupportConversations(signals []model.CRMBuyerSignal) bool {
	if len(signals) == 0 {
		return false
	}
	for _, signal := range signals {
		if signal.SourceType != model.CRMSignalSourceSupport ||
			((signal.SourceThreadID == nil || strings.TrimSpace(*signal.SourceThreadID) == "") &&
				(signal.SourceID == nil || strings.TrimSpace(*signal.SourceID) == "")) {
			return false
		}
	}
	return true
}

func plural(count int) string {
	if count == 1 {
		return ""
	}
	return "s"
}

func firstSignalValue(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}
