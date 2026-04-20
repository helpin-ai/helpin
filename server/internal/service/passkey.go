package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/go-webauthn/webauthn/protocol"
	gwebauthn "github.com/go-webauthn/webauthn/webauthn"

	"github.com/helpin-ai/helpin/server/internal/auth"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
	appwebauthn "github.com/helpin-ai/helpin/server/internal/webauthn"
	"gorm.io/gorm"
)

var (
	ErrPasskeyNotConfigured = errors.New("passkeys are not configured")
	ErrPasskeyNotFound      = errors.New("passkey not found")
	ErrNoPasskeyForAccount  = errors.New("no passkey found for this account")
)

type passkeyWebAuthnClient interface {
	Configured() bool
	BeginRegistration(ctx context.Context, user gwebauthn.User, exclusions []protocol.CredentialDescriptor) (*protocol.CredentialCreation, string, error)
	BeginAuthentication(ctx context.Context) (*protocol.CredentialAssertion, string, error)
	BeginAuthenticationForUser(ctx context.Context, user gwebauthn.User) (*protocol.CredentialAssertion, string, error)
	FinishRegistration(ctx context.Context, user gwebauthn.User, challenge string, payload []byte) (*gwebauthn.Credential, error)
	FinishAuthentication(ctx context.Context, challenge string, payload []byte, handler gwebauthn.DiscoverableUserHandler) (gwebauthn.User, *gwebauthn.Credential, error)
	FinishAuthenticationForUser(ctx context.Context, user gwebauthn.User, challenge string, payload []byte) (*gwebauthn.Credential, error)
	SessionInfo(ctx context.Context, challenge string) (*appwebauthn.SessionInfo, error)
}

// PasskeyService handles passkey ceremonies and passkey CRUD operations.
type PasskeyService struct {
	userRepo       *repository.UserRepository
	passkeyRepo    *repository.PasskeyRepository
	jwtManager     *auth.JWTManager
	webauthnClient passkeyWebAuthnClient
	encryptionKey  []byte
	logger         *slog.Logger
}

func NewPasskeyService(
	userRepo *repository.UserRepository,
	passkeyRepo *repository.PasskeyRepository,
	jwtManager *auth.JWTManager,
	webauthnClient passkeyWebAuthnClient,
	encryptionKey []byte,
) *PasskeyService {
	return &PasskeyService{
		userRepo:       userRepo,
		passkeyRepo:    passkeyRepo,
		jwtManager:     jwtManager,
		webauthnClient: webauthnClient,
		encryptionKey:  append([]byte(nil), encryptionKey...),
		logger:         slog.Default().With("service", "passkey"),
	}
}

func (s *PasskeyService) BeginRegistration(ctx context.Context, userID string) (*model.PasskeyOptionsResponse, error) {
	if !s.isConfigured() {
		return nil, ErrPasskeyNotConfigured
	}

	user, passkeyUser, exclusions, err := s.loadPasskeyUser(ctx, userID)
	if err != nil {
		return nil, err
	}

	options, challenge, err := s.webauthnClient.BeginRegistration(ctx, passkeyUser, exclusions)
	if err != nil {
		s.logger.ErrorContext(ctx, "failed to begin passkey registration", "user_id", userID, "error", err)
		return nil, fmt.Errorf("begin passkey registration: %w", err)
	}

	encoded, err := json.Marshal(options)
	if err != nil {
		return nil, fmt.Errorf("marshal passkey registration options: %w", err)
	}

	s.logger.InfoContext(ctx, "prepared passkey registration", "user_id", user.ID)

	return &model.PasskeyOptionsResponse{
		Challenge: challenge,
		Options:   encoded,
	}, nil
}

