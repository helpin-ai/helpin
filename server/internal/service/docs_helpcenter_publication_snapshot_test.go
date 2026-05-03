package service

import (
	"encoding/json"
	"testing"
)

func TestValidatePublicationSnapshotContentRejectsNonDoc(t *testing.T) {
	if _, err := validatePublicationSnapshotContent(json.RawMessage(`{"type":"paragraph"}`)); err == nil {
		t.Fatal("expected non-doc publication snapshot to be rejected")
	}
}

func TestPublicationContentEqualUsesPublishedFromSource(t *testing.T) {
	current := json.RawMessage(`{"type":"doc","content":[{"type":"codeBlock","attrs":{"language":"mermaid"},"content":[{"type":"text","text":"graph TD\nA-->B"}]}]}`)
	published := json.RawMessage(`{"type":"doc","content":[{"type":"resizableImage","attrs":{"src":"https://cdn.example.com/diagram.svg","publishedFrom":{"type":"codeBlock","attrs":{"language":"mermaid"},"content":[{"type":"text","text":"graph TD\nA-->B"}]}}}]}`)

	if !publicationContentEqual(current, published) {
		t.Fatal("expected publication image snapshot to compare equal to its source block")
	}
}

func TestPublicationContentEqualDetectsChangedPublishedFromSource(t *testing.T) {
	current := json.RawMessage(`{"type":"doc","content":[{"type":"codeBlock","attrs":{"language":"mermaid"},"content":[{"type":"text","text":"graph TD\nA-->C"}]}]}`)
	published := json.RawMessage(`{"type":"doc","content":[{"type":"resizableImage","attrs":{"src":"https://cdn.example.com/diagram.svg","publishedFrom":{"type":"codeBlock","attrs":{"language":"mermaid"},"content":[{"type":"text","text":"graph TD\nA-->B"}]}}}]}`)

	if publicationContentEqual(current, published) {
		t.Fatal("expected changed source block to differ from published snapshot")
	}
}

func TestPublicationContentEqualUsesExcalidrawPublishedFromSource(t *testing.T) {
	current := json.RawMessage(`{"type":"doc","content":[{"type":"excalidraw","attrs":{"title":"Checkout flow","scene":{"elements":[{"id":"shape-1","type":"rectangle"}],"appState":{},"files":{}}}}]}`)
	published := json.RawMessage(`{"type":"doc","content":[{"type":"resizableImage","attrs":{"src":"https://cdn.example.com/drawing.png","publishedFrom":{"type":"excalidraw","attrs":{"title":"Checkout flow","scene":{"elements":[{"id":"shape-1","type":"rectangle"}],"appState":{},"files":{}}}}}}]}`)

	if !publicationContentEqual(current, published) {
		t.Fatal("expected excalidraw image snapshot to compare equal to its source block")
	}
}

func TestPublicationContentEqualDetectsChangedExcalidrawPublishedFromSource(t *testing.T) {
	current := json.RawMessage(`{"type":"doc","content":[{"type":"excalidraw","attrs":{"title":"Checkout flow","scene":{"elements":[{"id":"shape-2","type":"ellipse"}],"appState":{},"files":{}}}}]}`)
	published := json.RawMessage(`{"type":"doc","content":[{"type":"resizableImage","attrs":{"src":"https://cdn.example.com/drawing.png","publishedFrom":{"type":"excalidraw","attrs":{"title":"Checkout flow","scene":{"elements":[{"id":"shape-1","type":"rectangle"}],"appState":{},"files":{}}}}}}]}`)

	if publicationContentEqual(current, published) {
		t.Fatal("expected changed excalidraw source block to differ from published snapshot")
	}
}
