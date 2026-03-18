package service

import (
	"context"
	"strings"
	"testing"

	"github.com/helpin-ai/helpin/server/internal/auth"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

// newAuthService is a test helper that creates a fresh AuthService with an
// in-memory SQLite database, a UserRepository, and a JWTManager.
func newAuthService(t *testing.T) (*AuthService, *repository.UserRepository) {
	t.Helper()
	db := newTestDB(t)
	userRepo := repository.NewUserRepository(db)
	jwtMgr := auth.NewJWTManager("test-secret")
	svc := NewAuthService(userRepo, nil, jwtMgr)
	return svc, userRepo
}

// ptr returns a pointer to the given string value.
func ptr(s string) *string { return &s }

// ----- Signup Tests --------------------------------------------------------

func TestSignup(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		svc, _ := newAuthService(t)
		ctx := context.Background()

		resp, err := svc.Signup(ctx, model.SignupRequest{
			Email:    "alice@example.com",
			Password: "strongpass1",
			FullName: "Alice Smith",
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if resp.AccessToken == "" {
			t.Error("expected non-empty access token")
		}
		if resp.RefreshToken == "" {
			t.Error("expected non-empty refresh token")
		}
		if resp.User.Email != "alice@example.com" {
			t.Errorf("expected email alice@example.com, got %s", resp.User.Email)
		}
		if resp.User.FullName != "Alice Smith" {
			t.Errorf("expected full name Alice Smith, got %s", resp.User.FullName)
		}
		if resp.User.ID == "" {
			t.Error("expected non-empty user ID")
		}
		if resp.User.CreatedAt.IsZero() {
			t.Error("expected non-zero created_at")
		}
	})

	t.Run("duplicate email", func(t *testing.T) {
		svc, _ := newAuthService(t)
		ctx := context.Background()

		_, err := svc.Signup(ctx, model.SignupRequest{
			Email:    "dup@example.com",
			Password: "strongpass1",
			FullName: "First User",
		})
		if err != nil {
			t.Fatalf("first signup failed: %v", err)
		}

		_, err = svc.Signup(ctx, model.SignupRequest{
			Email:    "dup@example.com",
			Password: "strongpass2",
			FullName: "Second User",
		})
		if err == nil {
			t.Fatal("expected error for duplicate email, got nil")
		}
		if !strings.Contains(err.Error(), "email already in use") {
			t.Errorf("expected 'email already in use' error, got: %v", err)
		}
	})

	t.Run("missing fields", func(t *testing.T) {
		svc, _ := newAuthService(t)
		ctx := context.Background()

		cases := []struct {
			name string
			req  model.SignupRequest
		}{
			{"empty email", model.SignupRequest{Email: "", Password: "strongpass1", FullName: "Name"}},
			{"empty password", model.SignupRequest{Email: "a@b.com", Password: "", FullName: "Name"}},
			{"empty full name", model.SignupRequest{Email: "a@b.com", Password: "strongpass1", FullName: ""}},
			{"all empty", model.SignupRequest{}},
		}

		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				_, err := svc.Signup(ctx, tc.req)
				if err == nil {
					t.Fatal("expected error for missing fields, got nil")
				}
				if !strings.Contains(err.Error(), "required") {
					t.Errorf("expected 'required' in error message, got: %v", err)
				}
			})
		}
	})
}

// ----- Signin Tests --------------------------------------------------------

