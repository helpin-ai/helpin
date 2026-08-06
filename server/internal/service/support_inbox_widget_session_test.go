package service

import (
	"context"
	"net/netip"
	"strings"
	"testing"
	"time"

	"github.com/helpin-ai/helpin/server/internal/geoip"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
	"github.com/helpin-ai/helpin/server/internal/requestmeta"
)

func TestSupportInboxServiceWidgetSessionLifecycle(t *testing.T) {
	db := newTestDB(t)

	const (
		workspaceID = "ws-widget-lifecycle"
		widgetKey   = "wk_widget_lifecycle"
	)

	seedWorkspace(t, db, workspaceID, "Widget Lifecycle WS", "widget-lifecycle", "user-123")

	ctx := context.Background()
	installationRepo := repository.NewSupportInboxInstallationRepository(db)
	sessionRepo := repository.NewSupportInboxSessionRepository(db)

	if err := installationRepo.Create(ctx, &model.SupportWidgetInstallation{
		WorkspaceID: workspaceID,
		WidgetKey:   widgetKey,
		SecretKey:   "sk_widget_lifecycle",
		Settings:    "{}",
		Active:      true,
	}); err != nil {
		t.Fatalf("create installation: %v", err)
	}

	svc := NewSupportInboxService(
		nil,
		nil,
		nil,
		nil,
		nil,
		installationRepo,
		sessionRepo,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
	)
	svc.SetGeoIPResolver(widgetTestGeoIPResolver{
		result: &geoip.Result{
			CountryCode: "US",
			CountryName: "United States",
			RegionName:  "California",
			CityName:    "San Francisco",
		},
	})

	name := "Jane Widget"
	email := "jane@example.com"
	pageURL := "https://example.com/pricing"
	timezone := "America/New_York"
	locale := "en-US"
	ctx = requestmeta.WithClientIP(ctx, requestmeta.ClientIP{
		Addr:   netip.MustParseAddr("198.51.100.8"),
		Source: "cf-connecting-ip",
	})

	identified, err := svc.CreateWidgetSession(ctx, widgetKey, "anon-identified", &name, &email, nil, &pageURL, &timezone, &locale)
	if err != nil {
		t.Fatalf("CreateWidgetSession identified: %v", err)
	}
	if identified.WorkspaceID != workspaceID {
		t.Fatalf("workspace_id = %q, want %q", identified.WorkspaceID, workspaceID)
	}
	if identified.SessionToken == "" || len(identified.SessionToken) != 64 {
		t.Fatalf("session_token = %q, want 64-char token", identified.SessionToken)
	}
	if identified.IsAnonymous {
		t.Fatal("expected identified session to be non-anonymous")
	}
	if identified.CustomerEmail == nil || *identified.CustomerEmail != email {
		t.Fatalf("customer_email = %v, want %q", identified.CustomerEmail, email)
	}
	if identified.LastPageURL == nil || *identified.LastPageURL != pageURL {
		t.Fatalf("last_page_url = %v, want %q", identified.LastPageURL, pageURL)
	}
	if identified.IPAddress == nil || *identified.IPAddress != "198.51.100.8" {
		t.Fatalf("ip_address = %v, want %q", identified.IPAddress, "198.51.100.8")
	}
	if identified.CountryCode == nil || *identified.CountryCode != "US" {
		t.Fatalf("country_code = %v, want %q", identified.CountryCode, "US")
	}
	if identified.CityName == nil || *identified.CityName != "San Francisco" {
		t.Fatalf("city_name = %v, want %q", identified.CityName, "San Francisco")
	}
	if remaining := time.Until(identified.ExpiresAt); remaining < (6*24*time.Hour) || remaining > (8*24*time.Hour) {
		t.Fatalf("expires_at remaining = %v, want about 7 days", remaining)
	}

	fetched, err := svc.GetWidgetSession(ctx, identified.SessionToken)
	if err != nil {
		t.Fatalf("GetWidgetSession: %v", err)
	}
	if fetched.ID != identified.ID {
		t.Fatalf("session id = %q, want %q", fetched.ID, identified.ID)
	}

	anonymous, err := svc.CreateWidgetSession(ctx, widgetKey, "anon-anonymous", nil, nil, nil, nil, nil, nil)
	if err != nil {
		t.Fatalf("CreateWidgetSession anonymous: %v", err)
	}
	if !anonymous.IsAnonymous {
		t.Fatal("expected anonymous session to be marked anonymous")
	}
	if anonymous.CustomerEmail != nil {
		t.Fatalf("customer_email = %v, want nil", anonymous.CustomerEmail)
	}

	if err := svc.RevokeWidgetSession(ctx, identified.SessionToken); err != nil {
		t.Fatalf("RevokeWidgetSession: %v", err)
	}

	if _, err := svc.GetWidgetSession(ctx, identified.SessionToken); err == nil || !strings.Contains(err.Error(), "session revoked") {
		t.Fatalf("GetWidgetSession after revoke error = %v, want revoked", err)
	}
}

type widgetTestGeoIPResolver struct {
	result *geoip.Result
	err    error
}

func (r widgetTestGeoIPResolver) Lookup(addr netip.Addr) (*geoip.Result, error) {
	if !addr.IsValid() {
		return nil, nil
	}
	return r.result, r.err
}

func TestSupportInboxServiceCreateWidgetSessionRejectsInvalidKey(t *testing.T) {
	db := newTestDB(t)

	ctx := context.Background()
	sessionRepo := repository.NewSupportInboxSessionRepository(db)
	installationRepo := repository.NewSupportInboxInstallationRepository(db)

	svc := NewSupportInboxService(
		nil,
		nil,
		nil,
		nil,
		nil,
		installationRepo,
		sessionRepo,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
	)

	if _, err := svc.CreateWidgetSession(ctx, "wk_missing", "anon-1", nil, nil, nil, nil, nil, nil); err == nil || !strings.Contains(err.Error(), "invalid widget key") {
		t.Fatalf("CreateWidgetSession error = %v, want invalid widget key", err)
	}
}

