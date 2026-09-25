package deployment

import "testing"

func TestSetupGuidePolicy(t *testing.T) {
	tests := []struct {
		name    string
		raw     string
		want    bool
		wantErr bool
	}{
		{name: "empty uses edition default", raw: "", want: SetupGuideDefault},
		{name: "whitespace uses edition default", raw: "  ", want: SetupGuideDefault},
		{name: "explicit true", raw: "true", want: true},
		{name: "explicit TRUE", raw: " TRUE ", want: true},
		{name: "explicit false", raw: "false", want: false},
		{name: "explicit zero", raw: "0", want: false},
		{name: "invalid value", raw: "maybe", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := SetupGuidePolicy(tt.raw)
			if (err != nil) != tt.wantErr {
				t.Fatalf("SetupGuidePolicy(%q) error = %v, wantErr %v", tt.raw, err, tt.wantErr)
			}
			if !tt.wantErr && got != tt.want {
				t.Errorf("SetupGuidePolicy(%q) = %v, want %v", tt.raw, got, tt.want)
			}
		})
	}
}
