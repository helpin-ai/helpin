package service

import (
	"context"
	"log/slog"

	"github.com/helpin-ai/helpin/server/internal/repository"
)

// MirrorBackfillReport summarizes a BackfillMissingDefaultLocaleMirrors run.
type MirrorBackfillReport struct {
	HelpCenters int
	Gaps        []repository.HelpcenterMirrorGap
	Failed      int
}

// BackfillMissingDefaultLocaleMirrors finds help centers missing default-locale
// mirror rows and, when apply is true, writes them. onFixed runs after each
// workspace is repaired, so callers can clear its public cache. Public reads
// do not write mirrors, so this repairs data written outside the services.
func (s *DocsHelpcenterTranslationService) BackfillMissingDefaultLocaleMirrors(
	ctx context.Context,
	workspaceID string,
	apply bool,
	onFixed func(ctx context.Context, workspaceID string),
) (MirrorBackfillReport, error) {
	gaps, err := s.translationRepo.ListMirrorGaps(ctx, workspaceID)
	if err != nil {
		return MirrorBackfillReport{}, err
	}

	report := MirrorBackfillReport{HelpCenters: len(gaps)}
	for _, gap := range gaps {
		if gap.Total() == 0 {
			continue
		}
		report.Gaps = append(report.Gaps, gap)
		if !apply {
			continue
		}
		if err := s.EnsureDefaultLocaleMirrorsForWorkspace(ctx, gap.WorkspaceID); err != nil {
			report.Failed++
			slog.ErrorContext(ctx, "helpcenter mirror backfill failed", "workspace_id", gap.WorkspaceID, "error", err)
			continue
		}
		if onFixed != nil {
			onFixed(ctx, gap.WorkspaceID)
		}
	}
	return report, nil
}