func TestSupportInboxServiceBackfillWidgetSessionGeo(t *testing.T) {
	db := newTestDB(t)

	const workspaceID = "ws-widget-geo-backfill"
	seedWorkspace(t, db, workspaceID, "Widget Geo Backfill WS", "widget-geo-backfill", "user-123")

	ctx := context.Background()
	sessionRepo := repository.NewSupportInboxSessionRepository(db)
	svc := NewSupportInboxService(
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		sessionRepo,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
	)
	svc.SetGeoIPResolver(widgetTestGeoIPResolver{
		result: &geoip.Result{
			CountryCode: "IN",
			CountryName: "India",
			RegionName:  "Karnataka",
			CityName:    "Bengaluru",
		},
	})

	ipAddress := "203.0.113.9"
	session := &model.SupportWidgetSession{
		WorkspaceID:  workspaceID,
		SessionToken: "geo-backfill-token",
		AnonymousID:  "anon-geo-backfill",
		IsAnonymous:  true,
		IPAddress:    &ipAddress,
		ExpiresAt:    time.Now().Add(24 * time.Hour),
	}
	if err := sessionRepo.Create(ctx, session); err != nil {
		t.Fatalf("create widget session: %v", err)
	}

	updated, err := svc.BackfillWidgetSessionGeo(ctx, 100)
	if err != nil {
		t.Fatalf("BackfillWidgetSessionGeo: %v", err)
	}
	if updated != 1 {
		t.Fatalf("updated = %d, want 1", updated)
	}

	stored, err := sessionRepo.GetByToken(ctx, session.SessionToken)
	if err != nil {
		t.Fatalf("GetByToken: %v", err)
	}
	if stored.CountryCode == nil || *stored.CountryCode != "IN" {
		t.Fatalf("country_code = %v, want %q", stored.CountryCode, "IN")
	}
	if stored.CountryName == nil || *stored.CountryName != "India" {
		t.Fatalf("country_name = %v, want %q", stored.CountryName, "India")
	}
}

func TestSupportInboxServiceGetWidgetSessionRefreshesGeoFromRequestIP(t *testing.T) {
	db := newTestDB(t)

	const workspaceID = "ws-widget-geo-refresh"
	seedWorkspace(t, db, workspaceID, "Widget Geo Refresh WS", "widget-geo-refresh", "user-123")

	ctx := context.Background()
	sessionRepo := repository.NewSupportInboxSessionRepository(db)
	svc := NewSupportInboxService(
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		sessionRepo,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
	)
	svc.SetGeoIPResolver(widgetTestGeoIPResolver{
		result: &geoip.Result{
			CountryCode: "GB",
			CountryName: "United Kingdom",
			RegionName:  "England",
			CityName:    "London",
		},
	})

	session := &model.SupportWidgetSession{
		WorkspaceID:  workspaceID,
		SessionToken: "geo-refresh-token",
		AnonymousID:  "anon-geo-refresh",
		IsAnonymous:  true,
		ExpiresAt:    time.Now().Add(24 * time.Hour),
	}
	if err := sessionRepo.Create(ctx, session); err != nil {
		t.Fatalf("create widget session: %v", err)
	}

	ctx = requestmeta.WithClientIP(ctx, requestmeta.ClientIP{
		Addr:   netip.MustParseAddr("8.8.8.8"),
		Source: "cf-connecting-ip",
	})
	refreshed, err := svc.GetWidgetSession(ctx, session.SessionToken)
	if err != nil {
		t.Fatalf("GetWidgetSession: %v", err)
	}
	if refreshed.IPAddress == nil || *refreshed.IPAddress != "8.8.8.8" {
		t.Fatalf("ip_address = %v, want %q", refreshed.IPAddress, "8.8.8.8")
	}
	if refreshed.CountryCode == nil || *refreshed.CountryCode != "GB" {
		t.Fatalf("country_code = %v, want %q", refreshed.CountryCode, "GB")
	}

	stored, err := sessionRepo.GetByToken(context.Background(), session.SessionToken)
	if err != nil {
		t.Fatalf("GetByToken: %v", err)
	}
	if stored.CountryCode == nil || *stored.CountryCode != "GB" {
		t.Fatalf("stored country_code = %v, want %q", stored.CountryCode, "GB")
	}
}

func TestSupportInboxServiceGetWidgetSessionRejectsExpired(t *testing.T) {
	db := newTestDB(t)

	const workspaceID = "ws-widget-expired"
	seedWorkspace(t, db, workspaceID, "Widget Expired WS", "widget-expired", "user-123")

	ctx := context.Background()
	sessionRepo := repository.NewSupportInboxSessionRepository(db)

	svc := NewSupportInboxService(
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		sessionRepo,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
	)

	expired := &model.SupportWidgetSession{
		WorkspaceID:  workspaceID,
		SessionToken: "expired-session-token",
		AnonymousID:  "anon-expired",
		IsAnonymous:  true,
		ExpiresAt:    time.Now().Add(-1 * time.Hour),
	}
	if err := sessionRepo.Create(ctx, expired); err != nil {
		t.Fatalf("create expired session: %v", err)
	}

	if _, err := svc.GetWidgetSession(ctx, expired.SessionToken); err == nil || !strings.Contains(err.Error(), "session expired") {
		t.Fatalf("GetWidgetSession error = %v, want expired", err)
	}
}

func TestSupportInboxServiceGetWidgetSessionExtendsActiveSessionNearExpiry(t *testing.T) {
	db := newTestDB(t)

	const workspaceID = "ws-widget-extend-expiry"
	seedWorkspace(t, db, workspaceID, "Widget Extend Expiry WS", "widget-extend-expiry", "user-123")

	ctx := context.Background()
	sessionRepo := repository.NewSupportInboxSessionRepository(db)
	svc := NewSupportInboxService(
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		sessionRepo,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
	)

	session := &model.SupportWidgetSession{
		WorkspaceID:  workspaceID,
		SessionToken: "extend-expiry-session-token",
		AnonymousID:  "anon-extend-expiry",
		IsAnonymous:  true,
		ExpiresAt:    time.Now().Add(15 * time.Minute),
	}
	if err := sessionRepo.Create(ctx, session); err != nil {
		t.Fatalf("create widget session: %v", err)
	}

	beforeRefresh := time.Now()
	refreshed, err := svc.GetWidgetSession(ctx, session.SessionToken)
	if err != nil {
		t.Fatalf("GetWidgetSession: %v", err)
	}
	if refreshed.ExpiresAt.Before(beforeRefresh.Add(59 * time.Minute)) {
		t.Fatalf("expires_at = %v, want at least about one hour from activity", refreshed.ExpiresAt)
	}

	stored, err := sessionRepo.GetByToken(ctx, session.SessionToken)
	if err != nil {
		t.Fatalf("GetByToken: %v", err)
	}
	if stored == nil || !stored.ExpiresAt.Equal(refreshed.ExpiresAt) {
		t.Fatalf("stored expires_at = %v, refreshed expires_at = %v", stored, refreshed.ExpiresAt)
	}
}

