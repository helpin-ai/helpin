---
name: code_review
description: Interactive review-first behavior for code review runs.
metadata:
  title: Code Review
  supported_runtimes:
    - native_sdk
    - codex
    - opencode
---

- Inspect the relevant code and run targeted validation when possible.
- Focus on correctness, regressions, missing tests, and delivery risk.
- When a task review includes repository base and working branch context, start by comparing the task branch against the base branch before widening scope. Avoid a plain working-tree diff unless the human explicitly asks to review uncommitted changes.
- Report findings first, ordered by severity, with concrete file references when available.
- Avoid low-signal commentary and avoid proposing unnecessary rewrites.
- Treat review as an interactive loop, not a one-shot report.
- After the initial findings pass, produce a `review_checkpoint` handoff and stop. Treat that handoff as the final action in the turn. Do not keep working after it in the same turn.
- In `native_sdk`, emit that handoff with `request_review_checkpoint`. In `codex` and `opencode`, use the runtime-specific structured handoff format declared by the active runtime instructions.
- Include structured findings in the `review_checkpoint` payload when possible: `findings[]` with `title`, `body`, `priority`, `confidence`, and `code_location`, plus `overall_correctness`, `overall_explanation`, and `overall_confidence_score`.
- Use `review_checkpoint` to present the review findings for approval or change feedback before you edit code.
- After you answer a follow-up, hand control back with `request_user_input` unless the latest human reply clearly says the review is done.
- Use `request_user_input` to ask what should happen next. Prefer a short next-step question with options like follow-up discussion, re-review after changes, or done.
- Do not finish immediately after posting findings unless the latest human reply clearly says the review is done, finished, complete, or equivalent.
- If the human asks for clarification, answer it, then ask what to do next with `request_user_input`.
- If the human asks for another review pass after changes, perform the re-review, report the result, and ask what to do next with `request_user_input`.
- If the human asks you to implement changes based on the review, switch into implementation mode in the same branch and workspace, make the requested fixes directly, run focused validation, create a local commit only, then summarize what changed and finish the run unless the human explicitly asked to stay in the review loop.
- After implementing approved findings, if the final re-review is clean, there are no remaining findings, and the requested validation is complete or blocked only by an unavailable local dependency, summarize the clean outcome and finish the run. Do not open another `review_checkpoint` or `request_user_input` loop in that case.
- When implementing agreed fixes, keep the change scoped to the selected findings instead of rewriting unrelated code.
- Do not push the branch or open a pull request from inside the run. Remote delivery remains backend-managed after the run finally completes.
