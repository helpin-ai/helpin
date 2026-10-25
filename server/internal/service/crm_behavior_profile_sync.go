package service

import (
	"context"
	"database/sql"
	"log/slog"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

const (
	// _crmBehaviorSyncInterval is how often each workspace's profiles are rebuilt.
	_crmBehaviorSyncInterval = time.Hour
	// _crmBehaviorRecentWindow is the window sessions_30d counts sessions in.
	_crmBehaviorRecentWindow = 30 * 24 * time.Hour
	// _crmBehaviorSyncTimeout bounds one workspace sync; it stays below the
	// lease so a slow sync is cancelled before another replica can take over.
	_crmBehaviorSyncTimeout = 45 * time.Minute
	_crmBehaviorSyncLease   = time.Hour
	// _crmBehaviorMaxSessions caps sessions_30d, matching the per-key cap
	// the ClickHouse reader applies.
	_crmBehaviorMaxSessions = 500
	// _crmBehaviorMaxAnonymousIDs bounds the linked anonymous ids one sync
	// inlines into the ClickHouse query; the most recently linked are kept.
	// The driver binds arguments client-side, so each id adds ~40 bytes of
	// query text and ClickHouse rejects queries over max_query_size (256 KiB
	// by default).
	_crmBehaviorMaxAnonymousIDs = 5000
)

// crmBehaviorLeaser serializes one workspace sync across API replicas.
type crmBehaviorLeaser interface {
	TryAcquireSignalEvaluatorLease(ctx context.Context, cadence, owner string, now time.Time, leaseFor time.Duration, initialWatermark time.Time) (time.Time, bool, error)
	ReleaseSignalEvaluatorLease(ctx context.Context, cadence, owner string, watermark time.Time) error
	AbandonSignalEvaluatorLease(ctx context.Context, cadence, owner string) error
}

// CRMBehaviorProfileSynchronizer rebuilds CRM behavior profiles from
// helpin.events: first and last activity plus recent sessions for every
// contact and company that product events can be tied to.
type CRMBehaviorProfileSynchronizer struct {
	profiles *repository.CRMBehaviorProfileRepository
	leases   crmBehaviorLeaser
	projects repository.EventWorkspaceResolver
	eventsDB *sql.DB
	owner    string
}

// NewCRMBehaviorProfileSynchronizer creates the synchronizer.
func NewCRMBehaviorProfileSynchronizer(
	profiles *repository.CRMBehaviorProfileRepository,
	leases crmBehaviorLeaser,
	projects repository.EventWorkspaceResolver,
	eventsDB *sql.DB,
) *CRMBehaviorProfileSynchronizer {
	return &CRMBehaviorProfileSynchronizer{
		profiles: profiles, leases: leases, projects: projects, eventsDB: eventsDB, owner: uuid.NewString(),
	}
}

// CRMBehaviorSyncResult summarizes one workspace sync.
type CRMBehaviorSyncResult struct {
	// TotalContacts and TotalCompanies count the workspace's CRM records.
	TotalContacts  int
	TotalCompanies int
	// Contacts counts contact profiles written.
	Contacts int
	// Companies counts company profiles written.
	Companies int
	// IdentifiedKeys counts user, email, and company keys seen in events.
	IdentifiedKeys int
	// UnmatchedKeys counts identified keys that matched no CRM record.
	UnmatchedKeys int
	// EmailOnlyContacts counts contacts matched only by email.
	EmailOnlyContacts int
	// MatchMethods counts profiles per match method; a profile reached
	// several ways counts under each.
	MatchMethods   map[string]int
	LinksTruncated bool
}

// Run syncs every event workspace now and then every interval until ctx ends.
func (s *CRMBehaviorProfileSynchronizer) Run(ctx context.Context) {
	if s == nil || s.profiles == nil || s.leases == nil || s.projects == nil || s.eventsDB == nil {
		return
	}
	s.runAndLog(ctx)
	ticker := time.NewTicker(_crmBehaviorSyncInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			s.runAndLog(ctx)
		case <-ctx.Done():
			return
		}
	}
}