func TestSupportInboxServiceSessionConversationLifecycle(t *testing.T) {
	db := newTestDB(t)

	workspaceID := "ws-widget-service"
	seedWorkspace(t, db, workspaceID, "Widget Service WS", "widget-service-ws", "user-123")

	ctx := context.Background()
	conversationRepo := repository.NewSupportConversationRepository(db)
	messageRepo := repository.NewSupportMessageRepository(db)
	sessionRepo := repository.NewSupportInboxSessionRepository(db)

	svc := NewSupportInboxService(
		conversationRepo,
		repository.NewSupportMailboxRepository(db),
		messageRepo,
		nil,
		nil,
		nil,
		sessionRepo,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
	)

	session := &model.SupportWidgetSession{
		WorkspaceID:  workspaceID,
		SessionToken: "widget-session-token",
		AnonymousID:  "anon-1",
		IsAnonymous:  true,
		ExpiresAt:    time.Now().Add(24 * time.Hour),
	}
	if err := sessionRepo.Create(ctx, session); err != nil {
		t.Fatalf("create session: %v", err)
	}

	ownedConversation := &model.SupportConversation{
		WorkspaceID: workspaceID,
		Subject:     "Owned conversation",
		Status:      "open",
		AnonymousID: strPtr("anon-1"),
	}
	if err := conversationRepo.Create(ctx, ownedConversation); err != nil {
		t.Fatalf("create owned conversation: %v", err)
	}

	otherConversation := &model.SupportConversation{
		WorkspaceID: workspaceID,
		Subject:     "Other visitor conversation",
		Status:      "open",
		AnonymousID: strPtr("anon-2"),
	}
	if err := conversationRepo.Create(ctx, otherConversation); err != nil {
		t.Fatalf("create other conversation: %v", err)
	}

	if err := svc.SetSessionConversation(ctx, session.SessionToken, ownedConversation.ID); err != nil {
		t.Fatalf("set session conversation: %v", err)
	}

	fetched, err := sessionRepo.GetByToken(ctx, session.SessionToken)
	if err != nil {
		t.Fatalf("get session: %v", err)
	}
	if fetched.ConversationID == nil || *fetched.ConversationID != ownedConversation.ID {
		t.Fatalf("expected conversation_id %q, got %v", ownedConversation.ID, fetched.ConversationID)
	}

	if err := svc.ClearSessionConversation(ctx, session.SessionToken); err != nil {
		t.Fatalf("clear session conversation: %v", err)
	}

	fetched, err = sessionRepo.GetByToken(ctx, session.SessionToken)
	if err != nil {
		t.Fatalf("get session after clear: %v", err)
	}
	if fetched.ConversationID != nil {
		t.Fatalf("expected cleared conversation_id, got %v", *fetched.ConversationID)
	}

	if err := svc.SetSessionConversation(ctx, session.SessionToken, otherConversation.ID); err == nil {
		t.Fatal("expected selecting another visitor's conversation to fail")
	}
}

func TestSupportInboxServiceIdentifyByAnonymousIDRefreshesContactIdentity(t *testing.T) {
	db := newTestDB(t)

	const (
		workspaceID = "ws-widget-identify-refresh"
		widgetKey   = "wk_widget_identify_refresh"
		anonymousID = "anon-refresh"
		email       = "jane@example.com"
	)

	seedWorkspace(t, db, workspaceID, "Widget Identify Refresh", "widget-identify-refresh", "user-123")

	ctx := context.Background()
	installationRepo := repository.NewSupportInboxInstallationRepository(db)
	conversationRepo := repository.NewSupportConversationRepository(db)
	sessionRepo := repository.NewSupportInboxSessionRepository(db)
	contactRepo := repository.NewCRMContactRepository(db)

	if err := installationRepo.Create(ctx, &model.SupportWidgetInstallation{
		WorkspaceID: workspaceID,
		WidgetKey:   widgetKey,
		SecretKey:   "sk_widget_identify_refresh",
		Settings:    "{}",
		Active:      true,
	}); err != nil {
		t.Fatalf("create installation: %v", err)
	}

	contact := &model.CRMContact{
		WorkspaceID:    workspaceID,
		DisplayID:      "CON-1",
		FirstName:      "Jane",
		LastName:       strPtr("Old"),
		Email:          strPtr(email),
		LifecycleStage: model.CRMLifecycleLead,
		LeadStatus:     model.CRMLeadStatusNew,
	}
	if err := contactRepo.Create(ctx, contact); err != nil {
		t.Fatalf("create contact: %v", err)
	}

	conversation := &model.SupportConversation{
		WorkspaceID:   workspaceID,
		Subject:       "Widget identity refresh",
		Status:        "open",
		AnonymousID:   strPtr(anonymousID),
		CustomerEmail: strPtr(email),
		CustomerName:  strPtr("Jane Old"),
	}
	if err := conversationRepo.Create(ctx, conversation); err != nil {
		t.Fatalf("create conversation: %v", err)
	}

	session := &model.SupportWidgetSession{
		WorkspaceID:   workspaceID,
		SessionToken:  "widget-refresh-token",
		AnonymousID:   anonymousID,
		IsAnonymous:   false,
		CustomerEmail: strPtr(email),
		CustomerName:  strPtr("Jane Old"),
		ExpiresAt:     time.Now().Add(24 * time.Hour),
	}
	if err := sessionRepo.Create(ctx, session); err != nil {
		t.Fatalf("create session: %v", err)
	}

	oldTime := time.Now().Add(-2 * time.Hour).UTC()
	for _, stmt := range []struct {
		query string
		args  []any
	}{
		{
			query: "UPDATE crm_contacts SET created_at = ?, updated_at = ? WHERE id = ?",
			args:  []any{oldTime, oldTime, contact.ID},
		},
		{
			query: "UPDATE support_conversations SET created_at = ?, updated_at = ? WHERE id = ?",
			args:  []any{oldTime, oldTime, conversation.ID},
		},
		{
			query: "UPDATE support_widget_sessions SET created_at = ?, updated_at = ? WHERE id = ?",
			args:  []any{oldTime, oldTime, session.ID},
		},
	} {
		if err := db.Exec(stmt.query, stmt.args...).Error; err != nil {
			t.Fatalf("seed old timestamps: %v", err)
		}
	}

	svc := NewSupportInboxService(
		conversationRepo,
		nil,
		nil,
		nil,
		nil,
		installationRepo,
		sessionRepo,
		nil,
		nil,
		nil,
		contactRepo,
		nil,
		nil,
		nil,
		nil,
	)

	if err := svc.IdentifyByAnonymousID(ctx, widgetKey, anonymousID, model.WidgetIdentityPayload{
		Email:  email,
		Name:   "Jane New",
		Source: "sdk_identify",
	}); err != nil {
		t.Fatalf("IdentifyByAnonymousID: %v", err)
	}

	updatedContact, err := contactRepo.GetByID(ctx, contact.ID)
	if err != nil {
		t.Fatalf("get contact: %v", err)
	}
	if updatedContact == nil {
		t.Fatal("expected updated contact")
	}
	if updatedContact.FirstName != "Jane" {
		t.Fatalf("contact first_name = %q, want %q", updatedContact.FirstName, "Jane")
	}
	if updatedContact.LastName == nil || *updatedContact.LastName != "New" {
		t.Fatalf("contact last_name = %v, want %q", updatedContact.LastName, "New")
	}
	if updatedContact.LifecycleStage != model.CRMLifecycleCustomer {
		t.Fatalf("contact lifecycle_stage = %q, want %q", updatedContact.LifecycleStage, model.CRMLifecycleCustomer)
	}
	if !updatedContact.UpdatedAt.After(oldTime) {
		t.Fatalf("contact updated_at = %v, want after %v", updatedContact.UpdatedAt, oldTime)
	}

	updatedConversation, err := conversationRepo.GetByID(ctx, workspaceID, conversation.ID, "", model.RoleOwner)
	if err != nil {
		t.Fatalf("get conversation: %v", err)
	}
	if updatedConversation == nil {
		t.Fatal("expected updated conversation")
	}
	if updatedConversation.CustomerName == nil || *updatedConversation.CustomerName != "Jane New" {
		t.Fatalf("conversation customer_name = %v, want %q", updatedConversation.CustomerName, "Jane New")
	}
	if updatedConversation.CRMContactID == nil || *updatedConversation.CRMContactID != contact.ID {
		t.Fatalf("conversation crm_contact_id = %v, want %q", updatedConversation.CRMContactID, contact.ID)
	}
	if updatedConversation.UpdatedAt.After(oldTime) {
		t.Fatalf("conversation updated_at = %v, want unchanged from %v", updatedConversation.UpdatedAt, oldTime)
	}

	updatedSession, err := sessionRepo.GetByToken(ctx, session.SessionToken)
	if err != nil {
		t.Fatalf("get session: %v", err)
	}
	if updatedSession == nil {
		t.Fatal("expected updated session")
	}
	if updatedSession.CustomerName == nil || *updatedSession.CustomerName != "Jane New" {
		t.Fatalf("session customer_name = %v, want %q", updatedSession.CustomerName, "Jane New")
	}
	if !updatedSession.UpdatedAt.After(oldTime) {
		t.Fatalf("session updated_at = %v, want after %v", updatedSession.UpdatedAt, oldTime)
	}
}

