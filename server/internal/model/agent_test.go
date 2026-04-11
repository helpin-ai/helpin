package model

import "testing"

func TestJSONBlobValueReturnsJSONString(t *testing.T) {
	value, err := JSONBlob(`{"reasoning_effort":"high"}`).Value()
	if err != nil {
		t.Fatalf("Value() returned error: %v", err)
	}

	text, ok := value.(string)
	if !ok {
		t.Fatalf("Value() type = %T, want string", value)
	}
	if text != `{"reasoning_effort":"high"}` {
		t.Fatalf("Value() = %q, want %q", text, `{"reasoning_effort":"high"}`)
	}
}
