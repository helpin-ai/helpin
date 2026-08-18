package handler

import "testing"

func TestParseSupportMessagePageLimit(t *testing.T) {
	tests := []struct {
		name    string
		raw     string
		want    int
		wantErr bool
	}{
		{name: "default", raw: "", want: 20},
		{name: "explicit", raw: "35", want: 35},
		{name: "zero", raw: "0", wantErr: true},
		{name: "too large", raw: "101", wantErr: true},
		{name: "not a number", raw: "many", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseSupportMessagePageLimit(tt.raw)
			if (err != nil) != tt.wantErr {
				t.Fatalf("parseSupportMessagePageLimit(%q) error = %v, wantErr %v", tt.raw, err, tt.wantErr)
			}
			if got != tt.want {
				t.Errorf("parseSupportMessagePageLimit(%q) = %d, want %d", tt.raw, got, tt.want)
			}
		})
	}
}
