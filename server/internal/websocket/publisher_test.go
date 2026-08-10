package websocket

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	nhooyrws "nhooyr.io/websocket"
)

func TestPublisher_NilSafe(t *testing.T) {
	var p *Publisher
	// Calling Publish on a nil publisher should not panic.
	p.Publish(Event{Action: "created", Entity: "test", WorkspaceID: "ws-1"})
}

func TestNewOrderedJetStreamPublisherUsesSynchronousDelivery(t *testing.T) {
	publisher := NewOrderedJetStreamPublisher(nil)
	if publisher.queue != nil {
		t.Fatal("ordered JetStream publisher unexpectedly has an asynchronous queue")
	}
}

func TestPublisher_LocalOnly(t *testing.T) {
	hub := NewHub()
	publisher := NewPublisher(hub, nil)

	// Should not panic when relay is nil.
	publisher.Publish(Event{
		Action:      "created",
		Entity:      "pm_story",
		EntityID:    "story-1",
		WorkspaceID: "ws-1",
	})

	// Give the goroutine time to run.
	time.Sleep(50 * time.Millisecond)
}

func TestPublisher_LocalDeliveryPreservesPublishOrder(t *testing.T) {
	hub := NewHub()
	ready := make(chan struct{})
	done := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := nhooyrws.Accept(w, r, nil)
		if err != nil {
			t.Errorf("accept websocket: %v", err)
			return
		}
		client := &Client{Conn: conn, ConnID: "ordered-client", UserID: "user-1", WorkspaceID: "ws-1"}
		hub.Register(client)
		close(ready)
		<-done
		hub.Unregister(client)
		_ = conn.Close(nhooyrws.StatusNormalClosure, "test complete")
	}))
	defer server.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	conn, _, err := nhooyrws.Dial(ctx, "ws"+strings.TrimPrefix(server.URL, "http"), nil)
	if err != nil {
		t.Fatalf("dial websocket: %v", err)
	}
	defer conn.CloseNow()
	<-ready
	defer close(done)

	publisher := NewPublisher(hub, nil)
	const eventCount = 100
	for index := 0; index < eventCount; index++ {
		publisher.Publish(Event{
			Action:      "created",
			Entity:      "coding_session_event",
			EntityID:    fmt.Sprintf("event-%03d", index),
			WorkspaceID: "ws-1",
		})
	}

	for index := 0; index < eventCount; index++ {
		_, payload, err := conn.Read(ctx)
		if err != nil {
			t.Fatalf("read event %d: %v", index, err)
		}
		var event Event
		if err := json.Unmarshal(payload, &event); err != nil {
			t.Fatalf("decode event %d: %v", index, err)
		}
		if want := fmt.Sprintf("event-%03d", index); event.EntityID != want {
			t.Fatalf("event %d id = %q, want %q", index, event.EntityID, want)
		}
	}
}

func TestPublisher_DualPublish(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})

	hub := NewHub()
	relay := NewRedisEventPublisher(rdb, "pod-test")
	publisher := NewPublisher(hub, relay)

	// Subscribe to the workspace channel to verify Redis publish.
	sub := rdb.Subscribe(context.Background(), "ws:events:ws-1")
	defer sub.Close()
	_, err := sub.Receive(context.Background())
	if err != nil {
		t.Fatalf("subscribe: %v", err)
	}

	event := Event{
		Action:      "created",
		Entity:      "pm_story",
		EntityID:    "story-1",
		WorkspaceID: "ws-1",
		ActorID:     "user-1",
	}

	publisher.Publish(event)

	// Verify the event was published to Redis.
	ch := sub.Channel()
	select {
	case msg := <-ch:
		if msg.Channel != "ws:events:ws-1" {
			t.Errorf("expected channel ws:events:ws-1, got %s", msg.Channel)
		}
		var env relayEnvelope
		if err := json.Unmarshal([]byte(msg.Payload), &env); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		if env.OriginPod != "pod-test" {
			t.Errorf("expected origin_pod pod-test, got %s", env.OriginPod)
		}
		if env.Event.EntityID != "story-1" {
			t.Errorf("expected entity_id story-1, got %s", env.Event.EntityID)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for message on Redis channel")
	}
}

