package service

import (
	"testing"

	emailtpl "github.com/helpin-ai/helpin/server/internal/email"
)

func TestNewNotificationServiceNormalizesTypedNilEmailClient(t *testing.T) {
	var client *emailtpl.Client
	service := NewNotificationService(nil, nil, nil, nil, nil, nil, nil, client, "")

	if service.emailClient != nil {
		t.Fatal("expected typed nil email client to be normalized to nil")
	}
}
