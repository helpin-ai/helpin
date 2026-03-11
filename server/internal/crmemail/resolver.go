package crmemail

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/mail"
	"strings"
	"unicode"

	"gorm.io/gorm"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

// Participant represents an email participant before CRM contact resolution.
type Participant struct {
	Email string
	Name  string
	Role  string
}

// ResolveInput controls how message participants should be normalized and linked.
type ResolveInput struct {
	WorkspaceID string
	Direction   string
	Settings    *model.CRMEmailSyncSettings
	SelfEmails  []string
	From        Participant
	To          []Participant
	CC          []Participant
}

// ResolveResult contains normalized participants plus the resolved CRM links.
type ResolveResult struct {
	From             Participant
	To               []Participant
	CC               []Participant
	Associations     []model.CRMEmailMessageContact
	ContactIDs       []string
	PrimaryContactID *string
}

// Resolver matches message participants to contacts and creates new contacts
// when sync settings allow it.
type Resolver struct {
	contactRepo *repository.CRMContactRepository
}

// NewResolver creates a CRM email participant resolver.
func NewResolver(contactRepo *repository.CRMContactRepository) *Resolver {
	return &Resolver{contactRepo: contactRepo}
}

// Resolve normalizes email participants, resolves all external contacts, and
// derives the backward-compatible primary contact_id.
func (r *Resolver) Resolve(ctx context.Context, input ResolveInput) (*ResolveResult, error) {
	result := &ResolveResult{
		From: normalizeParticipant(input.From),
		To:   normalizeParticipants(input.To),
		CC:   normalizeParticipants(input.CC),
	}

	selfSet := make(map[string]struct{}, len(input.SelfEmails))
	for _, email := range input.SelfEmails {
		normalized := NormalizeEmailAddress(email)
		if normalized != "" {
			selfSet[normalized] = struct{}{}
		}
	}

	participants := append([]Participant{}, result.To...)
	participants = append(participants, result.CC...)
	if result.From.Email != "" {
		participants = append([]Participant{result.From}, participants...)
	}

	uniqueEmails := make([]string, 0, len(participants))
	seenEmails := make(map[string]struct{}, len(participants))
	for _, participant := range participants {
		if participant.Email == "" {
			continue
		}
		if _, isSelf := selfSet[participant.Email]; isSelf {
			continue
		}
		if _, exists := seenEmails[participant.Email]; exists {
			continue
		}
		seenEmails[participant.Email] = struct{}{}
		uniqueEmails = append(uniqueEmails, participant.Email)
	}

	resolvedContacts, err := r.contactRepo.ListByEmails(ctx, input.WorkspaceID, uniqueEmails)
	if err != nil {
		return nil, fmt.Errorf("resolve contacts by email: %w", err)
	}

	seenAssociation := make(map[string]struct{}, len(participants))
	seenContactIDs := make(map[string]struct{}, len(participants))
	for _, participant := range participants {
		if participant.Email == "" {
			continue
		}
		if _, isSelf := selfSet[participant.Email]; isSelf {
			continue
		}

		contact, ok := resolvedContacts[participant.Email]
		if !ok && shouldAutoCreateContact(input.Settings, input.Direction, participant.Email) {
			created, err := r.createContact(ctx, input.WorkspaceID, participant.Email, participant.Name)
			if err != nil {
				return nil, err
			}
			if created != nil {
				contact = *created
				resolvedContacts[participant.Email] = contact
				ok = true
			}
		}
		if !ok {
			continue
		}

		key := participant.Role + ":" + contact.ID
		if _, exists := seenAssociation[key]; exists {
			continue
		}
		seenAssociation[key] = struct{}{}
		result.Associations = append(result.Associations, model.CRMEmailMessageContact{
			ContactID:       contact.ID,
			ParticipantRole: participant.Role,
			WorkspaceID:     input.WorkspaceID,
		})
		if _, exists := seenContactIDs[contact.ID]; !exists {
			seenContactIDs[contact.ID] = struct{}{}
			result.ContactIDs = append(result.ContactIDs, contact.ID)
		}
	}

	if len(result.ContactIDs) == 1 {
		result.PrimaryContactID = &result.ContactIDs[0]
	}
	return result, nil
}

// NormalizeEmailAddress returns a normalized lowercase email address or an empty
// string when the input is not a valid email.
func NormalizeEmailAddress(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	addr, err := mail.ParseAddress(value)
	if err != nil {
		return ""
	}
	return strings.ToLower(strings.TrimSpace(addr.Address))
}