func TestSupportInboxServiceIdentifyByAnonymousIDStoresExplicitFirstAndLastName(t *testing.T) {
	db := newTestDB(t)

	const (
		workspaceID = "ws-widget-identify-explicit-names"
		widgetKey   = "wk_widget_identify_explicit_names"
		anonymousID = "anon-explicit-names"
		email       = "mary@example.com"
	)

	seedWorkspace(t, db, workspaceID, "Widget Explicit Names", "widget-explicit-names", "user-123")

	ctx := context.Background()
	installationRepo := repository.NewSupportInboxInstallationRepository(db)
	conversationRepo := repository.NewSupportConversationRepository(db)
	sessionRepo := repository.NewSupportInboxSessionRepository(db)
	contactRepo := repository.NewCRMContactRepository(db)

	if err := installationRepo.Create(ctx, &model.SupportWidgetInstallation{
		WorkspaceID: workspaceID,
		WidgetKey:   widgetKey,
		SecretKey:   "sk_widget_identify_explicit_names",
		Settings:    "{}",
		Active:      true,
	}); err != nil {
		t.Fatalf("create installation: %v", err)
	}

	conversation := &model.SupportConversation{
		WorkspaceID: workspaceID,
		Subject:     "Widget explicit name parts",
		Status:      "open",
		AnonymousID: strPtr(anonymousID),
	}
	if err := conversationRepo.Create(ctx, conversation); err != nil {
		t.Fatalf("create conversation: %v", err)
	}

	session := &model.SupportWidgetSession{
		WorkspaceID:  workspaceID,
		SessionToken: "widget-explicit-name-token",
		AnonymousID:  anonymousID,
		IsAnonymous:  true,
		ExpiresAt:    time.Now().Add(24 * time.Hour),
	}
	if err := sessionRepo.Create(ctx, session); err != nil {
		t.Fatalf("create session: %v", err)
	}

	if err := svcWithWidgetRepos(installationRepo, conversationRepo, sessionRepo, contactRepo).IdentifyByAnonymousID(ctx, widgetKey, anonymousID, model.WidgetIdentityPayload{
		Email:     email,
		FirstName: "Mary Jane",
		LastName:  "van Dyke",
		Source:    "sdk_identify",
	}); err != nil {
		t.Fatalf("IdentifyByAnonymousID: %v", err)
	}

	contacts, _, err := contactRepo.List(ctx, workspaceID, model.CRMContactListFilters{}, model.PMPagination{Page: 1, PerPage: 10})
	if err != nil {
		t.Fatalf("list contacts: %v", err)
	}
	if len(contacts) != 1 {
		t.Fatalf("contact count = %d, want 1", len(contacts))
	}
	if contacts[0].FirstName != "Mary Jane" {
		t.Fatalf("contact first_name = %q, want %q", contacts[0].FirstName, "Mary Jane")
	}
	if contacts[0].LastName == nil || *contacts[0].LastName != "van Dyke" {
		t.Fatalf("contact last_name = %v, want %q", contacts[0].LastName, "van Dyke")
	}

	updatedConversation, err := conversationRepo.GetByID(ctx, workspaceID, conversation.ID, "", model.RoleOwner)
	if err != nil {
		t.Fatalf("get conversation: %v", err)
	}
	if updatedConversation.CustomerName == nil || *updatedConversation.CustomerName != "Mary Jane van Dyke" {
		t.Fatalf("conversation customer_name = %v, want %q", updatedConversation.CustomerName, "Mary Jane van Dyke")
	}
}

