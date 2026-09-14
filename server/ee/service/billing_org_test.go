//go:build ee

package service

import (
	"reflect"
	"testing"
	"time"

	"github.com/helpin-ai/helpin/server/internal/model"
)

func TestOrderedModelTiersUsesCustomerFacingOrder(t *testing.T) {
	input := map[string]struct{}{"flagship": {}, "small": {}, "large": {}}
	want := []string{"small", "large", "flagship"}
	if got := orderedModelTiers(input); !reflect.DeepEqual(got, want) {
		t.Fatalf("orderedModelTiers() = %#v, want %#v", got, want)
	}
	if len(input) != 3 {
		t.Fatal("orderedModelTiers mutated its input")
	}
}

func TestAIUsageAllowancePercentage(t *testing.T) {
	tests := []struct {
		name      string
		charged   int64
		allowance int64
		want      float64
	}{
		{name: "part of allowance", charged: 25, allowance: 1_000, want: 2.5},
		{name: "above allowance", charged: 1_125, allowance: 1_000, want: 112.5},
		{name: "no allowance", charged: 25, allowance: 0, want: 0},
		{name: "no charge", charged: 0, allowance: 1_000, want: 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := aiUsageAllowancePercentage(tt.charged, tt.allowance); got != tt.want {
				t.Fatalf("aiUsageAllowancePercentage(%d, %d) = %v, want %v", tt.charged, tt.allowance, got, tt.want)
			}
		})
	}
}

func TestPriceCentsForPlan(t *testing.T) {
	tests := []struct {
		name     string
		plan     string
		interval string
		want     int
	}{
		{name: "legacy no-charge monthly", plan: "free", interval: "monthly", want: 0},
		{name: "legacy no-charge annual", plan: "free", interval: "annual", want: 0},
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
