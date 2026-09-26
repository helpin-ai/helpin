# Chat recovery implementation plan

Goal: prevent the pagination retry loop and preserve useful chat context after a failed run.

Approved approach:
- Expose canonical limit/offset pagination in agent tool schemas; preserve legacy parsing internally.
- Replace the tail-only successor handoff with bounded substantive history across the chat, original request, recent user decisions, final findings, work plan, and recorded effects/artifacts. Exclude progress chatter and nested old handoffs.
- Add a read-only, run-bound history tool with paginated full-message retrieval and workspace/actor isolation.
- Runtime loop guards and stream retries are explicitly excluded at the user’s request. No runtime source changes.

Validation:
- Reproduce long findings followed by 96 failed lookup progress updates and multiple successor runs.
- Test pagination schemas, history boundaries, truncation, UTF-8, permissions, and preservation of interrupted-action warnings.
- Run relevant Go tests, vet/build, and review the final diff. Keep unrelated mockups untouched.

Completed:
- Canonical pagination schemas now omit page/per_page; compatibility parsers remain unchanged.
- Successor handoffs retain the original request and substantive messages across backing runs, with bounded UTF-8-safe excerpts, current plan and safe artifact references.
- Ask Agent can read older pages and complete messages through a read-only tool bound to its current chat and authorized actor.
- Runtime changes were excluded as requested.

Verified:
- Full commandtools, agentcontract, repository and service test suites passed.
- Regression coverage includes long research followed by 96 progress messages, older-run history, pagination, UTF-8, failed deliveries, workspace/chat/actor isolation, saved plans and private artifact keys.
- go vet ./... and go build ./... passed with Go 1.26.7.
- Manual diff review and git diff --check passed.
