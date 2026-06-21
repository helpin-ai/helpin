package service

import (
	"testing"
	"time"

	"github.com/helpin-ai/helpin/server/internal/model"
)

func TestPriceCentsForPlan(t *testing.T) {
	tests := []struct {
		name     string
		plan     string
		interval string
		want     int
	}{
		{name: "free monthly", plan: model.BillingPlanFree, interval: "monthly", want: 0},
		{name: "free annual", plan: model.BillingPlanFree, interval: "annual", want: 0},
		{name: "starter monthly", plan: model.BillingPlanStarter, interval: "monthly", want: 9900},
		{name: "starter annual", plan: model.BillingPlanStarter, interval: "annual", want: 94800},
		{name: "growth monthly", plan: model.BillingPlanGrowth, interval: "monthly", want: 29900},
		{name: "growth annual", plan: model.BillingPlanGrowth, interval: "annual", want: 286800},
		{name: "starter default interval is monthly", plan: model.BillingPlanStarter, interval: "", want: 9900},
		{name: "unknown plan", plan: "enterprise", interval: "monthly", want: 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := PriceCentsForPlan(tt.plan, tt.interval); got != tt.want {
				t.Errorf("PriceCentsForPlan(%q,%q) = %d, want %d", tt.plan, tt.interval, got, tt.want)
			}
		})
	}
}

func TestParseBillingPeriod(t *testing.T) {
	now := time.Date(2026, 6, 19, 12, 0, 0, 0, time.UTC)
	tests := []struct {
		name      string
		period    string
		wantStart time.Time
		wantEnd   time.Time
		wantErr   bool
	}{
		{
			name:      "explicit month",
			period:    "2026-03",
			wantStart: time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC),
			wantEnd:   time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC),
		},
		{
			name:      "empty defaults to current month",
			period:    "",
			wantStart: time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC),
			wantEnd:   time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC),
		},
		{
			name:      "december rolls to next year",
			period:    "2026-12",
			wantStart: time.Date(2026, 12, 1, 0, 0, 0, 0, time.UTC),
			wantEnd:   time.Date(2027, 1, 1, 0, 0, 0, 0, time.UTC),
		},
		{name: "invalid period", period: "2026/03", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			start, end, err := parseBillingPeriod(tt.period, now)
			if (err != nil) != tt.wantErr {
				t.Fatalf("parseBillingPeriod(%q) err = %v, wantErr %v", tt.period, err, tt.wantErr)
			}
			if tt.wantErr {
				return
			}
			if !start.Equal(tt.wantStart) {
				t.Errorf("start = %s, want %s", start, tt.wantStart)
			}
			if !end.Equal(tt.wantEnd) {
				t.Errorf("end = %s, want %s", end, tt.wantEnd)
			}
		})
	}
}

func TestParseUsageWindow(t *testing.T) {
	now := time.Date(2026, 6, 19, 12, 0, 0, 0, time.UTC)
	tests := []struct {
		name      string
		period    string
		start     string
		end       string
		wantStart time.Time
		wantEnd   time.Time
		wantErr   bool
	}{
		{
			name:      "explicit billing period dates",
			start:     "2026-06-21T10:30:00Z",
			end:       "2026-07-21T10:30:00Z",
			wantStart: time.Date(2026, 6, 21, 10, 30, 0, 0, time.UTC),
			wantEnd:   time.Date(2026, 7, 21, 10, 30, 0, 0, time.UTC),
		},
		{
			name:      "month period fallback",
			period:    "2026-03",
			wantStart: time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC),
			wantEnd:   time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC),
		},
		{
			name:    "end must be after start",
			start:   "2026-07-21T10:30:00Z",
			end:     "2026-06-21T10:30:00Z",
			wantErr: true,
		},
		{
			name:    "start and end must be paired",
			start:   "2026-06-21T10:30:00Z",
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			start, end, err := parseUsageWindow(tt.period, tt.start, tt.end, now)
			if (err != nil) != tt.wantErr {
				t.Fatalf("parseUsageWindow() err = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr {
				return
			}
			if !start.Equal(tt.wantStart) {
				t.Errorf("start = %s, want %s", start, tt.wantStart)
			}
			if !end.Equal(tt.wantEnd) {
				t.Errorf("end = %s, want %s", end, tt.wantEnd)
			}
		})
	}
}

// TestBuildUsageFeatures verifies the per-feature aggregation, cost lookup, and
// percentage math that GetWorkspaceUsage applies to grouped ledger rows.
func TestBuildUsageFeatures(t *testing.T) {
	featureCredits := map[string]int{
		BillingFeatureCodingRun:      300,
		BillingFeatureSupportAIReply: 100,
	}
	featureUsage := map[string]int{
		BillingFeatureCodingRun:      3,
		BillingFeatureSupportAIReply: 20,
	}
	total := 400

	features := buildUsageFeatures(featureCredits, featureUsage, total)

	if len(features) != 2 {
		t.Fatalf("expected 2 features, got %d", len(features))
	}
	// Sorted by credits desc: coding run first.
	if features[0].FeatureKey != BillingFeatureCodingRun {
		t.Fatalf("expected coding_run first, got %s", features[0].FeatureKey)
	}
	if features[0].Cost != 100 {
		t.Errorf("coding cost = %d, want 100", features[0].Cost)
	}
	if features[0].Usage != 3 {
		t.Errorf("coding usage = %d, want 3", features[0].Usage)
	}
	if features[0].Pct != 75 {
		t.Errorf("coding pct = %v, want 75", features[0].Pct)
	}
	if features[1].FeatureKey != BillingFeatureSupportAIReply {
		t.Errorf("expected support reply second, got %s", features[1].FeatureKey)
	}
	if features[1].Pct != 25 {
		t.Errorf("support pct = %v, want 25", features[1].Pct)
	}
}
