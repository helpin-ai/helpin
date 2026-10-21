package handler

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/helpin-ai/helpin/server/internal/authorization"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

// Only support admins may change the email of, or delete, a contact with a
// portal decision; CRM editors (members) may not.
func TestCRMContactPortalAuthorityRequiresSupportAdmin(t *testing.T) {
	h := NewCRMContactHandler(nil).SetAuthorization(authorization.NewAuthzService(nil, nil, nil))
	for _, tc := range []struct {
		role string
		want bool
	}{
		{"viewer", false},
		{"member", false},
		{"admin", true},
		{"owner", true},
	} {
		t.Run(tc.role, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodPut, "/api/crm/contacts/c1", nil)
			r = r.WithContext(authorization.WithActor(r.Context(), &authorization.Actor{UserID: "u1", Role: tc.role}))
			if got := repository.HasPortalAccessAuthority(h.withPortalAccessAuthority(r)); got != tc.want {
				t.Fatalf("role %s portal authority = %v, want %v", tc.role, got, tc.want)
			}
		})
	}
	anonymous := httptest.NewRequest(http.MethodPut, "/api/crm/contacts/c1", nil)
	if repository.HasPortalAccessAuthority(h.withPortalAccessAuthority(anonymous)) {
		t.Fatal("request without an actor has portal authority")
	}
}
