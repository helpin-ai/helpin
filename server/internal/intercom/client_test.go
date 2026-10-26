package intercom_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/helpin-ai/helpin/server/internal/intercom"
	"github.com/helpin-ai/helpin/server/internal/intercom/intercomtest"
)

func TestClientReadsPagesAndRetriesRateLimits(t *testing.T) {
	fake := intercomtest.New()
	defer fake.Close()
	for i := 0; i < 120; i++ {
		fake.Articles = append(fake.Articles, intercom.Article{ID: string(rune('a'+i%26)) + "x", Title: "Article"})
	}
	fake.Collections = []intercom.Collection{{ID: "1", Name: "Billing"}}
	for i := 0; i < 7; i++ {
		fake.Conversations = append(fake.Conversations, intercom.Conversation{ID: "c" + string(rune('0'+i)), State: "closed", CreatedAt: int64(1000 + i)})
	}
	fake.RateLimitNext = 2
	client := intercom.NewClient(intercomtest.Token, fake.URL)
	ctx := context.Background()

	me, err := client.Me(ctx)
	if err != nil || me.App.Name != "Acme" {
		t.Fatalf("me after rate limits: %+v %v", me, err)
	}
	articles, err := client.ListArticles(ctx)
	if err != nil || len(articles) != 120 {
		t.Fatalf("articles across pages: %d %v", len(articles), err)
	}
	collections, err := client.ListCollections(ctx)
	if err != nil || len(collections) != 1 {
		t.Fatalf("collections: %d %v", len(collections), err)
	}
	var ids []string
	cursor := ""
	for {
		page, err := client.SearchConversations(ctx, time.Unix(1002, 0), cursor, 2)
		if err != nil {
			t.Fatal(err)
		}
		for _, ref := range page.Conversations {
			ids = append(ids, ref.ID)
		}
		if page.Next == "" {
			break
		}
		cursor = page.Next
	}
	if len(ids) != 5 || ids[0] != "c2" || ids[4] != "c6" {
		t.Fatalf("searched conversations: %v", ids)
	}
	if _, err := client.GetConversation(ctx, "missing"); !errors.Is(err, intercom.ErrNotFound) {
		t.Fatalf("missing conversation: %v", err)
	}
	if _, err := intercom.NewClient("wrong", fake.URL).Me(ctx); !errors.Is(err, intercom.ErrUnauthorized) {
		t.Fatalf("bad token: %v", err)
	}
}