func TestSupportInboxServiceIdentifyByAnonymousIDCreatesCompanyAndPrimaryAssociation(t *testing.T) {
	db := newTestDB(t)

	const (
		workspaceID = "ws-widget-identify-company"
		widgetKey   = "wk_widget_identify_company"
		anonymousID = "anon-company"
		email       = "azhar@example.com"
	)

	seedWorkspace(t, db, workspaceID, "Widget Company", "widget-company", "user-123")

	ctx := context.Background()
	installationRepo := repository.NewSupportInboxInstallationRepository(db)
	conversationRepo := repository.NewSupportConversationRepository(db)
	sessionRepo := repository.NewSupportInboxSessionRepository(db)
	contactRepo := repository.NewCRMContactRepository(db)
	companyRepo := repository.NewCRMCompanyRepository(db)
	assocRepo := repository.NewCRMAssociationRepository(db)

	if err := installationRepo.Create(ctx, &model.SupportWidgetInstallation{
		WorkspaceID: workspaceID,
		WidgetKey:   widgetKey,
		SecretKey:   "sk_widget_identify_company",
		Settings:    "{}",
		Active:      true,
	}); err != nil {
		t.Fatalf("create installation: %v", err)
	}

	conversation := &model.SupportConversation{
		WorkspaceID: workspaceID,
		Subject:     "Widget company identify",
		Status:      "open",
		AnonymousID: strPtr(anonymousID),
	}
	if err := conversationRepo.Create(ctx, conversation); err != nil {
		t.Fatalf("create conversation: %v", err)
	}

	session := &model.SupportWidgetSession{
		WorkspaceID:    workspaceID,
		ConversationID: &conversation.ID,
		SessionToken:   "widget-company-token",
		AnonymousID:    anonymousID,
		IsAnonymous:    true,
		ExpiresAt:      time.Now().Add(24 * time.Hour),
	}
	if err := sessionRepo.Create(ctx, session); err != nil {
		t.Fatalf("create session: %v", err)
	}

	if err := svcWithWidgetRepos(installationRepo, conversationRepo, sessionRepo, contactRepo).IdentifyByAnonymousID(ctx, widgetKey, anonymousID, model.WidgetIdentityPayload{
		Email:     email,
		FirstName: "M",
		LastName:  "Azhar",
		Source:    "sdk_identify",
		Company: model.JSONB{
			"id":         "company-123",
			"name":       "Acme Inc",
			"domain":     "https://www.acme.example/pricing",
			"created_at": "2024-01-15T00:00:00Z",
			"plan":       "enterprise",
			"custom": map[string]interface{}{
				"region": "emea",
			},
		},
	}); err != nil {
		t.Fatalf("IdentifyByAnonymousID: %v", err)
	}

	contacts, _, err := contactRepo.List(ctx, workspaceID, model.CRMContactListFilters{}, model.PMPagination{Page: 1, PerPage: 10})
	if err != nil {
		t.Fatalf("list contacts: %v", err)
	}
	if len(contacts) != 1 {
		t.Fatalf("contact count = %d, want 1", len(contacts))
	}

	companies, _, err := companyRepo.List(ctx, workspaceID, model.CRMCompanyListFilters{}, model.PMPagination{Page: 1, PerPage: 10})
	if err != nil {
		t.Fatalf("list companies: %v", err)
	}
	if len(companies) != 1 {
		t.Fatalf("company count = %d, want 1", len(companies))
	}
	company := companies[0]
	if company.ExternalID == nil || *company.ExternalID != "company-123" {
		t.Fatalf("company external_id = %v, want company-123", company.ExternalID)
	}
	if company.Name != "Acme Inc" {
		t.Fatalf("company name = %q, want Acme Inc", company.Name)
	}
	if company.Domain == nil || *company.Domain != "acme.example" {
		t.Fatalf("company domain = %v, want acme.example", company.Domain)
	}
	if company.CustomProperties["sdk_company_id"] != "company-123" || company.CustomProperties["plan"] != "enterprise" || company.CustomProperties["region"] != "emea" {
		t.Fatalf("company custom_properties = %#v, want sdk company id, plan, and region", company.CustomProperties)
	}

	assocs, err := assocRepo.ListByObject(ctx, workspaceID, model.CRMObjectContact, contacts[0].ID)
	if err != nil {
		t.Fatalf("list associations: %v", err)
	}
	if len(assocs) != 1 {
		t.Fatalf("association count = %d, want 1", len(assocs))
	}
	otherType, otherID := otherAssociationSide(assocs[0], model.CRMObjectContact, contacts[0].ID)
	if otherType != model.CRMObjectCompany || otherID != company.ID {
		t.Fatalf("association other side = %s/%s, want company/%s", otherType, otherID, company.ID)
	}
	if assocs[0].AssociationLabel == nil || *assocs[0].AssociationLabel != primaryCompanyAssociationLabel {
		t.Fatalf("association label = %v, want primary", assocs[0].AssociationLabel)
	}
	linkedConversation, err := conversationRepo.GetByID(ctx, workspaceID, conversation.ID, "", model.RoleOwner)
	if err != nil {
		t.Fatalf("get linked conversation after first identify: %v", err)
	}
	if linkedConversation.CRMCompanyID == nil || *linkedConversation.CRMCompanyID != company.ID {
		t.Fatalf("conversation crm_company_id = %v, want %q", linkedConversation.CRMCompanyID, company.ID)
	}

	if err := svcWithWidgetRepos(installationRepo, conversationRepo, sessionRepo, contactRepo).IdentifyByAnonymousID(ctx, widgetKey, anonymousID, model.WidgetIdentityPayload{
		Email:  email,
		Source: "sdk_identify",
		Company: model.JSONB{
			"id":   "company-456",
			"name": "Beta Inc",
		},
	}); err != nil {
		t.Fatalf("IdentifyByAnonymousID second company: %v", err)
	}

	companies, _, err = companyRepo.List(ctx, workspaceID, model.CRMCompanyListFilters{}, model.PMPagination{Page: 1, PerPage: 10})
	if err != nil {
		t.Fatalf("list companies after switch: %v", err)
	}
	if len(companies) != 2 {
		t.Fatalf("company count after switch = %d, want 2", len(companies))
	}
	assocs, err = assocRepo.ListByObject(ctx, workspaceID, model.CRMObjectContact, contacts[0].ID)
	if err != nil {
		t.Fatalf("list associations after switch: %v", err)
	}
	if len(assocs) != 2 {
		t.Fatalf("association count after switch = %d, want 2", len(assocs))
	}
	primaryCompanyID := ""
	for _, assoc := range assocs {
		_, associatedCompanyID := otherAssociationSide(assoc, model.CRMObjectContact, contacts[0].ID)
		if isPrimaryCompanyAssociationLabel(assoc.AssociationLabel) {
			if primaryCompanyID != "" {
				t.Fatalf("multiple primary company memberships: %q and %q", primaryCompanyID, associatedCompanyID)
			}
			primaryCompanyID = associatedCompanyID
		}
	}
	if primaryCompanyID != company.ID {
		t.Fatalf("primary company after active-company switch = %q, want original %q", primaryCompanyID, company.ID)
	}
	secondCompany, err := companyRepo.GetByExternalID(ctx, workspaceID, "company-456")
	if err != nil || secondCompany == nil {
		t.Fatalf("get second company = %#v, %v", secondCompany, err)
	}
	updatedSession, err := sessionRepo.GetByToken(ctx, session.SessionToken)
	if err != nil {
		t.Fatalf("get session after switch: %v", err)
	}
	if updatedSession.CRMCompanyID == nil || *updatedSession.CRMCompanyID != secondCompany.ID {
		t.Fatalf("session crm_company_id after switch = %v, want %q", updatedSession.CRMCompanyID, secondCompany.ID)
	}
	linkedConversation, err = conversationRepo.GetByID(ctx, workspaceID, conversation.ID, "", model.RoleOwner)
	if err != nil {
		t.Fatalf("get linked conversation after switch: %v", err)
	}
	if linkedConversation.CRMCompanyID == nil || *linkedConversation.CRMCompanyID != company.ID {
		t.Fatalf("conversation crm_company_id after switch = %v, want stable %q", linkedConversation.CRMCompanyID, company.ID)
	}
}

