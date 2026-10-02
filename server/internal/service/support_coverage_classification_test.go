package service

import (
	"context"
	"testing"
)

func TestCoverageClassificationRejectsUnknownTypes(t *testing.T) {
	for _, gapType := range []string{"", "unknown", "made_up_type"} {
		t.Run(gapType, func(t *testing.T) {
			svc := NewSupportCoverageService(nil)
			if err := svc.ReclassifyGap(context.Background(), "ws-1", "gap-1", gapType); err == nil {
				t.Fatal("expected invalid classification to be rejected before accessing repository")
			}
		})
	}
}