func TestBroadcastAll_LocalOnly(t *testing.T) {
	hub := NewHub()

	// BroadcastAll with nil relay should not panic — just does local broadcast.
	hub.BroadcastAll(Event{
		Action:      "created",
		Entity:      "pm_story",
		EntityID:    "story-1",
		WorkspaceID: "ws-1",
	})

	// Give the goroutine time to run.
	time.Sleep(50 * time.Millisecond)
}

func TestBroadcastAll_WithRelay(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})

	hub := NewHub()
	relay := NewRedisEventPublisher(rdb, "pod-test")
	hub.SetRelay(relay)

	// Subscribe to the workspace channel to verify Redis publish.
	sub := rdb.Subscribe(context.Background(), "ws:events:ws-1")
	defer sub.Close()
	_, err := sub.Receive(context.Background())
	if err != nil {
		t.Fatalf("subscribe: %v", err)
	}

	event := Event{
		Action:      "updated",
		Entity:      "support_conversation",
		EntityID:    "conv-1",
		WorkspaceID: "ws-1",
		ActorID:     "user-1",
	}

	hub.BroadcastAll(event)

	// Verify the event was published to Redis.
	ch := sub.Channel()
	select {
	case msg := <-ch:
		if msg.Channel != "ws:events:ws-1" {
			t.Errorf("expected channel ws:events:ws-1, got %s", msg.Channel)
		}
		var env relayEnvelope
		if err := json.Unmarshal([]byte(msg.Payload), &env); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		if env.Event.Entity != "support_conversation" {
			t.Errorf("expected entity support_conversation, got %s", env.Event.Entity)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for message on Redis channel")
	}
}

func TestHub_Register_EnsuresWorkspaceSubscription(t *testing.T) {
	_, rdb := setupMiniredis(t)

	hub := NewHub()
	relay := NewRedisRelay(rdb, hub, "pod-1")
	hub.SetRelay(relay)

	client := &Client{UserID: "user-1", WorkspaceID: "ws-1", IsWidget: false}
	hub.Register(client)

	if relay.RefCount("ws-1") != 1 {
		t.Errorf("expected ref count 1 after Register, got %d", relay.RefCount("ws-1"))
	}

	// Register another client for the same workspace.
	client2 := &Client{UserID: "user-2", WorkspaceID: "ws-1", IsWidget: false}
	hub.Register(client2)

	if relay.RefCount("ws-1") != 2 {
		t.Errorf("expected ref count 2, got %d", relay.RefCount("ws-1"))
	}

	// Unregister one — ref count decreases.
	hub.Unregister(client)
	if relay.RefCount("ws-1") != 1 {
		t.Errorf("expected ref count 1 after Unregister, got %d", relay.RefCount("ws-1"))
	}

	// Unregister last — ref count 0.
	hub.Unregister(client2)
	if relay.RefCount("ws-1") != 0 {
		t.Errorf("expected ref count 0 after final Unregister, got %d", relay.RefCount("ws-1"))
	}
}

func TestHub_Unregister_BroadcastsDisconnectViaRelay(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})

	hub := NewHub()
	relay := NewRedisEventPublisher(rdb, "pod-test")
	hub.SetRelay(relay)

	// Subscribe to workspace channel.
	sub := rdb.Subscribe(context.Background(), "ws:events:ws-1")
	defer sub.Close()
	_, err := sub.Receive(context.Background())
	if err != nil {
		t.Fatalf("subscribe: %v", err)
	}

	// Set up an agent client with viewing state.
	client := &Client{ConnID: "conn-1", UserID: "user-1", WorkspaceID: "ws-1", IsWidget: false}
	hub.Register(client)
	hub.Presence.SetViewing(context.Background(), "ws-1", "conv-1", "user-1", "conn-1")

	// Unregister should broadcast viewing_stopped via BroadcastAll → Redis.
	hub.Unregister(client)

	ch := sub.Channel()
	select {
	case msg := <-ch:
		var env relayEnvelope
		if err := json.Unmarshal([]byte(msg.Payload), &env); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		if env.Event.Action != "viewing_stopped" {
			t.Errorf("expected viewing_stopped, got %s", env.Event.Action)
		}
		if env.Event.EntityID != "conv-1" {
			t.Errorf("expected entity_id conv-1, got %s", env.Event.EntityID)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for disconnect broadcast on Redis")
	}
}
