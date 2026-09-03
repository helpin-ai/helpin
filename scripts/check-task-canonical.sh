#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")/.."

legacy_pattern="\\bstory_id\\b|\\bstory_type\\b|\\bstory_planner\\b|\\bstory_plan(_doc)?\\b|\\bstory_refs\\b|\\bproposed_stories\\b|\\bcreated_story_ids\\b|\\bpm\\.create_followup_stories\\b|\\badd_story_comment\\b|\\blist_story_checklist\\b|\\bupdate_story_state\\b|\\bstory\\.state_entered\\b|[\"']story[\"']"

if matches=$(rg -n \
  --glob '!server/internal/dbmigrate/**' \
  --glob '!server/migrations/**' \
  --glob '!**/*_test.go' \
  --glob '!**/*.test.ts' \
  --glob '!**/*.test.tsx' \
  --glob '!server/internal/service/shortcut_*' \
  --glob '!server/internal/service/pm_import_shortcut_api.go' \
  --glob '!server/internal/model/crm_signal_score.go' \
  --glob '!server/internal/service/crm_signal_*' \
  --glob '!frontend/src/lib/crmTypes.ts' \
  --glob '!frontend/src/components/crm/SignalWorkspaceFeed.tsx' \
  "$legacy_pattern" server/internal frontend/src packages server/skills/system); then
  echo "Helpin-owned Story-era contracts remain:"
  echo "$matches"
  exit 1
fi

echo "Task canonical contract check passed."
