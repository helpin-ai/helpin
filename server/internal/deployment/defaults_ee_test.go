//go:build ee

package deployment

import "testing"

func TestEnterpriseHidesSetupGuideByDefault(t *testing.T) {
	enabled, err := SetupGuidePolicy("")
	if err != nil || enabled {
		t.Fatalf("SetupGuidePolicy(\"\") = %v, %v; want false", enabled, err)
	}
}