func (s *CRMBehaviorProfileSynchronizer) runAndLog(ctx context.Context) {
	workspaceIDs, err := s.projects.ListAllActiveEventWorkspaceIDs(ctx)
	if err != nil {
		slog.WarnContext(ctx, "list workspaces for CRM behavior profiles failed", "error", err)
		return
	}
	// The list is complete when it loads, so profiles outside it are stale.
	if removed, err := s.profiles.DeleteProfilesOutsideWorkspaces(ctx, workspaceIDs); err != nil {
		slog.WarnContext(ctx, "delete stale CRM behavior profiles failed", "error", err)
	} else if removed > 0 {
		slog.InfoContext(ctx, "deleted CRM behavior profiles of workspaces without events", "rows", removed)
	}
	for _, workspaceID := range workspaceIDs {
		if ctx.Err() != nil {
			return
		}
		s.runWorkspaceAndLog(ctx, workspaceID, time.Now().UTC())
	}
}

func (s *CRMBehaviorProfileSynchronizer) runWorkspaceAndLog(ctx context.Context, workspaceID string, now time.Time) {
	leaseKey := "crm_behavior_profile_sync:" + workspaceID
	watermark, acquired, err := s.leases.TryAcquireSignalEvaluatorLease(
		ctx, leaseKey, s.owner, now, _crmBehaviorSyncLease, now.Add(-2*_crmBehaviorSyncInterval),
	)
	if err != nil {
		slog.WarnContext(ctx, "CRM behavior profile lease failed", "error", err, "workspace_id", workspaceID)
		return
	}
	if !acquired {
		return
	}
	// Another replica synced recently: keep its watermark and skip.
	if now.Sub(watermark) < _crmBehaviorSyncInterval-5*time.Minute {
		if err := s.leases.ReleaseSignalEvaluatorLease(ctx, leaseKey, s.owner, watermark); err != nil {
			slog.WarnContext(ctx, "CRM behavior profile lease release failed", "error", err, "workspace_id", workspaceID)
		}
		return
	}
	syncCtx, cancel := context.WithTimeout(ctx, _crmBehaviorSyncTimeout)
	result, err := s.SyncWorkspace(syncCtx, workspaceID, now)
	cancel()
	if err != nil {
		if abandonErr := s.leases.AbandonSignalEvaluatorLease(ctx, leaseKey, s.owner); abandonErr != nil {
			slog.WarnContext(ctx, "CRM behavior profile lease abandon failed", "error", abandonErr, "workspace_id", workspaceID)
		}
		slog.WarnContext(ctx, "CRM behavior profile sync failed", "error", err, "workspace_id", workspaceID)
		return
	}
	if err := s.leases.ReleaseSignalEvaluatorLease(ctx, leaseKey, s.owner, now); err != nil {
		slog.WarnContext(ctx, "CRM behavior profile lease release failed", "error", err, "workspace_id", workspaceID)
		return
	}
	slog.InfoContext(ctx, "CRM behavior profile sync complete",
		"workspace_id", workspaceID, "contacts", result.Contacts, "companies", result.Companies,
		"identified_keys", result.IdentifiedKeys, "unmatched_keys", result.UnmatchedKeys,
		"email_only_contacts", result.EmailOnlyContacts, "links_truncated", result.LinksTruncated)
}

// SyncWorkspace rebuilds one workspace's profiles from events before now.
func (s *CRMBehaviorProfileSynchronizer) SyncWorkspace(ctx context.Context, workspaceID string, now time.Time) (*CRMBehaviorSyncResult, error) {
	profiles, result, err := s.ComputeWorkspace(ctx, workspaceID, now)
	if err != nil {
		return nil, err
	}
	if err := s.profiles.ReplaceWorkspaceProfiles(ctx, workspaceID, profiles); err != nil {
		return nil, err
	}
	return result, nil
}