func (s *PasskeyService) FinishRegistration(ctx context.Context, userID string, req model.PasskeyRegisterRequest, userAgent string) (*model.PasskeyResponse, error) {
	if !s.isConfigured() {
		return nil, ErrPasskeyNotConfigured
	}
	if strings.TrimSpace(req.Challenge) == "" || len(req.Credential) == 0 {
		return nil, fmt.Errorf("challenge and credential are required")
	}

	user, passkeyUser, _, err := s.loadPasskeyUser(ctx, userID)
	if err != nil {
		return nil, err
	}

	credential, err := s.webauthnClient.FinishRegistration(ctx, passkeyUser, req.Challenge, req.Credential)
	if err != nil {
		s.logger.InfoContext(ctx, "passkey registration failed", "user_id", userID, "error", err)
		return nil, fmt.Errorf("verify passkey registration: %w", err)
	}

	transports, err := json.Marshal(credential.Transport)
	if err != nil {
		return nil, fmt.Errorf("marshal passkey transports: %w", err)
	}

	passkey := &model.UserPasskey{
		UserID:          user.ID,
		CredentialID:    append([]byte(nil), credential.ID...),
		PublicKey:       append([]byte(nil), credential.PublicKey...),
		AttestationType: credential.AttestationType,
		Transport:       append(model.JSONBlob(nil), transports...),
		SignCount:       int64(credential.Authenticator.SignCount),
		Name:            normalizePasskeyName(req.Name, userAgent),
		AAGUID:          append([]byte(nil), credential.Authenticator.AAGUID...),
		Flags:           int(credential.Flags.ProtocolValue()),
		Verified:        credential.Flags.UserVerified,
	}

	created, err := s.passkeyRepo.Create(ctx, passkey)
	if err != nil {
		s.logger.ErrorContext(ctx, "failed to persist passkey", "user_id", user.ID, "error", err)
		return nil, fmt.Errorf("store passkey: %w", err)
	}

	s.logger.InfoContext(ctx, "registered passkey", "user_id", user.ID, "passkey_id", created.ID)

	public := created.Public()
	return &public, nil
}

func (s *PasskeyService) BeginAuthentication(ctx context.Context, req model.PasskeyAuthenticationOptionsRequest) (*model.PasskeyOptionsResponse, error) {
	if !s.isConfigured() {
		return nil, ErrPasskeyNotConfigured
	}

	var (
		options   *protocol.CredentialAssertion
		challenge string
		err       error
	)

	if req.EmailHint != nil && strings.TrimSpace(*req.EmailHint) != "" {
		user, passkeyUser, _, loadErr := s.loadPasskeyUserByEmail(ctx, *req.EmailHint)
		if loadErr != nil {
			return nil, loadErr
		}

		options, challenge, err = s.webauthnClient.BeginAuthenticationForUser(ctx, passkeyUser)
		if err != nil {
			s.logger.InfoContext(ctx, "failed to begin hinted passkey authentication", "user_id", user.ID, "error", err)
			return nil, fmt.Errorf("begin passkey authentication: %w", err)
		}
	} else {
		options, challenge, err = s.webauthnClient.BeginAuthentication(ctx)
		if err != nil {
			s.logger.InfoContext(ctx, "failed to begin discoverable passkey authentication", "error", err)
			return nil, fmt.Errorf("begin passkey authentication: %w", err)
		}
	}

	encoded, err := json.Marshal(options)
	if err != nil {
		return nil, fmt.Errorf("marshal passkey authentication options: %w", err)
	}

	return &model.PasskeyOptionsResponse{
		Challenge: challenge,
		Options:   encoded,
	}, nil
}

