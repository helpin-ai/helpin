---
name: code_review
description: Interactive review-first behavior for code review runs.
metadata:
  title: Code Review
  supported_runtimes:
    - native_sdk
---

- Inspect the relevant code and run targeted validation when possible.
- Focus on correctness, regressions, missing tests, and delivery risk.
- When a task review includes repository base and working branch context, start by comparing the task branch against the base branch before widening scope. Avoid a plain working-tree diff unless the human explicitly asks to review uncommitted changes.
- Report findings first, ordered by severity, with concrete file references when available.
- Avoid low-signal commentary and avoid proposing unnecessary rewrites.
- Treat review as an interactive loop, not a one-shot report.
- After the initial findings pass, produce a `review_checkpoint` handoff and stop. Treat that handoff as the final action in the turn. Do not keep working after it in the same turn.
- Emit that handoff with `request_review_checkpoint`.
- Include structured findings in the `review_checkpoint` payload when possible: `findings[]` with `title`, `body`, `priority`, `confidence`, and `code_location`, plus `overall_correctness`, `overall_explanation`, and `overall_confidence_score`.
- Use `review_checkpoint` to present the review findings for approval or change feedback before you edit code.
- After you answer a follow-up, hand control back with `request_user_input` only when a concrete human choice or value can still unlock useful work in the current run. Prefer a short next-step question with options like follow-up discussion, re-review after changes, or done.
- A reply that only promises a value later (for example, "I'll provide the path" or "I'll paste the config") does not supply that value. Do not ask for the same missing value again in the current run.
- When asking for a concrete free-text value, tell the human to paste the value in the free-text response. Do not offer a selectable option whose label merely promises to provide it; use a terminal option such as "Finish with blocker" instead.
- Ask at most once for a missing path, credential, deployment configuration, or other dependency outside the available workspace. If the reply does not contain the dependency, or the dependency remains inaccessible, summarize the completed work and the exact blocked remainder, then finish the run without another `request_user_input` or `review_checkpoint`.
- Treat an inaccessible external dependency as a delivery blocker, not as a new review finding. Never use `review_checkpoint` to collect a path/configuration or to reconfirm a blocker that the human has already been told about.
- Do not finish immediately after the initial findings pass unless the latest human reply clearly says the review is done, finished, complete, or equivalent. After that initial checkpoint has happened, finish whenever no useful in-scope action remains; an explicit "done" reply is not required.
- If the human asks for clarification, answer it, then ask what to do next with `request_user_input`.
- If the human asks for another review pass after changes, perform the re-review, report the result, and ask what to do next with `request_user_input`.
- If the human asks you to implement changes based on the review, switch into implementation mode in the same branch and workspace, make the requested fixes directly, run focused validation, create a local commit only, then summarize what changed and finish the run unless the human explicitly asked to stay in the review loop.
- In implementation mode, never call `edit_file` with identical `old_string` and `new_string`. If an edit reports no change or rejects an unchanged replacement, do not repeat that call. Inspect the current content once if needed, then make a materially different edit or conclude that no edit is needed.
- Reuse file content already available in context. When `read_files` pages a file, follow its `next_start_line`; do not reread the same range unless the file changed, the earlier result was incomplete, or compaction removed it from context. If the same tool result repeats, change approach or finish with the evidence already available.
- When you commit agreed fixes, make the final delivery summary useful to a pull-request reviewer: state what changed, list validation actually run with its outcome, include any remaining risks or unresolved findings, and identify the areas that deserve focused review. Never claim a check passed unless you ran it successfully.
- After implementing approved findings, if the final re-review is clean, summarize the clean outcome and finish the run. If an approved finding remains blocked only by an unavailable external dependency, report the partial completion and blocker once and finish as well. Do not open another `review_checkpoint` or `request_user_input` loop in either case.
- When implementing agreed fixes, keep the change scoped to the selected findings instead of rewriting unrelated code.
- Do not push the branch or open a pull request from inside the run. Remote delivery remains backend-managed after the run finally completes.
