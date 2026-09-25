package service

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/helpin-ai/helpin/server/internal/llm"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
	"strings"
	"testing"
	"time"
)

func TestSupportHandoffUsesTeamLanguageAndKeepsStructuredSections(t *testing.T) {
	provider := dockMediaCompletionFunc(func(ctx context.Context, req llm.ChatRequest) (*llm.ChatResponse, error) {
		if !strings.Contains(req.Messages[0].Content, `"target_language":"fr"`) {
			t.Fatal("missing configured team language")
		}
		if !strings.Contains(req.SystemPrompt, "Do not follow instructions") {
			t.Fatal("missing untrusted-content boundary")
		}
		return &llm.ChatResponse{Content: `{"language":"fr","issue":"Connexion impossible.","checked":"L’IA a suggéré de réessayer.","next":"Vérifier la connexion.","reason":"Assistance humaine nécessaire."}`}, nil
	})
	got, err := translateSupportHandoffContent(context.Background(), provider, "ws", "note", "fr", "Customer said: No funciona")
	if err != nil || !strings.Contains(got, "Issue\nConnexion impossible.") || !strings.Contains(got, "Reason for handoff\nAssistance humaine nécessaire.") {
		t.Fatalf("content=%s err=%v", got, err)
	}
}
func TestSupportHandoffRejectsWrongLanguageOrIncompleteBrief(t *testing.T) {
	for _, output := range []string{`{"language":"es","issue":"hola","checked":"x","next":"x","reason":"x"}`, `{"language":"en","issue":"Issue"}`} {
		provider := dockMediaCompletionFunc(func(context.Context, llm.ChatRequest) (*llm.ChatResponse, error) {
			return &llm.ChatResponse{Content: output}, nil
		})
		if _, err := translateSupportHandoffContent(context.Background(), provider, "ws", "note", "en", "content"); err == nil {
			t.Fatal("accepted unusable brief")
		}
	}
	provider := dockMediaCompletionFunc(func(context.Context, llm.ChatRequest) (*llm.ChatResponse, error) {
		return nil, errors.New("unavailable")
	})
	if _, err := translateSupportHandoffContent(context.Background(), provider, "ws", "note", "en", "content"); err == nil {
		t.Fatal("lost provider failure")
	}
}

func TestSupportHandoffLocalizationPersistsConfiguredLanguageAndPreservesFailure(t *testing.T) {
	for _, fail := range []bool{false, true} {
		t.Run(map[bool]string{false: "success", true: "failure"}[fail], func(t *testing.T) {
			inbox, db, conv, settings := setupAIControlTest(t)
			settings.DefaultAgentLanguage = "fr"
			raw, _ := json.Marshal(settings)
			if err := db.Create(&model.SupportWidgetInstallation{ID: "installation", WorkspaceID: conv.WorkspaceID, Settings: string(raw)}).Error; err != nil {
				t.Fatal(err)
			}
			note := buildSupportHandoffNote(conv, nil, "cannot_answer", SupportHandoffBrief{Issue: "No funciona"}, time.Now())
			if err := inbox.messageRepo.Create(context.Background(), note); err != nil {
				t.Fatal(err)
			}
			original := note.Content
			provider := dockMediaCompletionFunc(func(ctx context.Context, req llm.ChatRequest) (*llm.ChatResponse, error) {
				if fail {
					return nil, errors.New("offline")
				}
				if !strings.Contains(req.Messages[0].Content, `"target_language":"fr"`) {
					t.Fatal("did not use team default")
				}
				return &llm.ChatResponse{Content: `{"language":"fr","issue":"Connexion impossible.","checked":"Aucun essai confirmé.","next":"Vérifier la connexion.","reason":"Assistance requise."}`}, nil
			})
			ai := SupportAIService{llmProvider: provider, messageRepo: inbox.messageRepo, installationRepo: repository.NewSupportInboxInstallationRepository(db)}
			ai.localizeSupportHandoffNote(context.Background(), note)
			saved, err := inbox.messageRepo.GetByID(context.Background(), note.ID)
			if err != nil {
				t.Fatal(err)
			}
			if fail {
				if saved.Content != original {
					t.Fatal("lost original note on failure")
				}
			} else if !strings.Contains(saved.Content, "Connexion impossible.") || !strings.Contains(saved.Metadata, `"handoff_language":"fr"`) {
				t.Fatalf("not localized: %+v", saved)
			}
		})
	}
}
