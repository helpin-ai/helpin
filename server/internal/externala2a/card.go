package externala2a

import (
	"encoding/json"
	"fmt"
	"strings"
)

// Card is the validated subset of an A2A agent card Helpin displays and
// stores. It accepts A2A v1.0 cards (supportedInterfaces) and v0.3 cards
// (top-level url, preferredTransport and protocolVersion).
type Card struct {
	Name              string
	Description       string
	Version           string
	ProviderName      string
	InterfaceURL      string
	ProtocolBinding   string
	ProtocolVersion   string
	Streaming         bool
	PushNotifications bool
	Skills            []Skill
}

// Skill is one advertised agent skill.
type Skill struct {
	ID          string
	Name        string
	Description string
}

type rawCard struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Version     string `json:"version"`
	Provider    *struct {
		Organization string `json:"organization"`
	} `json:"provider"`
	SupportedInterfaces []rawInterface `json:"supportedInterfaces"`
	// A2A v0.3 fields.
	URL                  string         `json:"url"`
	PreferredTransport   string         `json:"preferredTransport"`
	ProtocolVersion      string         `json:"protocolVersion"`
	AdditionalInterfaces []rawInterface `json:"additionalInterfaces"`
	Capabilities         *struct {
		Streaming         bool `json:"streaming"`
		PushNotifications bool `json:"pushNotifications"`
	} `json:"capabilities"`
	Skills []struct {
		ID          string `json:"id"`
		Name        string `json:"name"`
		Description string `json:"description"`
	} `json:"skills"`
}

type rawInterface struct {
	URL             string `json:"url"`
	ProtocolBinding string `json:"protocolBinding"`
	ProtocolVersion string `json:"protocolVersion"`
	// v0.3 AgentInterface names the binding "transport".
	Transport string `json:"transport"`
}

const (
	maxSkills        = 100
	maxTextLength    = 4000
	defaultBinding   = "JSONRPC"
	legacyCardSchema = "0.3"
)

// ParseCard validates an agent card document.
func ParseCard(raw []byte) (*Card, error) {
	var doc rawCard
	if err := json.Unmarshal(raw, &doc); err != nil {
		return nil, fmt.Errorf("agent card is not valid JSON")
	}
	card := &Card{
		Name:        clip(doc.Name, 120),
		Description: clip(doc.Description, maxTextLength),
		Version:     clip(doc.Version, 64),
	}
	if card.Name == "" {
		return nil, fmt.Errorf("agent card has no name")
	}
	if doc.Provider != nil {
		card.ProviderName = clip(doc.Provider.Organization, 200)
	}
	if doc.Capabilities != nil {
		card.Streaming = doc.Capabilities.Streaming
		card.PushNotifications = doc.Capabilities.PushNotifications
	}
	iface, ok := pickInterface(doc.SupportedInterfaces)
	if ok {
		card.InterfaceURL = iface.URL
		card.ProtocolBinding = iface.ProtocolBinding
		card.ProtocolVersion = iface.ProtocolVersion
	} else if strings.TrimSpace(doc.URL) != "" {
		card.InterfaceURL = strings.TrimSpace(doc.URL)
		card.ProtocolBinding = normalizeBinding(doc.PreferredTransport)
		card.ProtocolVersion = strings.TrimSpace(doc.ProtocolVersion)
		if card.ProtocolVersion == "" {
			card.ProtocolVersion = legacyCardSchema
		}
		// A v0.3 card may list a JSON-RPC endpoint among its extra interfaces
		// while preferring another transport.
		if card.ProtocolBinding != defaultBinding {
			for _, extra := range doc.AdditionalInterfaces {
				if normalizeBinding(firstNonEmpty(extra.ProtocolBinding, extra.Transport)) == defaultBinding && strings.TrimSpace(extra.URL) != "" {
					card.InterfaceURL = strings.TrimSpace(extra.URL)
					card.ProtocolBinding = defaultBinding
					break
				}
			}
		}
	} else {
		return nil, fmt.Errorf("agent card has no supported interface URL")
	}
	for _, skill := range doc.Skills {
		if len(card.Skills) >= maxSkills {
			break
		}
		name := clip(skill.Name, 200)
		id := clip(skill.ID, 200)
		if name == "" && id == "" {
			continue
		}
		card.Skills = append(card.Skills, Skill{ID: id, Name: firstNonEmpty(name, id), Description: clip(skill.Description, maxTextLength)})
	}
	return card, nil
}

func pickInterface(interfaces []rawInterface) (rawInterface, bool) {
	var first *rawInterface
	for i := range interfaces {
		iface := interfaces[i]
		iface.URL = strings.TrimSpace(iface.URL)
		if iface.URL == "" {
			continue
		}
		iface.ProtocolBinding = normalizeBinding(firstNonEmpty(iface.ProtocolBinding, iface.Transport))
		iface.ProtocolVersion = strings.TrimSpace(iface.ProtocolVersion)
		if iface.ProtocolBinding == defaultBinding {
			return iface, true
		}
		if first == nil {
			first = &iface
		}
	}
	if first == nil {
		return rawInterface{}, false
	}
	return *first, true
}

func normalizeBinding(value string) string {
	value = strings.ToUpper(strings.TrimSpace(value))
	switch value {
	case "", "JSONRPC", "JSON-RPC", "JSON_RPC":
		return defaultBinding
	case "HTTP+JSON", "HTTP_JSON", "REST":
		return "HTTP+JSON"
	default:
		return value
	}
}

func clip(value string, limit int) string {
	value = strings.TrimSpace(value)
	if runes := []rune(value); len(runes) > limit {
		value = strings.TrimSpace(string(runes[:limit]))
	}
	return value
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