func TestSignin(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		svc, _ := newAuthService(t)
		ctx := context.Background()

		// First signup to create the user.
		_, err := svc.Signup(ctx, model.SignupRequest{
			Email:    "bob@example.com",
			Password: "correcthorse",
			FullName: "Bob Jones",
		})
		if err != nil {
			t.Fatalf("signup failed: %v", err)
		}

		resp, err := svc.Signin(ctx, model.SigninRequest{
			Email:    "bob@example.com",
			Password: "correcthorse",
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if resp.AccessToken == "" {
			t.Error("expected non-empty access token")
		}
		if resp.RefreshToken == "" {
			t.Error("expected non-empty refresh token")
		}
		if resp.User.Email != "bob@example.com" {
			t.Errorf("expected email bob@example.com, got %s", resp.User.Email)
		}
		if resp.User.FullName != "Bob Jones" {
			t.Errorf("expected full name Bob Jones, got %s", resp.User.FullName)
		}
	})

	t.Run("wrong password", func(t *testing.T) {
		svc, _ := newAuthService(t)
		ctx := context.Background()

		_, err := svc.Signup(ctx, model.SignupRequest{
			Email:    "carol@example.com",
			Password: "realpassword",
			FullName: "Carol White",
		})
		if err != nil {
			t.Fatalf("signup failed: %v", err)
		}

		_, err = svc.Signin(ctx, model.SigninRequest{
			Email:    "carol@example.com",
			Password: "wrongpassword",
		})
		if err == nil {
			t.Fatal("expected error for wrong password, got nil")
		}
		if !strings.Contains(err.Error(), "invalid credentials") {
			t.Errorf("expected 'invalid credentials' error, got: %v", err)
		}
	})

	t.Run("nonexistent email", func(t *testing.T) {
		svc, _ := newAuthService(t)
		ctx := context.Background()

		_, err := svc.Signin(ctx, model.SigninRequest{
			Email:    "nobody@example.com",
			Password: "somepassword",
		})
		if err == nil {
			t.Fatal("expected error for nonexistent email, got nil")
		}
		if !strings.Contains(err.Error(), "invalid credentials") {
			t.Errorf("expected 'invalid credentials' error, got: %v", err)
		}
	})

	t.Run("missing fields", func(t *testing.T) {
		svc, _ := newAuthService(t)
		ctx := context.Background()

		cases := []struct {
			name string
			req  model.SigninRequest
		}{
			{"empty email", model.SigninRequest{Email: "", Password: "password1"}},
			{"empty password", model.SigninRequest{Email: "a@b.com", Password: ""}},
			{"both empty", model.SigninRequest{}},
		}

		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				_, err := svc.Signin(ctx, tc.req)
				if err == nil {
					t.Fatal("expected error for missing fields, got nil")
				}
				if !strings.Contains(err.Error(), "required") {
					t.Errorf("expected 'required' in error message, got: %v", err)
				}
			})
		}
	})
}

// ----- GetProfile Tests ----------------------------------------------------

func TestGetProfile(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		svc, _ := newAuthService(t)
		ctx := context.Background()

		signupResp, err := svc.Signup(ctx, model.SignupRequest{
			Email:    "dave@example.com",
			Password: "password123",
			FullName: "Dave Brown",
		})
		if err != nil {
			t.Fatalf("signup failed: %v", err)
		}

		profile, err := svc.GetProfile(ctx, signupResp.User.ID)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if profile.Email != "dave@example.com" {
			t.Errorf("expected email dave@example.com, got %s", profile.Email)
		}
		if profile.FullName != "Dave Brown" {
			t.Errorf("expected full name Dave Brown, got %s", profile.FullName)
		}
		if profile.ID != signupResp.User.ID {
			t.Errorf("expected ID %s, got %s", signupResp.User.ID, profile.ID)
		}
	})

	t.Run("not found", func(t *testing.T) {
		svc, _ := newAuthService(t)
		ctx := context.Background()

		_, err := svc.GetProfile(ctx, "nonexistent-user-id")
		if err == nil {
			t.Fatal("expected error for unknown user ID, got nil")
		}
		if !strings.Contains(err.Error(), "user not found") {
			t.Errorf("expected 'user not found' error, got: %v", err)
		}
	})
}

// ----- UpdateProfile Tests -------------------------------------------------

