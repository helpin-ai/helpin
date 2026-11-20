// Package slack handles Slack's signed Events API and Web API protocol.
package slack

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"time"
)

// Verify authenticates the untouched request body and limits replay to five minutes.
func Verify(secret, stamp, signature string, body []byte, now time.Time) bool {
	seconds, err := strconv.ParseInt(stamp, 10, 64)
	if err != nil || secret == "" || seconds < now.Unix()-300 || seconds > now.Unix()+300 {
		return false
	}
	encoded, ok := strings.CutPrefix(signature, "v0=")
	if !ok {
		return false
	}
	expected, err := hex.DecodeString(encoded)
	if err != nil {
		return false
	}
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte("v0:" + stamp + ":"))
	mac.Write(body)
	return hmac.Equal(expected, mac.Sum(nil))
}

// Envelope identifies the installation and delivery.
type Envelope struct {
	Type      string  `json:"type"`
	Challenge string  `json:"challenge"`
	TeamID    string  `json:"team_id"`
	EventID   string  `json:"event_id"`
	Event     Message `json:"event"`
}

// Message carries the original Slack IDs as strings to preserve timestamp precision.
type Message struct {
	Type     string `json:"type"`
	Subtype  string `json:"subtype"`
	Channel  string `json:"channel"`
	User     string `json:"user"`
	Team     string `json:"team"`
	BotID    string `json:"bot_id"`
	TS       string `json:"ts"`
	ThreadTS string `json:"thread_ts"`
	Text     string `json:"text"`
	Files    []File `json:"files"`
}

// File holds only the metadata required to schedule an authenticated download.
type File struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Mime string `json:"mimetype"`
	Size int64  `json:"size"`
}

// Root identifies the conversation independently of the individual sender.
func (m Message) Root() string {
	if m.ThreadTS != "" {
		return m.ThreadTS
	}
	return m.TS
}

// HumanMessage admits ordinary channel messages and file shares, excluding bot echoes.
func (m Message) HumanMessage() bool {
	return (m.Type == "message" || m.Type == "app_mention") && (m.Subtype == "" || m.Subtype == "file_share") && m.BotID == "" && m.User != "" && m.TS != "" && m.Channel != ""
}

// Parse decodes an authenticated callback.
func Parse(body []byte) (Envelope, error) {
	var e Envelope
	if err := json.Unmarshal(body, &e); err != nil {
		return e, err
	}
	if e.Type == "event_callback" && (e.TeamID == "" || e.EventID == "") {
		return e, errors.New("incomplete Slack event envelope")
	}
	return e, nil
}
