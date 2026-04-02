package repository

import (
	"testing"
	"time"

	"github.com/helpin-ai/helpin/server/internal/model"
)

func TestStartOfBoardWeekUsesMondayUTC(t *testing.T) {
	input := time.Date(2026, time.March, 7, 18, 30, 0, 0, time.UTC)
	got := startOfBoardWeek(input)
	want := time.Date(2026, time.March, 2, 0, 0, 0, 0, time.UTC)
	if !got.Equal(want) {
		t.Fatalf("startOfBoardWeek() = %s, want %s", got.Format(time.RFC3339), want.Format(time.RFC3339))
	}
}

func TestBuildDoneStoryGroupsBucketsByCompletionWeek(t *testing.T) {
	now := time.Date(2026, time.March, 7, 12, 0, 0, 0, time.UTC)

	stories := []model.BoardStory{
		{
			PMStory: model.PMStory{
				ID:          "story-1",
				UpdatedAt:   time.Date(2026, time.March, 6, 12, 0, 0, 0, time.UTC),
				CompletedAt: timePtr(time.Date(2026, time.March, 5, 9, 0, 0, 0, time.UTC)),
			},
		},
		{
			PMStory: model.PMStory{
				ID:          "story-2",
				UpdatedAt:   time.Date(2026, time.February, 19, 12, 0, 0, 0, time.UTC),
				CompletedAt: timePtr(time.Date(2026, time.February, 18, 9, 0, 0, 0, time.UTC)),
			},
		},
		{
			PMStory: model.PMStory{
				ID:          "story-3",
				UpdatedAt:   time.Date(2026, time.February, 17, 12, 0, 0, 0, time.UTC),
				CompletedAt: timePtr(time.Date(2026, time.February, 16, 9, 0, 0, 0, time.UTC)),
			},
		},
	}

	groups := buildDoneTaskGroups(stories, now)
	if len(groups) != 2 {
		t.Fatalf("len(groups) = %d, want 2", len(groups))
	}
	if groups[0].Label != boardDoneGroupThisWeekLabel {
		t.Fatalf("groups[0].label = %q, want %q", groups[0].Label, boardDoneGroupThisWeekLabel)
	}
	if len(groups[0].Stories) != 1 || groups[0].Stories[0].ID != "story-1" {
		t.Fatalf("groups[0].stories = %#v, want story-1 only", groups[0].Stories)
	}
	if groups[1].Label != "Week of Feb 16, 2026" {
		t.Fatalf("groups[1].label = %q, want %q", groups[1].Label, "Week of Feb 16, 2026")
	}
	if len(groups[1].Stories) != 2 {
		t.Fatalf("len(groups[1].stories) = %d, want 2", len(groups[1].Stories))
	}
}

func timePtr(v time.Time) *time.Time {
	return &v
}