// ParseAddressJSONArray decodes a JSON string array and normalizes each entry.
func ParseAddressJSONArray(raw json.RawMessage) []string {
	if len(raw) == 0 {
		return nil
	}
	var values []string
	if err := json.Unmarshal(raw, &values); err != nil {
		return nil
	}
	result := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		email := NormalizeEmailAddress(value)
		if email == "" {
			continue
		}
		if _, exists := seen[email]; exists {
			continue
		}
		seen[email] = struct{}{}
		result = append(result, email)
	}
	return result
}

func normalizeParticipants(participants []Participant) []Participant {
	result := make([]Participant, 0, len(participants))
	seen := make(map[string]int, len(participants))
	for _, participant := range participants {
		normalized := normalizeParticipant(participant)
		if normalized.Email == "" {
			continue
		}
		key := normalized.Role + ":" + normalized.Email
		if idx, exists := seen[key]; exists {
			if result[idx].Name == "" && normalized.Name != "" {
				result[idx].Name = normalized.Name
			}
			continue
		}
		seen[key] = len(result)
		result = append(result, normalized)
	}
	return result
}

func normalizeParticipant(participant Participant) Participant {
	role := participant.Role
	if role == "" {
		role = model.CRMEmailParticipantRoleManual
	}
	return Participant{
		Email: NormalizeEmailAddress(participant.Email),
		Name:  strings.TrimSpace(participant.Name),
		Role:  role,
	}
}

func shouldAutoCreateContact(settings *model.CRMEmailSyncSettings, direction, email string) bool {
	if settings == nil {
		return false
	}
	switch settings.RecordCreationMode {
	case "always":
		// allowed
	case "selective":
		if direction != model.CRMEmailDirectionOutbound {
			return false
		}
	default:
		return false
	}
	return !model.IsBlockedRecordPrefix(settings, email)
}

func (r *Resolver) createContact(ctx context.Context, workspaceID, email, displayName string) (*model.CRMContact, error) {
	existing, err := r.contactRepo.GetByEmail(ctx, workspaceID, email)
	if err != nil {
		return nil, fmt.Errorf("get contact by email: %w", err)
	}
	if existing != nil {
		return existing, nil
	}

	displayID, err := r.contactRepo.GetNextDisplayID(ctx, workspaceID)
	if err != nil {
		return nil, fmt.Errorf("allocate contact display_id: %w", err)
	}

	firstName, lastName := deriveContactName(displayName, email)
	source := "email_sync"
	contact := &model.CRMContact{
		WorkspaceID:    workspaceID,
		DisplayID:      displayID,
		Email:          &email,
		FirstName:      firstName,
		LastName:       lastName,
		LifecycleStage: model.CRMLifecycleSubscriber,
		LeadStatus:     model.CRMLeadStatusNew,
		Source:         &source,
	}

	if err := r.contactRepo.Create(ctx, contact); err != nil {
		if isDuplicateCreateError(err) {
			existing, lookupErr := r.contactRepo.GetByEmail(ctx, workspaceID, email)
			if lookupErr != nil {
				return nil, fmt.Errorf("lookup duplicated contact by email: %w", lookupErr)
			}
			if existing != nil {
				return existing, nil
			}
		}
		return nil, fmt.Errorf("create contact from email: %w", err)
	}
	return contact, nil
}

func isDuplicateCreateError(err error) bool {
	if errors.Is(err, gorm.ErrDuplicatedKey) {
		return true
	}
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "duplicate key") || strings.Contains(message, "unique constraint")
}

func deriveContactName(displayName, email string) (string, *string) {
	displayName = strings.TrimSpace(displayName)
	if displayName == "" {
		displayName = humanizeLocalPart(email)
	}
	parts := strings.Fields(displayName)
	if len(parts) == 0 {
		fallback := "Contact"
		return fallback, nil
	}
	firstName := parts[0]
	if len(parts) == 1 {
		return firstName, nil
	}
	lastName := strings.Join(parts[1:], " ")
	return firstName, &lastName
}

func humanizeLocalPart(email string) string {
	localPart := email
	if at := strings.Index(localPart, "@"); at >= 0 {
		localPart = localPart[:at]
	}
	replacer := strings.NewReplacer(".", " ", "_", " ", "-", " ", "+", " ")
	localPart = replacer.Replace(localPart)
	fields := strings.Fields(localPart)
	if len(fields) == 0 {
		return "Contact"
	}
	for i := range fields {
		fields[i] = titleWord(fields[i])
	}
	return strings.Join(fields, " ")
}

func titleWord(value string) string {
	runes := []rune(strings.ToLower(value))
	if len(runes) == 0 {
		return value
	}
	runes[0] = unicode.ToUpper(runes[0])
	return string(runes)
}
