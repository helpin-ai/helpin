package service

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/helpin-ai/helpin/server/internal/auth"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

// AuthService handles authentication business logic.
type AuthService struct {
	userRepo   *repository.UserRepository
	jwtManager *auth.JWTManager
	logger     *slog.Logger
}

// NewAuthService creates a new AuthService.
func NewAuthService(userRepo *repository.UserRepository, jwtManager *auth.JWTManager) *AuthService {
	return &AuthService{
		userRepo:   userRepo,
		jwtManager: jwtManager,
		logger:     slog.Default().With("service", "auth"),
	}
}

// Signup creates a new user account and returns auth tokens.
func (s *AuthService) Signup(ctx context.Context, req model.SignupRequest) (*model.AuthResponse, error) {
	if req.Email == "" || req.Password == "" || req.FullName == "" {
		return nil, fmt.Errorf("email, password, and full_name are required")
	}

	existing, err := s.userRepo.GetByEmail(ctx, req.Email)
	if err != nil {
		s.logger.ErrorContext(ctx, "failed to check existing user", "email", req.Email, "error", err)
		return nil, fmt.Errorf("check existing user: %w", err)
	}
	if existing != nil {
		s.logger.InfoContext(ctx, "signup attempted with existing email", "email", req.Email)
		return nil, fmt.Errorf("email already in use")
	}

	hash, err := auth.HashPassword(req.Password)
	if err != nil {
		s.logger.ErrorContext(ctx, "failed to hash password during signup", "email", req.Email, "error", err)
		return nil, fmt.Errorf("hash password: %w", err)
	}

	user, err := s.userRepo.Create(ctx, req.Email, hash, req.FullName)
	if err != nil {
		s.logger.ErrorContext(ctx, "failed to create user", "email", req.Email, "error", err)
		return nil, fmt.Errorf("create user: %w", err)
	}

	accessToken, refreshToken, err := s.jwtManager.GenerateTokenPair(user.ID, user.Email)
	if err != nil {
		s.logger.ErrorContext(ctx, "failed to generate tokens after signup", "user_id", user.ID, "error", err)
		return nil, fmt.Errorf("generate tokens: %w", err)
	}

	s.logger.InfoContext(ctx, "user signed up", "user_id", user.ID, "email", user.Email)

	return &model.AuthResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		User:         toUserProfile(user),
	}, nil
}

// Signin authenticates a user and returns auth tokens.
func (s *AuthService) Signin(ctx context.Context, req model.SigninRequest) (*model.AuthResponse, error) {
	if req.Email == "" || req.Password == "" {
		return nil, fmt.Errorf("email and password are required")
	}

	user, err := s.userRepo.GetByEmail(ctx, req.Email)
	if err != nil {
		s.logger.ErrorContext(ctx, "failed to find user during signin", "email", req.Email, "error", err)
		return nil, fmt.Errorf("find user: %w", err)
	}
	if user == nil {
		s.logger.InfoContext(ctx, "signin attempted with unknown email", "email", req.Email)
		return nil, fmt.Errorf("invalid credentials")
	}

	if err := auth.CheckPassword(req.Password, user.PasswordHash); err != nil {
		s.logger.InfoContext(ctx, "signin failed invalid password", "user_id", user.ID, "email", req.Email)
		return nil, fmt.Errorf("invalid credentials")
	}

	accessToken, refreshToken, err := s.jwtManager.GenerateTokenPair(user.ID, user.Email)
	if err != nil {
		s.logger.ErrorContext(ctx, "failed to generate tokens after signin", "user_id", user.ID, "error", err)
		return nil, fmt.Errorf("generate tokens: %w", err)
	}

	s.logger.InfoContext(ctx, "user signed in", "user_id", user.ID, "email", user.Email)

	return &model.AuthResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		User:         toUserProfile(user),
	}, nil
}

