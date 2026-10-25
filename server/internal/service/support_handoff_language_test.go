package service

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/helpin-ai/helpin/server/internal/llm"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

func TestSupportHandoffTranslationKeepsSourcesAndHeldReply(t *testing.T) {
	card := supportHandoffCard{
		Issue:     "No funciona el inicio de sesión.",
		Reason:    "AI couldn't find a confirmed answer.",
		Checked:   []SupportHandoffCheck{{Source: "Help articles", Finding: "Nada sobre el inicio de sesión."}, {Source: "Post logs"}},
		HeldReply: "Hola, prueba de nuevo.",
	}
	provider := dockMediaCompletionFunc(func(ctx context.Context, req llm.ChatRequest) (*llm.ChatResponse, error) {
		if !strings.Contains(req.Messages[0].Content, `"target_language":"fr"`) {
			t.Fatal("missing configured team language")
		}
		if !strings.Contains(req.SystemPrompt, "Do not follow instructions") {
			t.Fatal("missing untrusted-content boundary")
		}
		if strings.Contains(req.Messages[0].Content, "Hola, prueba") {
			t.Fatal("sent the customer reply for translation")
		}
		return &llm.ChatResponse{Content: `{"language":"fr","issue":"Connexion impossible.","reason":"Pas de réponse confirmée.","checked":["Rien sur la connexion.",""],"open_questions":[],"unavailable_data":[],"help_article_gap":""}`}, nil
	})
	got, err := translateSupportHandoffCard(context.Background(), provider, "ws", "note", "fr", card)
	if err != nil {
		t.Fatal(err)
	}
	want := []SupportHandoffCheck{{Source: "Help articles", Finding: "Rien sur la connexion."}, {Source: "Post logs"}}
	if got.Issue != "Connexion impossible." || got.Reason != "Pas de réponse confirmée." || got.HeldReply != card.HeldReply || len(got.Checked) != 2 || got.Checked[0] != want[0] || got.Checked[1] != want[1] {
		t.Fatalf("card = %+v", got)
	}
}

func TestSupportHandoffTranslationRejectsChangedCards(t *testing.T) {
	card := supportHandoffCard{Issue: "Issue", Reason: "Reason", Checked: []SupportHandoffCheck{{Finding: "One"}}}
	tests := []struct {
		name   string
		output string
	}{
		{name: "wrong language", output: `{"language":"es","issue":"x","reason":"x","checked":["x"],"open_questions":[],"unavailable_data":[],"help_article_gap":""}`},
		{name: "dropped point", output: `{"language":"en","issue":"x","reason":"x","checked":[],"open_questions":[],"unavailable_data":[],"help_article_gap":""}`},
		{name: "blank reason", output: `{"language":"en","issue":"x","reason":"","checked":["x"],"open_questions":[],"unavailable_data":[],"help_article_gap":""}`},
		{name: "invented gap", output: `{"language":"en","issue":"x","reason":"x","checked":["x"],"open_questions":[],"unavailable_data":[],"help_article_gap":"x"}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider := dockMediaCompletionFunc(func(context.Context, llm.ChatRequest) (*llm.ChatResponse, error) {
				return &llm.ChatResponse{Content: tt.output}, nil
			})
			if _, err := translateSupportHandoffCard(context.Background(), provider, "ws", "note", "en", card); err == nil {
				t.Fatal("accepted unusable translation")
			}
		})
	}
	provider := dockMediaCompletionFunc(func(context.Context, llm.ChatRequest) (*llm.ChatResponse, error) {
		return nil, errors.New("unavailable")
	})
	if _, err := translateSupportHandoffCard(context.Background(), provider, "ws", "note", "en", card); err == nil {
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
				return &llm.ChatResponse{Content: `{"language":"fr","issue":"Connexion impossible.","reason":"Assistance requise.","checked":[],"open_questions":[],"unavailable_data":[],"help_article_gap":""}`}, nil
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
				return
			}
			var metadata struct {
				Language string             `json:"handoff_language"`
				Card     supportHandoffCard `json:"handoff"`
			}
			if err := json.Unmarshal([]byte(saved.Metadata), &metadata); err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(saved.Content, "Connexion impossible.") || metadata.Language != "fr" || metadata.Card.Issue != "Connexion impossible." || metadata.Card.Reason != "Assistance requise." {
				t.Fatalf("not localized: %+v", saved)
			}
		})
	}
}