func TestSupportInboxServiceIdentifyCompanySkipsExpiredSessionConversation(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()
	const (
		workspaceID = "ws-widget-expired-company"
		widgetKey   = "wk_widget_expired_company"
		anonymousID = "anon-expired-company"
	)
	seedWorkspace(t, db, workspaceID, "Expired Company", "expired-company", "user-123")
	installationRepo := repository.NewSupportInboxInstallationRepository(db)
	conversationRepo := repository.NewSupportConversationRepository(db)
	sessionRepo := repository.NewSupportInboxSessionRepository(db)
	contactRepo := repository.NewCRMContactRepository(db)
	if err := installationRepo.Create(ctx, &model.SupportWidgetInstallation{WorkspaceID: workspaceID, WidgetKey: widgetKey, SecretKey: "sk-expired-company", Settings: "{}", Active: true}); err != nil {
		t.Fatalf("create installation: %v", err)
	}
	conversation := &model.SupportConversation{WorkspaceID: workspaceID, Subject: "Expired session", Status: "open", AnonymousID: strPtr(anonymousID)}
	if err := conversationRepo.Create(ctx, conversation); err != nil {
		t.Fatalf("create conversation: %v", err)
	}
	session := &model.SupportWidgetSession{WorkspaceID: workspaceID, ConversationID: &conversation.ID, SessionToken: "expired-company-token", AnonymousID: anonymousID, IsAnonymous: true, ExpiresAt: time.Now().Add(-time.Hour)}
	if err := sessionRepo.Create(ctx, session); err != nil {
		t.Fatalf("create expired session: %v", err)
	}

	if err := svcWithWidgetRepos(installationRepo, conversationRepo, sessionRepo, contactRepo).IdentifyByAnonymousID(ctx, widgetKey, anonymousID, model.WidgetIdentityPayload{
		Email:   "expired@example.com",
		Company: model.JSONB{"id": "expired-company", "name": "Expired Co"},
	}); err != nil {
		t.Fatalf("IdentifyByAnonymousID: %v", err)
	}
	updated, err := conversationRepo.GetByID(ctx, workspaceID, conversation.ID, "", model.RoleOwner)
	if err != nil {
		t.Fatalf("get conversation: %v", err)
	}
	if updated.CRMCompanyID != nil {
		t.Fatalf("expired session conversation crm_company_id = %v, want nil", updated.CRMCompanyID)
	}
}

func TestSupportInboxServiceUpgradeWidgetSessionUpdatesOnlyCurrentSessionCompany(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()
	const (
		workspaceID = "ws-widget-upgrade-current-company"
		anonymousID = "anon-upgrade-current-company"
	)
	seedWorkspace(t, db, workspaceID, "Widget Current Company", "widget-current-company", "user-123")
	conversationRepo := repository.NewSupportConversationRepository(db)
	sessionRepo := repository.NewSupportInboxSessionRepository(db)
	contactRepo := repository.NewCRMContactRepository(db)

	currentConversation := &model.SupportConversation{WorkspaceID: workspaceID, Subject: "Current tab", Status: "open", AnonymousID: strPtr(anonymousID)}
	otherConversation := &model.SupportConversation{WorkspaceID: workspaceID, Subject: "Other tab", Status: "open", AnonymousID: strPtr(anonymousID)}
	for _, conversation := range []*model.SupportConversation{currentConversation, otherConversation} {
		if err := conversationRepo.Create(ctx, conversation); err != nil {
			t.Fatalf("create conversation: %v", err)
		}
	}
	currentSession := &model.SupportWidgetSession{
		WorkspaceID: workspaceID, ConversationID: &currentConversation.ID,
		SessionToken: "current-company-token", AnonymousID: anonymousID,
		IsAnonymous: true, ExpiresAt: time.Now().Add(24 * time.Hour),
	}
	otherSession := &model.SupportWidgetSession{
		WorkspaceID: workspaceID, ConversationID: &otherConversation.ID,
		SessionToken: "other-company-token", AnonymousID: anonymousID,
		IsAnonymous: true, ExpiresAt: time.Now().Add(24 * time.Hour),
	}
	for _, session := range []*model.SupportWidgetSession{currentSession, otherSession} {
		if err := sessionRepo.Create(ctx, session); err != nil {
			t.Fatalf("create session: %v", err)
		}
	}

	svc := svcWithWidgetRepos(nil, conversationRepo, sessionRepo, contactRepo)
	if err := svc.UpgradeWidgetSession(ctx, currentSession.SessionToken, model.WidgetIdentityPayload{
		Email:   "current@example.com",
		Company: model.JSONB{"id": "current-account", "name": "Current Account"},
	}); err != nil {
		t.Fatalf("UpgradeWidgetSession: %v", err)
	}

	updatedCurrentSession, err := sessionRepo.GetByToken(ctx, currentSession.SessionToken)
	if err != nil {
		t.Fatalf("get current session: %v", err)
	}
	updatedOtherSession, err := sessionRepo.GetByToken(ctx, otherSession.SessionToken)
	if err != nil {
		t.Fatalf("get other session: %v", err)
	}
	if updatedCurrentSession.CRMCompanyID == nil {
		t.Fatal("current session company was not captured")
	}
	if updatedOtherSession.CRMCompanyID != nil {
		t.Fatalf("other active session company = %v, want nil", updatedOtherSession.CRMCompanyID)
	}

	updatedCurrentConversation, err := conversationRepo.GetByID(ctx, workspaceID, currentConversation.ID, "", model.RoleOwner)
	if err != nil {
		t.Fatalf("get current conversation: %v", err)
	}
	updatedOtherConversation, err := conversationRepo.GetByID(ctx, workspaceID, otherConversation.ID, "", model.RoleOwner)
	if err != nil {
		t.Fatalf("get other conversation: %v", err)
	}
	if updatedCurrentConversation.CRMCompanyID == nil || *updatedCurrentConversation.CRMCompanyID != *updatedCurrentSession.CRMCompanyID {
		t.Fatalf("current conversation company = %v, want current session company %v", updatedCurrentConversation.CRMCompanyID, updatedCurrentSession.CRMCompanyID)
	}
	if updatedOtherConversation.CRMCompanyID != nil {
		t.Fatalf("other active conversation company = %v, want nil", updatedOtherConversation.CRMCompanyID)
	}
}

