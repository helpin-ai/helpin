package model

import (
	"sync"
	"testing"

	"gorm.io/gorm/schema"
)

func TestExternalMCPServerOAuthScopesColumnName(t *testing.T) {
	parsed, err := schema.Parse(&ExternalMCPServer{}, &sync.Map{}, schema.NamingStrategy{})
	if err != nil {
		t.Fatalf("parse ExternalMCPServer schema: %v", err)
	}

	field := parsed.LookUpField("OAuthScopes")
	if field == nil {
		t.Fatal("OAuthScopes field is missing from the GORM schema")
	}
	if field.DBName != "oauth_scopes" {
		t.Fatalf("OAuthScopes DB column = %q, want %q", field.DBName, "oauth_scopes")
	}
}