// GetProfile returns the profile for the given user ID.
func (s *AuthService) GetProfile(ctx context.Context, userID string) (*model.UserProfile, error) {
	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("get user: %w", err)
	}
	if user == nil {
		return nil, fmt.Errorf("user not found")
	}
	profile := toUserProfile(user)
	return &profile, nil
}

// UpdateProfile updates the authenticated user's profile.
func (s *AuthService) UpdateProfile(ctx context.Context, userID string, req model.UpdateProfileRequest) (*model.UserProfile, error) {
	user, err := s.userRepo.Update(ctx, userID, req.FullName, req.AvatarURL, req.DefaultWorkspaceID)
	if err != nil {
		return nil, fmt.Errorf("update profile: %w", err)
	}
	profile := toUserProfile(user)
	return &profile, nil
}

// RefreshToken validates a refresh token and issues a new token pair.
func (s *AuthService) RefreshToken(ctx context.Context, refreshToken string) (*model.AuthResponse, error) {
	claims, err := s.jwtManager.ValidateToken(refreshToken)
	if err != nil {
		s.logger.ErrorContext(ctx, "invalid refresh token", "error", err)
		return nil, fmt.Errorf("invalid refresh token: %w", err)
	}

	user, err := s.userRepo.GetByID(ctx, claims.UserID)
	if err != nil {
		s.logger.ErrorContext(ctx, "failed to get user during token refresh", "user_id", claims.UserID, "error", err)
		return nil, fmt.Errorf("get user: %w", err)
	}
	if user == nil {
		s.logger.ErrorContext(ctx, "user not found during token refresh", "user_id", claims.UserID)
		return nil, fmt.Errorf("user not found")
	}

	accessToken, newRefresh, err := s.jwtManager.GenerateTokenPair(user.ID, user.Email)
	if err != nil {
		s.logger.ErrorContext(ctx, "failed to generate tokens during refresh", "user_id", user.ID, "error", err)
		return nil, fmt.Errorf("generate tokens: %w", err)
	}

	s.logger.InfoContext(ctx, "token refreshed", "user_id", user.ID)

	return &model.AuthResponse{
		AccessToken:  accessToken,
		RefreshToken: newRefresh,
		User:         toUserProfile(user),
	}, nil
}

// ChangePassword verifies the current password and sets a new one.
func (s *AuthService) ChangePassword(ctx context.Context, userID string, req model.ChangePasswordRequest) error {
	if req.CurrentPassword == "" || req.NewPassword == "" {
		return fmt.Errorf("current_password and new_password are required")
	}
	if len(req.NewPassword) < 8 {
		return fmt.Errorf("new password must be at least 8 characters")
	}

	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		s.logger.ErrorContext(ctx, "failed to get user during password change", "user_id", userID, "error", err)
		return fmt.Errorf("get user: %w", err)
	}
	if user == nil {
		s.logger.ErrorContext(ctx, "user not found during password change", "user_id", userID)
		return fmt.Errorf("user not found")
	}

	if err := auth.CheckPassword(req.CurrentPassword, user.PasswordHash); err != nil {
		s.logger.InfoContext(ctx, "password change failed invalid current password", "user_id", userID)
		return fmt.Errorf("current password is incorrect")
	}

	hash, err := auth.HashPassword(req.NewPassword)
	if err != nil {
		s.logger.ErrorContext(ctx, "failed to hash new password", "user_id", userID, "error", err)
		return fmt.Errorf("hash password: %w", err)
	}

	if err := s.userRepo.UpdatePassword(ctx, userID, hash); err != nil {
		s.logger.ErrorContext(ctx, "failed to update password", "user_id", userID, "error", err)
		return err
	}

	s.logger.InfoContext(ctx, "password changed", "user_id", userID)
	return nil
}

func toUserProfile(u *model.User) model.UserProfile {
	return model.UserProfile{
		ID:                 u.ID,
		Email:              u.Email,
		FullName:           u.FullName,
		AvatarURL:          u.AvatarURL,
		DefaultWorkspaceID: u.DefaultWorkspaceID,
		CreatedAt:          u.CreatedAt,
	}
}