func TestSupportInboxServiceWidgetCreateConversationCopiesSessionCompany(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()
	const workspaceID = "ws-widget-create-company"
	seedWorkspace(t, db, workspaceID, "Widget Create Company", "widget-create-company", "user-123")
	company := &model.CRMCompany{WorkspaceID: workspaceID, DisplayID: "COM-1", Name: "Acme"}
	if err := repository.NewCRMCompanyRepository(db).Create(ctx, company); err != nil {
		t.Fatalf("create company: %v", err)
	}
	session := &model.SupportWidgetSession{
		WorkspaceID:  workspaceID,
		SessionToken: "widget-create-company-token",
		AnonymousID:  "anon-widget-create-company",
		IsAnonymous:  true,
		CRMCompanyID: &company.ID,
		ExpiresAt:    time.Now().Add(time.Hour),
	}
	sessionRepo := repository.NewSupportInboxSessionRepository(db)
	if err := sessionRepo.Create(ctx, session); err != nil {
		t.Fatalf("create session: %v", err)
	}
	conversationRepo := repository.NewSupportConversationRepository(db)
	svc := NewSupportInboxService(
		conversationRepo,
		repository.NewSupportMailboxRepository(db),
		nil, nil, nil,
		repository.NewSupportInboxInstallationRepository(db),
		sessionRepo,
		nil, nil, nil,
		repository.NewCRMContactRepository(db),
		nil, nil, nil, nil,
	)
	created, err := svc.WidgetCreateConversation(ctx, session.SessionToken)
	if err != nil {
		t.Fatalf("WidgetCreateConversation: %v", err)
	}
	if created.CRMCompanyID == nil || *created.CRMCompanyID != company.ID {
		t.Fatalf("created conversation crm_company_id = %v, want %q", created.CRMCompanyID, company.ID)
	}
}

func TestSupportInboxServiceCompanyExternalIDDoesNotFallBackToDomainOrName(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()
	const workspaceID = "ws-authoritative-company-id"
	seedWorkspace(t, db, workspaceID, "Authoritative Company", "authoritative-company", "user-123")

	companyRepo := repository.NewCRMCompanyRepository(db)
	existingExternalID := "tenant-existing"
	domain := "shared.example"
	existing := &model.CRMCompany{
		WorkspaceID: workspaceID,
		DisplayID:   "COM-1",
		ExternalID:  &existingExternalID,
		Name:        "Shared Company",
		Domain:      &domain,
	}
	if err := companyRepo.Create(ctx, existing); err != nil {
		t.Fatalf("create existing company: %v", err)
	}

	companyID, err := (&SupportInboxService{}).matchOrCreateCRMCompanyIdentityTx(ctx, companyRepo, workspaceID, model.WidgetIdentityPayload{
		Company: model.JSONB{
			"id":     "tenant-new",
			"name":   "Shared Company",
			"domain": domain,
		},
	})
	if err != nil {
		t.Fatalf("matchOrCreateCRMCompanyIdentityTx: %v", err)
	}
	if companyID == nil || *companyID == existing.ID {
		t.Fatalf("resolved company = %v, want a distinct company from %q", companyID, existing.ID)
	}
	companies, _, err := companyRepo.List(ctx, workspaceID, model.CRMCompanyListFilters{}, model.PMPagination{Page: 1, PerPage: 10})
	if err != nil {
		t.Fatalf("list companies: %v", err)
	}
	if len(companies) != 2 {
		t.Fatalf("company count = %d, want 2", len(companies))
	}
}

func TestSyncCRMCompanyIdentityDeletesExplicitNullCustomProperty(t *testing.T) {
	company := &model.CRMCompany{
		Name: "Acme",
		CustomProperties: model.JSONB{
			"plan":         "enterprise",
			"support_tier": "gold",
		},
	}
	identity := resolvedWidgetCompany{
		name: "Acme",
		customProperties: model.JSONB{
			"plan":       nil,
			"seats_used": float64(12),
		},
	}

	if !syncCRMCompanyIdentity(company, identity) {
		t.Fatal("expected identity sync to report a change")
	}
	if _, ok := company.CustomProperties["plan"]; ok {
		t.Fatalf("plan property = %#v, want removed", company.CustomProperties["plan"])
	}
	if company.CustomProperties["support_tier"] != "gold" {
		t.Fatalf("support_tier = %#v, want preserved", company.CustomProperties["support_tier"])
	}
	if company.CustomProperties["seats_used"] != float64(12) {
		t.Fatalf("seats_used = %#v, want numeric 12", company.CustomProperties["seats_used"])
	}
}

func TestSupportInboxServiceIdentifyByAnonymousIDDerivesNameFromEmail(t *testing.T) {
	db := newTestDB(t)

	const (
		workspaceID = "ws-widget-identify-derived-name"
		widgetKey   = "wk_widget_identify_derived_name"
		anonymousID = "anon-derived-name"
		email       = "jane.doe+trial@example.com"
	)

	seedWorkspace(t, db, workspaceID, "Widget Derived Name", "widget-derived-name", "user-123")

	ctx := context.Background()
	installationRepo := repository.NewSupportInboxInstallationRepository(db)
	conversationRepo := repository.NewSupportConversationRepository(db)
	sessionRepo := repository.NewSupportInboxSessionRepository(db)
	contactRepo := repository.NewCRMContactRepository(db)

	if err := installationRepo.Create(ctx, &model.SupportWidgetInstallation{
		WorkspaceID: workspaceID,
		WidgetKey:   widgetKey,
		SecretKey:   "sk_widget_identify_derived_name",
		Settings:    "{}",
		Active:      true,
	}); err != nil {
		t.Fatalf("create installation: %v", err)
	}

	conversation := &model.SupportConversation{
		WorkspaceID: workspaceID,
		Subject:     "Widget derived name",
		Status:      "open",
		AnonymousID: strPtr(anonymousID),
	}
	if err := conversationRepo.Create(ctx, conversation); err != nil {
		t.Fatalf("create conversation: %v", err)
	}

	session := &model.SupportWidgetSession{
		WorkspaceID:  workspaceID,
		SessionToken: "widget-derived-name-token",
		AnonymousID:  anonymousID,
		IsAnonymous:  true,
		ExpiresAt:    time.Now().Add(24 * time.Hour),
	}
	if err := sessionRepo.Create(ctx, session); err != nil {
		t.Fatalf("create session: %v", err)
	}

	if err := svcWithWidgetRepos(installationRepo, conversationRepo, sessionRepo, contactRepo).IdentifyByAnonymousID(ctx, widgetKey, anonymousID, model.WidgetIdentityPayload{
		Email:  email,
		Source: "sdk_lead",
	}); err != nil {
		t.Fatalf("IdentifyByAnonymousID: %v", err)
	}

	contacts, _, err := contactRepo.List(ctx, workspaceID, model.CRMContactListFilters{}, model.PMPagination{Page: 1, PerPage: 10})
	if err != nil {
		t.Fatalf("list contacts: %v", err)
	}
	if len(contacts) != 1 {
		t.Fatalf("contact count = %d, want 1", len(contacts))
	}
	if contacts[0].FirstName != "Jane" {
		t.Fatalf("contact first_name = %q, want %q", contacts[0].FirstName, "Jane")
	}
	if contacts[0].LastName == nil || *contacts[0].LastName != "Doe" {
		t.Fatalf("contact last_name = %v, want %q", contacts[0].LastName, "Doe")
	}

	updatedConversation, err := conversationRepo.GetByID(ctx, workspaceID, conversation.ID, "", model.RoleOwner)
	if err != nil {
		t.Fatalf("get conversation: %v", err)
	}
	if updatedConversation.CustomerName == nil || *updatedConversation.CustomerName != "Jane Doe" {
		t.Fatalf("conversation customer_name = %v, want %q", updatedConversation.CustomerName, "Jane Doe")
	}

	updatedSession, err := sessionRepo.GetByToken(ctx, session.SessionToken)
	if err != nil {
		t.Fatalf("get session: %v", err)
	}
	if updatedSession.CustomerName == nil || *updatedSession.CustomerName != "Jane Doe" {
		t.Fatalf("session customer_name = %v, want %q", updatedSession.CustomerName, "Jane Doe")
	}
}