func TestUpdateProfile(t *testing.T) {
	t.Run("update full name", func(t *testing.T) {
		svc, _ := newAuthService(t)
		ctx := context.Background()

		signupResp, err := svc.Signup(ctx, model.SignupRequest{
			Email:    "eve@example.com",
			Password: "password123",
			FullName: "Eve Original",
		})
		if err != nil {
			t.Fatalf("signup failed: %v", err)
		}

		updated, err := svc.UpdateProfile(ctx, signupResp.User.ID, model.UpdateProfileRequest{
			FullName: ptr("Eve Updated"),
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if updated.FullName != "Eve Updated" {
			t.Errorf("expected full name 'Eve Updated', got %s", updated.FullName)
		}
		if updated.Email != "eve@example.com" {
			t.Errorf("expected email unchanged, got %s", updated.Email)
		}
	})

	t.Run("update avatar url", func(t *testing.T) {
		svc, _ := newAuthService(t)
		ctx := context.Background()

		signupResp, err := svc.Signup(ctx, model.SignupRequest{
			Email:    "frank@example.com",
			Password: "password123",
			FullName: "Frank Test",
		})
		if err != nil {
			t.Fatalf("signup failed: %v", err)
		}

		updated, err := svc.UpdateProfile(ctx, signupResp.User.ID, model.UpdateProfileRequest{
			AvatarURL: ptr("https://example.com/avatar.png"),
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if updated.AvatarURL == nil || *updated.AvatarURL != "https://example.com/avatar.png" {
			t.Errorf("expected avatar URL 'https://example.com/avatar.png', got %v", updated.AvatarURL)
		}
	})

	t.Run("update default workspace id", func(t *testing.T) {
		svc, _ := newAuthService(t)
		ctx := context.Background()

		signupResp, err := svc.Signup(ctx, model.SignupRequest{
			Email:    "grace@example.com",
			Password: "password123",
			FullName: "Grace Test",
		})
		if err != nil {
			t.Fatalf("signup failed: %v", err)
		}

		wsID := "workspace-id-123"
		updated, err := svc.UpdateProfile(ctx, signupResp.User.ID, model.UpdateProfileRequest{
			DefaultWorkspaceID: ptr(wsID),
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if updated.DefaultWorkspaceID == nil || *updated.DefaultWorkspaceID != wsID {
			t.Errorf("expected default workspace ID %s, got %v", wsID, updated.DefaultWorkspaceID)
		}
	})

	t.Run("verified via get profile", func(t *testing.T) {
		svc, _ := newAuthService(t)
		ctx := context.Background()

		signupResp, err := svc.Signup(ctx, model.SignupRequest{
			Email:    "harry@example.com",
			Password: "password123",
			FullName: "Harry Before",
		})
		if err != nil {
			t.Fatalf("signup failed: %v", err)
		}

		_, err = svc.UpdateProfile(ctx, signupResp.User.ID, model.UpdateProfileRequest{
			FullName: ptr("Harry After"),
		})
		if err != nil {
			t.Fatalf("update failed: %v", err)
		}

		// Verify through GetProfile that the change persisted.
		profile, err := svc.GetProfile(ctx, signupResp.User.ID)
		if err != nil {
			t.Fatalf("get profile failed: %v", err)
		}
		if profile.FullName != "Harry After" {
			t.Errorf("expected full name 'Harry After' after update, got %s", profile.FullName)
		}
	})
}

// ----- RefreshToken Tests --------------------------------------------------

func TestRefreshToken(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		svc, _ := newAuthService(t)
		ctx := context.Background()

		signupResp, err := svc.Signup(ctx, model.SignupRequest{
			Email:    "ivan@example.com",
			Password: "password123",
			FullName: "Ivan Test",
		})
		if err != nil {
			t.Fatalf("signup failed: %v", err)
		}

		refreshResp, err := svc.RefreshToken(ctx, signupResp.RefreshToken)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if refreshResp.AccessToken == "" {
			t.Error("expected non-empty access token")
		}
		if refreshResp.RefreshToken == "" {
			t.Error("expected non-empty refresh token")
		}
		if refreshResp.User.Email != "ivan@example.com" {
			t.Errorf("expected email ivan@example.com, got %s", refreshResp.User.Email)
		}
		if refreshResp.User.FullName != "Ivan Test" {
			t.Errorf("expected full name Ivan Test, got %s", refreshResp.User.FullName)
		}
	})

	t.Run("invalid token", func(t *testing.T) {
		svc, _ := newAuthService(t)
		ctx := context.Background()

		_, err := svc.RefreshToken(ctx, "garbage-token-value")
		if err == nil {
			t.Fatal("expected error for invalid refresh token, got nil")
		}
		if !strings.Contains(err.Error(), "invalid") {
			t.Errorf("expected 'invalid' in error message, got: %v", err)
		}
	})

	t.Run("token from different secret", func(t *testing.T) {
		svc, _ := newAuthService(t)
		ctx := context.Background()

		// Generate a token with a different secret.
		otherJWT := auth.NewJWTManager("different-secret")
		_, otherRefresh, err := otherJWT.GenerateTokenPair("some-id", "some@email.com")
		if err != nil {
			t.Fatalf("generate token pair failed: %v", err)
		}

		_, err = svc.RefreshToken(ctx, otherRefresh)
		if err == nil {
			t.Fatal("expected error for token from different secret, got nil")
		}
		if !strings.Contains(err.Error(), "invalid") {
			t.Errorf("expected 'invalid' in error message, got: %v", err)
		}
	})
}

// ----- ChangePassword Tests ------------------------------------------------

func TestChangePassword(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		svc, _ := newAuthService(t)
		ctx := context.Background()

		signupResp, err := svc.Signup(ctx, model.SignupRequest{
			Email:    "jane@example.com",
			Password: "oldpassword1",
			FullName: "Jane Test",
		})
		if err != nil {
			t.Fatalf("signup failed: %v", err)
		}

		err = svc.ChangePassword(ctx, signupResp.User.ID, model.ChangePasswordRequest{
			CurrentPassword: "oldpassword1",
			NewPassword:     "newpassword1",
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// Verify: signin with old password should fail.
		_, err = svc.Signin(ctx, model.SigninRequest{
			Email:    "jane@example.com",
			Password: "oldpassword1",
		})
		if err == nil {
			t.Error("expected signin with old password to fail after change")
		}

		// Verify: signin with new password should succeed.
		resp, err := svc.Signin(ctx, model.SigninRequest{
			Email:    "jane@example.com",
			Password: "newpassword1",
		})
		if err != nil {
			t.Fatalf("signin with new password failed: %v", err)
		}
		if resp.User.Email != "jane@example.com" {
			t.Errorf("expected email jane@example.com, got %s", resp.User.Email)
		}
	})

	t.Run("wrong current password", func(t *testing.T) {
		svc, _ := newAuthService(t)
		ctx := context.Background()

		signupResp, err := svc.Signup(ctx, model.SignupRequest{
			Email:    "kyle@example.com",
			Password: "realpassword",
			FullName: "Kyle Test",
		})
		if err != nil {
			t.Fatalf("signup failed: %v", err)
		}

		err = svc.ChangePassword(ctx, signupResp.User.ID, model.ChangePasswordRequest{
			CurrentPassword: "wrongcurrent",
			NewPassword:     "newpassword1",
		})
		if err == nil {
			t.Fatal("expected error for wrong current password, got nil")
		}
		if !strings.Contains(err.Error(), "current password is incorrect") {
			t.Errorf("expected 'current password is incorrect' error, got: %v", err)
		}
	})

	t.Run("new password too short", func(t *testing.T) {
		svc, _ := newAuthService(t)
		ctx := context.Background()

		signupResp, err := svc.Signup(ctx, model.SignupRequest{
			Email:    "laura@example.com",
			Password: "password123",
			FullName: "Laura Test",
		})
		if err != nil {
			t.Fatalf("signup failed: %v", err)
		}

		err = svc.ChangePassword(ctx, signupResp.User.ID, model.ChangePasswordRequest{
			CurrentPassword: "password123",
			NewPassword:     "short",
		})
		if err == nil {
			t.Fatal("expected error for short new password, got nil")
		}
		if !strings.Contains(err.Error(), "at least 8 characters") {
			t.Errorf("expected 'at least 8 characters' error, got: %v", err)
		}
	})

	t.Run("missing fields", func(t *testing.T) {
		svc, _ := newAuthService(t)
		ctx := context.Background()

		signupResp, err := svc.Signup(ctx, model.SignupRequest{
			Email:    "mike@example.com",
			Password: "password123",
			FullName: "Mike Test",
		})
		if err != nil {
			t.Fatalf("signup failed: %v", err)
		}

		cases := []struct {
			name string
			req  model.ChangePasswordRequest
		}{
			{"empty current", model.ChangePasswordRequest{CurrentPassword: "", NewPassword: "newpassword1"}},
			{"empty new", model.ChangePasswordRequest{CurrentPassword: "password123", NewPassword: ""}},
			{"both empty", model.ChangePasswordRequest{}},
		}

		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				err := svc.ChangePassword(ctx, signupResp.User.ID, tc.req)
				if err == nil {
					t.Fatal("expected error for missing fields, got nil")
				}
				if !strings.Contains(err.Error(), "required") {
					t.Errorf("expected 'required' in error message, got: %v", err)
				}
			})
		}
	})

	t.Run("exactly 8 characters succeeds", func(t *testing.T) {
		svc, _ := newAuthService(t)
		ctx := context.Background()

		signupResp, err := svc.Signup(ctx, model.SignupRequest{
			Email:    "nina@example.com",
			Password: "password123",
			FullName: "Nina Test",
		})
		if err != nil {
			t.Fatalf("signup failed: %v", err)
		}

		err = svc.ChangePassword(ctx, signupResp.User.ID, model.ChangePasswordRequest{
			CurrentPassword: "password123",
			NewPassword:     "exactly8",
		})
		if err != nil {
			t.Fatalf("expected 8-char password to succeed, got: %v", err)
		}
	})

	t.Run("7 characters fails", func(t *testing.T) {
		svc, _ := newAuthService(t)
		ctx := context.Background()

		signupResp, err := svc.Signup(ctx, model.SignupRequest{
			Email:    "otto@example.com",
			Password: "password123",
			FullName: "Otto Test",
		})
		if err != nil {
			t.Fatalf("signup failed: %v", err)
		}

		err = svc.ChangePassword(ctx, signupResp.User.ID, model.ChangePasswordRequest{
			CurrentPassword: "password123",
			NewPassword:     "seven77",
		})
		if err == nil {
			t.Fatal("expected error for 7-char password, got nil")
		}
		if !strings.Contains(err.Error(), "at least 8 characters") {
			t.Errorf("expected 'at least 8 characters' error, got: %v", err)
		}
	})
}