// ComputeWorkspace builds one workspace's profiles without writing them, so
// coverage can be measured read-only.
func (s *CRMBehaviorProfileSynchronizer) ComputeWorkspace(
	ctx context.Context, workspaceID string, now time.Time,
) ([]model.CRMBehaviorProfile, *CRMBehaviorSyncResult, error) {
	index, err := s.profiles.LoadIdentityIndex(ctx, workspaceID)
	if err != nil {
		return nil, nil, err
	}
	reader, err := repository.NewClickHouseEventRepository(s.eventsDB, s.projects, workspaceID)
	if err != nil {
		return nil, nil, err
	}
	anonymousIDs, truncated := linkedAnonymousIDs(index, _crmBehaviorMaxAnonymousIDs)
	stats, err := reader.ListBehaviorKeyStats(ctx, now, _crmBehaviorRecentWindow, anonymousIDs)
	if err != nil {
		return nil, nil, err
	}
	profiles, result := buildCRMBehaviorProfiles(workspaceID, index, stats, now)
	result.LinksTruncated = truncated
	return profiles, result, nil
}

// linkedAnonymousIDs returns up to limit distinct anonymous ids from the
// links, most recently linked first, and whether older ids were left out.
// Only the anonymous ids are capped: user-id links still match in full.
func linkedAnonymousIDs(index *repository.CRMBehaviorIdentityIndex, limit int) ([]string, bool) {
	seen := map[string]bool{}
	var result []string
	for _, link := range index.Links {
		id := strings.TrimSpace(link.AnonymousID)
		if id == "" || seen[id] {
			continue
		}
		if len(result) == limit {
			return result, true
		}
		seen[id] = true
		result = append(result, id)
	}
	return result, false
}

type crmBehaviorAccumulator struct {
	first, last time.Time
	sessions    map[string]bool
	methods     map[string]bool
}

func (a *crmBehaviorAccumulator) add(stats repository.BehaviorKeyStats, method string) {
	if a.first.IsZero() || stats.FirstSeenAt.Before(a.first) {
		a.first = stats.FirstSeenAt
	}
	if stats.LastSeenAt.After(a.last) {
		a.last = stats.LastSeenAt
	}
	for _, session := range stats.RecentSessionIDs {
		a.sessions[session] = true
	}
	a.methods[method] = true
}

type crmBehaviorMatch struct {
	entityType, entityID, method string
}

// buildCRMBehaviorProfiles ties identity-key stats to CRM records. A record
// reached through several keys merges them: earliest first seen, latest last
// seen, and the union of recent sessions.
func buildCRMBehaviorProfiles(
	workspaceID string, index *repository.CRMBehaviorIdentityIndex, stats []repository.BehaviorKeyStats, now time.Time,
) ([]model.CRMBehaviorProfile, *CRMBehaviorSyncResult) {
	result := &CRMBehaviorSyncResult{
		TotalContacts: len(index.ContactIDs), TotalCompanies: index.CompanyCount,
		MatchMethods: map[string]int{},
	}
	byUser, byAnonymous := crmBehaviorLinkIndexes(index.Links)
	accumulators := map[[2]string]*crmBehaviorAccumulator{}
	for _, stat := range stats {
		matches := crmBehaviorMatchesForKey(stat.Key, index, byUser, byAnonymous)
		if !strings.HasPrefix(stat.Key, repository.BehaviorKeyAnonymousID) {
			result.IdentifiedKeys++
			if len(matches) == 0 {
				result.UnmatchedKeys++
			}
		}
		for _, match := range matches {
			id := [2]string{match.entityType, match.entityID}
			accumulator := accumulators[id]
			if accumulator == nil {
				accumulator = &crmBehaviorAccumulator{sessions: map[string]bool{}, methods: map[string]bool{}}
				accumulators[id] = accumulator
			}
			accumulator.add(stat, match.method)
		}
	}
	profiles := make([]model.CRMBehaviorProfile, 0, len(accumulators))
	for id, accumulator := range accumulators {
		methods := make([]string, 0, len(accumulator.methods))
		for method := range accumulator.methods {
			methods = append(methods, method)
		}
		sort.Strings(methods)
		for _, method := range methods {
			result.MatchMethods[id[0]+":"+method]++
		}
		sessions := min(len(accumulator.sessions), _crmBehaviorMaxSessions)
		profiles = append(profiles, model.CRMBehaviorProfile{
			EntityType: id[0], EntityID: id[1], WorkspaceID: workspaceID,
			FirstSeenAt: accumulator.first, LastSeenAt: accumulator.last, Sessions30d: sessions,
			MatchMethods: strings.Join(methods, ","), ComputedAt: now.UTC(),
		})
		if id[0] == model.CRMFieldObjectContact {
			result.Contacts++
			if len(methods) == 1 && methods[0] == model.CRMBehaviorMatchEmail {
				result.EmailOnlyContacts++
			}
		} else {
			result.Companies++
		}
	}
	sort.Slice(profiles, func(i, j int) bool {
		if profiles[i].EntityType != profiles[j].EntityType {
			return profiles[i].EntityType < profiles[j].EntityType
		}
		return profiles[i].EntityID < profiles[j].EntityID
	})
	return profiles, result
}