func svcWithWidgetRepos(
	installationRepo *repository.SupportInboxInstallationRepository,
	conversationRepo *repository.SupportConversationRepository,
	sessionRepo *repository.SupportInboxSessionRepository,
	contactRepo *repository.CRMContactRepository,
) *SupportInboxService {
	return NewSupportInboxService(
		conversationRepo,
		nil,
		nil,
		nil,
		nil,
		installationRepo,
		sessionRepo,
		nil,
		nil,
		nil,
		contactRepo,
		nil,
		nil,
		nil,
		nil,
	)
}

func TestSupportInboxServiceListConversationMessages_AllowsWidgetValidatedContextWithoutActor(t *testing.T) {
	db := newTestDB(t)

	workspaceID := "ws-widget-messages"
	seedWorkspace(t, db, workspaceID, "Widget Messages WS", "widget-messages-ws", "user-123")

	ctx := context.Background()
	conversationRepo := repository.NewSupportConversationRepository(db)
	messageRepo := repository.NewSupportMessageRepository(db)

	svc := NewSupportInboxService(
		conversationRepo,
		repository.NewSupportMailboxRepository(db),
		messageRepo,
		nil,
		nil,
		repository.NewSupportInboxInstallationRepository(db),
		repository.NewSupportInboxSessionRepository(db),
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
	)

	conversation := &model.SupportConversation{
		WorkspaceID: workspaceID,
		Subject:     "Widget conversation",
		Status:      "open",
		AnonymousID: strPtr("anon-1"),
	}
	if err := conversationRepo.Create(ctx, conversation); err != nil {
		t.Fatalf("create conversation: %v", err)
	}

	message := &model.SupportMessage{
		WorkspaceID:    workspaceID,
		ConversationID: conversation.ID,
		SenderType:     "customer",
		MessageType:    "reply",
		Content:        "Need help with refund",
		IsInternal:     false,
	}
	if err := messageRepo.Create(ctx, message); err != nil {
		t.Fatalf("create message: %v", err)
	}

	messages, err := svc.ListConversationMessages(ctx, workspaceID, conversation.ID, false)
	if err != nil {
		t.Fatalf("ListConversationMessages: %v", err)
	}
	if len(messages) != 1 {
		t.Fatalf("message count = %d, want 1", len(messages))
	}
	if messages[0].ID != message.ID {
		t.Fatalf("message id = %q, want %q", messages[0].ID, message.ID)
	}
}

func TestWidgetCreateMessageReopensResolvedConversation(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()

	workspaceID := "ws-widget-reopen"
	seedWorkspace(t, db, workspaceID, "Widget Reopen WS", "widget-reopen-ws", "user-123")

	conversationRepo := repository.NewSupportConversationRepository(db)
	messageRepo := repository.NewSupportMessageRepository(db)
	sessionRepo := repository.NewSupportInboxSessionRepository(db)
	installationRepo := repository.NewSupportInboxInstallationRepository(db)

	resolvedAt := time.Date(2026, 5, 1, 12, 0, 0, 0, time.UTC)
	conversation := &model.SupportConversation{
		WorkspaceID: workspaceID,
		Subject:     "Resolved widget conversation",
		Status:      model.SupportConversationStatusResolved,
		FlowState:   strPtr(model.SupportConversationFlowStateResolvedByHuman),
		ResolvedAt:  &resolvedAt,
		ClosedAt:    &resolvedAt,
		AnonymousID: strPtr("anon-widget-reopen"),
		Source:      "widget",
		Channel:     "widget",
	}
	if err := conversationRepo.Create(ctx, conversation); err != nil {
		t.Fatalf("create conversation: %v", err)
	}

	session := &model.SupportWidgetSession{
		WorkspaceID:    workspaceID,
		SessionToken:   "widget-reopen-session",
		AnonymousID:    "anon-widget-reopen",
		IsAnonymous:    true,
		ConversationID: &conversation.ID,
		ExpiresAt:      time.Now().Add(24 * time.Hour),
	}
	if err := sessionRepo.Create(ctx, session); err != nil {
		t.Fatalf("create session: %v", err)
	}

	svc := NewSupportInboxService(
		conversationRepo,
		repository.NewSupportMailboxRepository(db),
		messageRepo,
		nil,
		nil,
		installationRepo,
		sessionRepo,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
	)

	if _, err := svc.WidgetCreateMessage(ctx, session.SessionToken, "I need more help on this.", nil); err != nil {
		t.Fatalf("WidgetCreateMessage: %v", err)
	}

	updated, err := conversationRepo.GetByID(ctx, workspaceID, conversation.ID, "", model.RoleOwner)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if updated.Status != model.SupportConversationStatusOpen {
		t.Fatalf("status = %q, want %q", updated.Status, model.SupportConversationStatusOpen)
	}
	if updated.ResolvedAt != nil {
		t.Fatalf("resolved_at = %#v, want nil", updated.ResolvedAt)
	}
	if updated.ClosedAt != nil {
		t.Fatalf("closed_at = %#v, want nil", updated.ClosedAt)
	}
	if updated.FlowState == nil || *updated.FlowState != model.SupportConversationFlowStateWaitingForHuman {
		t.Fatalf("flow_state = %#v, want %q", updated.FlowState, model.SupportConversationFlowStateWaitingForHuman)
	}
}
