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

func TestJevLifecycleEnvironment(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://test")
	t.Setenv("JWT_SECRET", "test-only")
	t.Setenv("JEV_HANDOFF_MODE", "shadow")
	t.Setenv("JEV_FOLLOW_UP_MODE", "off")
	t.Setenv("JEV_HANDOFF_THRESHOLD", "0.98")
	t.Setenv("JEV_FOLLOW_UP_THRESHOLD", "")
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.JevHandoffMode != "shadow" || cfg.JevFollowUpMode != "off" || cfg.JevHandoffThreshold != .98 || cfg.JevFollowUpThreshold != .95 {
		t.Fatal("lifecycle environment not loaded")
	}
}

func TestJevPMEnvironment(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://test")
	t.Setenv("JWT_SECRET", "test-only")
	t.Setenv("JEV_PM_MODE", "shadow")
	t.Setenv("JEV_PM_THRESHOLD", "0.98")
	t.Setenv("JEV_PM_DAILY_LIMIT", "25")
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.JevPMMode != "shadow" || cfg.JevPMThreshold != .98 || cfg.JevPMDailyLimit != 25 {
		t.Fatal("PM configuration not loaded")
	}
}
