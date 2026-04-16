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
	if remaining := time.Until(identified.ExpiresAt); remaining < (29*24*time.Hour) || remaining > (31*24*time.Hour) {
		t.Fatalf("expires_at remaining = %v, want about 30 days", remaining)
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
