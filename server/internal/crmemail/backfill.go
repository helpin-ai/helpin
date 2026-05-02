package crmemail

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

// BackfillRunner creates message-contact associations for legacy CRM email
// records that predate participant-based email linking.
type BackfillRunner struct {
	emailRepo        *repository.CRMEmailRepository
	syncSettingsRepo *repository.CRMEmailSyncSettingsRepository
	resolver         *Resolver
}

// NewBackfillRunner creates a legacy CRM email association backfill runner.
func NewBackfillRunner(
	emailRepo *repository.CRMEmailRepository,
	syncSettingsRepo *repository.CRMEmailSyncSettingsRepository,
	resolver *Resolver,
) *BackfillRunner {
	return &BackfillRunner{
		emailRepo:        emailRepo,
		syncSettingsRepo: syncSettingsRepo,
		resolver:         resolver,
	}
}

// Run processes messages missing participant associations in small batches and
// rebuilds thread-level contact caches.
func (r *BackfillRunner) Run(ctx context.Context) error {
	if r.emailRepo == nil || r.resolver == nil {
		return nil
	}

	settingsCache := map[string]*model.CRMEmailSyncSettings{}
	accountCache := map[string]*model.CRMEmailAccount{}

	for {
		messages, err := r.emailRepo.ListMessagesMissingAssociations(ctx, 100)
		if err != nil {
			return err
		}
		if len(messages) == 0 {
			break
		}

		madeProgress := false
		hadErrors := false
		for _, message := range messages {
			account, ok := accountCache[message.EmailAccountID]
			if !ok {
				account, err = r.emailRepo.GetAccountByID(ctx, message.EmailAccountID)
				if err != nil {
					slog.ErrorContext(ctx, "crm email backfill: load account", "error", err, "message_id", message.ID)
					hadErrors = true
					continue
				}
				if account == nil {
					continue
				}
				accountCache[message.EmailAccountID] = account
			}

			settings, ok := settingsCache[message.WorkspaceID]
			if !ok {
				if r.syncSettingsRepo != nil {
					settings, err = r.syncSettingsRepo.GetByWorkspace(ctx, message.WorkspaceID)
				}
				if err != nil {
					slog.ErrorContext(ctx, "crm email backfill: load sync settings", "error", err, "workspace_id", message.WorkspaceID)
					defaults := model.DefaultEmailSyncSettings()
					defaults.WorkspaceID = message.WorkspaceID
					settings = &defaults
				} else if settings == nil {
					defaults := model.DefaultEmailSyncSettings()
					defaults.WorkspaceID = message.WorkspaceID
					settings = &defaults
				}
				settingsCache[message.WorkspaceID] = settings
			}

			result, err := r.resolver.Resolve(ctx, ResolveInput{
				WorkspaceID: message.WorkspaceID,
				Direction:   message.Direction,
				Settings:    settings,
				SelfEmails:  []string{account.EmailAddress},
				From: Participant{
					Email: message.FromAddress,
					Name:  stringValue(message.FromName),
					Role:  model.CRMEmailParticipantRoleFrom,
				},
				To: participantsFromAddresses(ParseAddressJSONArray(message.ToAddresses), model.CRMEmailParticipantRoleTo),
				CC: participantsFromAddresses(ParseAddressJSONArray(message.CCAddresses), model.CRMEmailParticipantRoleCC),
			})
			if err != nil {
				slog.ErrorContext(ctx, "crm email backfill: resolve participants", "error", err, "message_id", message.ID)
				hadErrors = true
				continue
			}

			associations := cloneAssociationsForMessage(message.ID, result.Associations)
			primaryContactID := result.PrimaryContactID
			if len(associations) == 0 && message.ContactID != nil {
				associations = []model.CRMEmailMessageContact{{
					MessageID:       message.ID,
					ContactID:       *message.ContactID,
					ParticipantRole: model.CRMEmailParticipantRoleManual,
					WorkspaceID:     message.WorkspaceID,
				}}
				primaryContactID = message.ContactID
			}

			if err := r.emailRepo.ReplaceMessageContacts(ctx, message.ID, primaryContactID, associations); err != nil {
				slog.ErrorContext(ctx, "crm email backfill: save associations", "error", err, "message_id", message.ID)
				hadErrors = true
				continue
			}
			if len(associations) > 0 || primaryContactID != nil {
				madeProgress = true
			}
		}
		if !madeProgress {
			if hadErrors {
				return fmt.Errorf("crm email association backfill made no progress after processing %d messages", len(messages))
			}
			break
		}
	}

	return r.emailRepo.RefreshAllThreadContactIDs(ctx)
}

func cloneAssociationsForMessage(messageID string, associations []model.CRMEmailMessageContact) []model.CRMEmailMessageContact {
	if len(associations) == 0 {
		return nil
	}
	result := make([]model.CRMEmailMessageContact, 0, len(associations))
	for _, association := range associations {
		association.MessageID = messageID
		result = append(result, association)
	}
	return result
}

func participantsFromAddresses(addresses []string, role string) []Participant {
	result := make([]Participant, 0, len(addresses))
	for _, address := range addresses {
		result = append(result, Participant{Email: address, Role: role})
	}
	return result
}

func stringValue(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}
