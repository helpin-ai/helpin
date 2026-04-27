package ordering

import (
	"encoding/json"
	"math/rand"
	"os"
	"testing"
)

// testVector represents a single test case from vectors.json.
type testVector struct {
	Lower    string `json:"lower"`
	Upper    string `json:"upper"`
	Expected string `json:"expected"`
	Error    bool   `json:"error"`
}

func loadVectors(t *testing.T) []testVector {
	t.Helper()
	data, err := os.ReadFile("testdata/vectors.json")
	if err != nil {
		t.Fatalf("failed to read vectors.json: %v", err)
	}
	var vecs []testVector
	if err := json.Unmarshal(data, &vecs); err != nil {
		t.Fatalf("failed to unmarshal vectors.json: %v", err)
	}
	return vecs
}

func TestBetween_Vectors(t *testing.T) {
	vecs := loadVectors(t)
	for i, v := range vecs {
		got, err := Between(v.Lower, v.Upper)
		if v.Error {
			if err == nil {
				t.Errorf("vector %d: Between(%q, %q) = %q, want error", i, v.Lower, v.Upper, got)
			}
			continue
		}
		if err != nil {
			t.Errorf("vector %d: Between(%q, %q) returned error: %v", i, v.Lower, v.Upper, err)
			continue
		}
		if got != v.Expected {
			t.Errorf("vector %d: Between(%q, %q) = %q, want %q", i, v.Lower, v.Upper, got, v.Expected)
		}
		// Verify ordering invariants.
		if v.Lower != "" && got <= v.Lower {
			t.Errorf("vector %d: result %q is not > lower %q", i, got, v.Lower)
		}
		if v.Upper != "" && got >= v.Upper {
			t.Errorf("vector %d: result %q is not < upper %q", i, got, v.Upper)
		}
	}
}

func TestBetween_StressOrdering(t *testing.T) {
	// Start with two boundary keys and insert 1000 items at random positions.
	keys := []string{"ae", "av"}

	rng := rand.New(rand.NewSource(42))

	for i := 0; i < 1000; i++ {
		// Pick a random gap.
		idx := rng.Intn(len(keys) - 1)
		lower := keys[idx]
		upper := keys[idx+1]
		mid, err := Between(lower, upper)
		if err != nil {
			t.Fatalf("iteration %d: Between(%q, %q) error: %v", i, lower, upper, err)
		}
		// Insert mid into the sorted slice at idx+1.
		keys = append(keys, "")
		copy(keys[idx+2:], keys[idx+1:])
		keys[idx+1] = mid
	}

	// Verify strictly ascending, no duplicates, and bounded length.
	seen := make(map[string]bool, len(keys))
	for i, k := range keys {
		if seen[k] {
			t.Fatalf("duplicate key at position %d: %q", i, k)
		}
		seen[k] = true
		if i > 0 && k <= keys[i-1] {
			t.Fatalf("not strictly ascending at position %d: %q <= %q", i, k, keys[i-1])
		}
		if len(k) >= 30 {
			t.Fatalf("key too long at position %d: %q (len=%d)", i, k, len(k))
		}
	}
	t.Logf("inserted 1000 keys, max length observed among %d keys", len(keys))
}

func TestBetween_SequentialAppend(t *testing.T) {
	var keys []string
	last := ""
	for i := 0; i < 100; i++ {
		k, err := Between(last, "")
		if err != nil {
			t.Fatalf("iteration %d: Between(%q, \"\") error: %v", i, last, err)
		}
		keys = append(keys, k)
		last = k
	}

	// Verify strictly ascending.
	for i := 1; i < len(keys); i++ {
		if keys[i] <= keys[i-1] {
			t.Fatalf("not strictly ascending at position %d: %q <= %q", i, keys[i], keys[i-1])
		}
	}
	t.Logf("100 sequential appends, last key: %q (len=%d)", keys[len(keys)-1], len(keys[len(keys)-1]))
}
