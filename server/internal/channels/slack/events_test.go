package slack

import (
	"crypto/hmac"
	"crypto/sha256"
	"fmt"
	"testing"
	"time"
)

func TestSignedEventsBindBodyAndTimestamp(t *testing.T) {
	now := time.Unix(1700000000, 0)
	body := []byte(`{"type":"url_verification","challenge":"abc"}`)
	mac := hmac.New(sha256.New, []byte("signing-key"))
	fmt.Fprintf(mac, "v0:%d:%s", now.Unix(), body)
	sig := fmt.Sprintf("v0=%x", mac.Sum(nil))
	if !Verify("signing-key", "1700000000", sig, body, now) {
		t.Fatal("valid signature rejected")
	}
	if Verify("signing-key", "1700000000", sig, append(body, ' '), now) || Verify("signing-key", "1700000000", sig, body, now.Add(6*time.Minute)) {
		t.Fatal("accepted altered or stale event")
	}
}

func TestSlackThreadKeepsSenderSeparate(t *testing.T) {
	raw := []byte(`{"type":"event_callback","team_id":"T1","event_id":"Ev1","event":{"type":"message","channel":"C1","user":"U2","ts":"12.2","thread_ts":"10.1","text":"same here"}}`)
	env, err := Parse(raw)
	if err != nil || env.Event.Root() != "10.1" || env.Event.User != "U2" || env.TeamID != "T1" {
		t.Fatalf("%+v %v", env, err)
	}
	if !env.Event.HumanMessage() {
		t.Fatal("ignored human reply")
	}
	env.Event.BotID = "B1"
	if env.Event.HumanMessage() {
		t.Fatal("accepted bot echo")
	}
}