func (s *PasskeyService) FinishAuthentication(ctx context.Context, req model.PasskeyAuthenticateRequest) (*model.SigninResponse, error) {
	if !s.isConfigured() {
		return nil, ErrPasskeyNotConfigured
	}
	if strings.TrimSpace(req.Challenge) == "" || len(req.Credential) == 0 {
		return nil, fmt.Errorf("challenge and credential are required")
	}

	sessionInfo, err := s.webauthnClient.SessionInfo(ctx, req.Challenge)
	if err != nil {
		s.logger.InfoContext(ctx, "passkey authentication session unavailable", "error", err)
		return nil, fmt.Errorf("load passkey session: %w", err)
	}

	var (
		user       *model.User
		credential *gwebauthn.Credential
	)

	switch sessionInfo.Kind {
	case appwebauthn.SessionKindKnownUserLogin:
		user, credential, err = s.finishKnownUserAuthentication(ctx, string(sessionInfo.UserID), req)
	case appwebauthn.SessionKindAuthentication:
		user, credential, err = s.finishDiscoverableAuthentication(ctx, req)
	default:
		err = appwebauthn.ErrUnexpectedSession
	}
	if err != nil {
		s.logger.InfoContext(ctx, "passkey authentication failed", "error", err)
		return nil, fmt.Errorf("verify passkey authentication: %w", err)
	}

	storedPasskey, err := s.passkeyRepo.GetByCredentialID(ctx, credential.ID)
	if err != nil {
		return nil, fmt.Errorf("reload passkey: %w", err)
	}
	if storedPasskey == nil {
		return nil, ErrNoPasskeyForAccount
	}

	if _, err := s.passkeyRepo.UpdateAuthenticationState(
		ctx,
		storedPasskey.ID,
		int64(credential.Authenticator.SignCount),
		int(credential.Flags.ProtocolValue()),
		credential.Flags.UserVerified,
	); err != nil {
		return nil, fmt.Errorf("update passkey usage: %w", err)
	}

	if user.TOTPVerified && !credential.Flags.UserVerified {
		if len(s.encryptionKey) != 32 || user.TOTPSecretEncrypted == nil || strings.TrimSpace(*user.TOTPSecretEncrypted) == "" {
			return nil, fmt.Errorf("two-factor authentication is not available")
		}

		twoFAToken, err := s.jwtManager.Generate2FAToken(user.ID, user.Email, req.RememberMe)
		if err != nil {
			return nil, fmt.Errorf("generate 2fa token: %w", err)
		}

		return &model.SigninResponse{
			Requires2FA: true,
			TwoFAToken:  twoFAToken,
		}, nil
	}

	accessToken, refreshToken, err := s.jwtManager.GenerateTokenPair(user.ID, user.Email, req.RememberMe)
	if err != nil {
		return nil, fmt.Errorf("generate auth tokens: %w", err)
	}

	profile := toUserProfile(user)
	s.logger.InfoContext(ctx, "passkey login completed", "user_id", user.ID, "passkey_id", storedPasskey.ID, "user_verified", credential.Flags.UserVerified)

	return &model.SigninResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		User:         &profile,
	}, nil
}

func (s *PasskeyService) ListPasskeys(ctx context.Context, userID string) (*model.PasskeyListResponse, error) {
	passkeys, err := s.passkeyRepo.ListByUser(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("list passkeys: %w", err)
	}

	items := make([]model.PasskeyResponse, 0, len(passkeys))
	for _, passkey := range passkeys {
		items = append(items, passkey.Public())
	}

	return &model.PasskeyListResponse{Passkeys: items}, nil
}

func (s *PasskeyService) DeletePasskey(ctx context.Context, userID, passkeyID string) error {
	if err := s.passkeyRepo.Delete(ctx, userID, passkeyID); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) || strings.Contains(err.Error(), "record not found") {
			return ErrPasskeyNotFound
		}
		if errors.Is(err, context.Canceled) {
			return err
		}
		return fmt.Errorf("delete passkey: %w", err)
	}

	s.logger.InfoContext(ctx, "deleted passkey", "user_id", userID, "passkey_id", passkeyID)
	return nil
}

func (s *PasskeyService) finishDiscoverableAuthentication(ctx context.Context, req model.PasskeyAuthenticateRequest) (*model.User, *gwebauthn.Credential, error) {
	passkeyUser, credential, err := s.webauthnClient.FinishAuthentication(ctx, req.Challenge, req.Credential, func(rawID, userHandle []byte) (gwebauthn.User, error) {
		passkey, err := s.passkeyRepo.GetByCredentialID(ctx, rawID)
		if err != nil {
			return nil, err
		}
		if passkey == nil {
			return nil, ErrNoPasskeyForAccount
		}

		userID := passkey.UserID
		if len(userHandle) > 0 {
			userID = string(userHandle)
		}

		user, passkeyUser, _, err := s.loadPasskeyUser(ctx, userID)
		if err != nil {
			return nil, err
		}

		// Ensure the credential that unlocked the authenticator still belongs to the same user we will sign in.
		if user.ID != passkey.UserID {
			return nil, ErrNoPasskeyForAccount
		}

		return passkeyUser, nil
	})
	if err != nil {
		return nil, nil, err
	}

	resolvedUser, ok := passkeyUser.(*storedPasskeyUser)
	if !ok {
		return nil, nil, fmt.Errorf("unexpected discoverable passkey user type")
	}

	return resolvedUser.user, credential, nil
}

func (s *PasskeyService) finishKnownUserAuthentication(ctx context.Context, userID string, req model.PasskeyAuthenticateRequest) (*model.User, *gwebauthn.Credential, error) {
	user, passkeyUser, _, err := s.loadPasskeyUser(ctx, userID)
	if err != nil {
		return nil, nil, err
	}

	credential, err := s.webauthnClient.FinishAuthenticationForUser(ctx, passkeyUser, req.Challenge, req.Credential)
	if err != nil {
		return nil, nil, err
	}

	return user, credential, nil
}