func crmBehaviorLinkIndexes(links []repository.CRMBehaviorIdentityLink) (map[string][]repository.CRMBehaviorIdentityLink, map[string][]repository.CRMBehaviorIdentityLink) {
	byUser := map[string][]repository.CRMBehaviorIdentityLink{}
	byAnonymous := map[string][]repository.CRMBehaviorIdentityLink{}
	for _, link := range links {
		if link.ExternalUserID != nil && strings.TrimSpace(*link.ExternalUserID) != "" {
			key := strings.TrimSpace(*link.ExternalUserID)
			byUser[key] = append(byUser[key], link)
		}
		if anonymous := strings.TrimSpace(link.AnonymousID); anonymous != "" {
			byAnonymous[anonymous] = append(byAnonymous[anonymous], link)
		}
	}
	return byUser, byAnonymous
}

func crmBehaviorMatchesForKey(
	key string, index *repository.CRMBehaviorIdentityIndex,
	byUser, byAnonymous map[string][]repository.CRMBehaviorIdentityLink,
) []crmBehaviorMatch {
	var matches []crmBehaviorMatch
	// Links only count for records the workspace owns.
	linkMatches := func(links []repository.CRMBehaviorIdentityLink) {
		for _, link := range links {
			if link.ContactID != nil && index.ContactIDs[strings.ToLower(*link.ContactID)] {
				matches = append(matches, crmBehaviorMatch{model.CRMFieldObjectContact, *link.ContactID, model.CRMBehaviorMatchIdentityLink})
			}
			if link.CompanyID != nil && index.CompanyKeyMethods[strings.ToLower(*link.CompanyID)] == model.CRMBehaviorMatchRecordID {
				matches = append(matches, crmBehaviorMatch{model.CRMFieldObjectCompany, *link.CompanyID, model.CRMBehaviorMatchIdentityLink})
			}
		}
	}
	switch {
	case strings.HasPrefix(key, repository.BehaviorKeyUserID):
		userID := strings.TrimPrefix(key, repository.BehaviorKeyUserID)
		linkMatches(byUser[userID])
		if index.ContactIDs[strings.ToLower(userID)] {
			matches = append(matches, crmBehaviorMatch{model.CRMFieldObjectContact, strings.ToLower(userID), model.CRMBehaviorMatchRecordID})
		}
	case strings.HasPrefix(key, repository.BehaviorKeyAnonymousID):
		linkMatches(byAnonymous[strings.TrimPrefix(key, repository.BehaviorKeyAnonymousID)])
	case strings.HasPrefix(key, repository.BehaviorKeyEmail):
		// An address shared by several contacts cannot say which one acted.
		if contacts := index.ContactEmails[strings.TrimPrefix(key, repository.BehaviorKeyEmail)]; len(contacts) == 1 {
			matches = append(matches, crmBehaviorMatch{model.CRMFieldObjectContact, contacts[0], model.CRMBehaviorMatchEmail})
		}
	case strings.HasPrefix(key, repository.BehaviorKeyCompanyID):
		companyKey := strings.TrimPrefix(key, repository.BehaviorKeyCompanyID)
		companyID, ok := index.CompanyKeys[companyKey]
		if !ok {
			companyKey = strings.ToLower(companyKey)
			companyID, ok = index.CompanyKeys[companyKey]
		}
		if ok {
			matches = append(matches, crmBehaviorMatch{model.CRMFieldObjectCompany, companyID, index.CompanyKeyMethods[companyKey]})
		}
	}
	return matches
}
