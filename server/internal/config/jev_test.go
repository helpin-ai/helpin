package config

import (
	"math"
	"testing"
)

func TestJevProbabilityConfiguration(t *testing.T) {
	if parseJevProbability("", .9) != .9 || parseJevProbability("0.97", .9) != .97 || !math.IsNaN(parseJevProbability("invalid", .9)) {
		t.Fatal("invalid Jev probability configuration")
	}
}