func (s *PasskeyService) loadPasskeyUser(ctx context.Context, userID string) (*model.User, *storedPasskeyUser, []protocol.CredentialDescriptor, error) {
	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("get user: %w", err)
	}
	if user == nil {
		return nil, nil, nil, fmt.Errorf("user not found")
	}

	passkeys, err := s.passkeyRepo.ListByUser(ctx, user.ID)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("list user passkeys: %w", err)
	}

	credentials := make([]gwebauthn.Credential, 0, len(passkeys))
	descriptors := make([]protocol.CredentialDescriptor, 0, len(passkeys))
	for _, passkey := range passkeys {
		credential, err := storedPasskeyToCredential(passkey)
		if err != nil {
			return nil, nil, nil, err
		}
		credentials = append(credentials, credential)
		descriptors = append(descriptors, credential.Descriptor())
	}

	return user, &storedPasskeyUser{
		user:        user,
		credentials: credentials,
	}, descriptors, nil
}

func (s *PasskeyService) loadPasskeyUserByEmail(ctx context.Context, email string) (*model.User, *storedPasskeyUser, []protocol.CredentialDescriptor, error) {
	user, err := s.userRepo.GetByEmail(ctx, strings.TrimSpace(email))
	if err != nil {
		return nil, nil, nil, fmt.Errorf("get user by email: %w", err)
	}
	if user == nil {
		return nil, nil, nil, ErrNoPasskeyForAccount
	}

	userModel, passkeyUser, descriptors, err := s.loadPasskeyUser(ctx, user.ID)
	if err != nil {
		return nil, nil, nil, err
	}
	if len(descriptors) == 0 {
		return nil, nil, nil, ErrNoPasskeyForAccount
	}

	return userModel, passkeyUser, descriptors, nil
}

func (s *PasskeyService) isConfigured() bool {
	return s.webauthnClient != nil && s.webauthnClient.Configured()
}

type storedPasskeyUser struct {
	user        *model.User
	credentials []gwebauthn.Credential
}

func (u *storedPasskeyUser) WebAuthnID() []byte {
	return []byte(u.user.ID)
}

func (u *storedPasskeyUser) WebAuthnName() string {
	return u.user.Email
}

func (u *storedPasskeyUser) WebAuthnDisplayName() string {
	if strings.TrimSpace(u.user.FullName) != "" {
		return u.user.FullName
	}
	return u.user.Email
}

func (u *storedPasskeyUser) WebAuthnCredentials() []gwebauthn.Credential {
	return append([]gwebauthn.Credential(nil), u.credentials...)
}

func storedPasskeyToCredential(passkey model.UserPasskey) (gwebauthn.Credential, error) {
	var transports []protocol.AuthenticatorTransport
	if len(passkey.Transport) > 0 {
		if err := json.Unmarshal(passkey.Transport, &transports); err != nil {
			return gwebauthn.Credential{}, fmt.Errorf("decode passkey transports: %w", err)
		}
	}

	flags := gwebauthn.NewCredentialFlags(protocol.AuthenticatorFlags(passkey.Flags))
	if passkey.Verified {
		flags.UserVerified = true
	}

	return gwebauthn.Credential{
		ID:              append([]byte(nil), passkey.CredentialID...),
		PublicKey:       append([]byte(nil), passkey.PublicKey...),
		AttestationType: passkey.AttestationType,
		Transport:       transports,
		Flags:           flags,
		Authenticator: gwebauthn.Authenticator{
			AAGUID:    append([]byte(nil), passkey.AAGUID...),
			SignCount: uint32(passkey.SignCount),
		},
	}, nil
}

func normalizePasskeyName(name *string, userAgent string) string {
	if name != nil && strings.TrimSpace(*name) != "" {
		return strings.TrimSpace(*name)
	}

	ua := strings.ToLower(strings.TrimSpace(userAgent))
	switch {
	case strings.Contains(ua, "iphone"), strings.Contains(ua, "ipad"), strings.Contains(ua, "ios"):
		return "iPhone or iPad passkey"
	case strings.Contains(ua, "mac os"), strings.Contains(ua, "macintosh"):
		return "Mac passkey"
	case strings.Contains(ua, "windows"):
		return "Windows passkey"
	case strings.Contains(ua, "android"):
		return "Android passkey"
	case strings.Contains(ua, "linux"):
		return "Linux passkey"
	default:
		return "This device"
	}
}
