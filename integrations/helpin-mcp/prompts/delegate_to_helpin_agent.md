# Delegate to a Helpin agent

Confirm context and list available agents. Verify the exact agent and target, then call `start_agent_run` once with a stable idempotency key. Poll `get_agent_run` until it reaches a terminal or user-input state. Report artifacts and links; never create duplicate active runs.
