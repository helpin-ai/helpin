# Support greeting browser test report

This assessment helps Helpin contributors decide whether the opening-greeting shortcut is ready for rollout. It records local browser tests performed on 18 September 2026 with real provider calls. The greeting shortcut works, but the tested stack is **not ready for an end-to-end rollout**: inbox triage dominates greeting latency, and ordinary support replies cannot start their agent run.

## Environment and method

- Helpin revision: `09a6da904`; Agent Runtime revision: `2a04b96`, including its existing local edits. No application source was changed during this browser assessment.
- Built the current API, widget-core, SDK, and runtime. Used Chromium through Playwright, with a fresh browser context for each opening-message scenario.
- Adapted the historical [widget HTML fixture](../testing/support-widget-test.html) to a disposable local API, synthetic workspace, and newly built SDK. The historical fixture's original remote host and CDN were not tested.
- Used an isolated PostgreSQL database, NATS instance, and Redis database. Existing server processes and customer data were left untouched. The previously running API binary was older than the greeting implementation.
- Community API, real OpenRouter calls, Luna greeting route. The ordinary support profile selected `deepseek/deepseek-v4-flash-0731`, but runtime readiness rejected it before a model call. Jev was disabled; this is not a live Jev evaluation. Enterprise billing behavior was not browser-tested.
- Browser latency starts immediately before clicking Send and ends when the generated reply is visible. The static welcome message is excluded. Model-stage latency comes from persisted message metadata; triage duration comes from server logs. Messages travel over WebSocket, so no HTTP send-acknowledgment metric is reported.
- Five sequential greetings per configuration, without concurrent load. Diagnostic runs used while preparing the harness are excluded. Samples are small and configurations were tested sequentially, not randomly interleaved; these are observations, not production latency guarantees.

## Greeting latency

| Opening message | Triage enabled: browser | Triage LLM stage | Luna stage | Triage disabled: browser | Luna stage |
| --- | ---: | ---: | ---: | ---: | ---: |
| `hi` | 13.731 s | 12.070 s | 1.035 s | 1.669 s | 0.941 s |
| `hello` | 4.084 s | 2.500 s | 0.820 s | 1.657 s | 0.855 s |
| `hey` | 26.345 s | 24.699 s | 0.789 s | 1.659 s | 0.811 s |
| `good morning` | 16.277 s | 14.347 s | 0.959 s | 1.675 s | 0.854 s |
| `hola` | 26.363 s | 24.817 s | 0.841 s | 4.178 s | 3.355 s |
| **Median** | **16.277 s** | | | **1.669 s** | |
| **Mean** | **17.360 s** | | | **2.168 s** | |

All ten replies used `openai/gpt-5.6-luna`, carried `ai_pre_route=initial_greeting`, and avoided an active agent run. Responses were short and appropriate; `hola` received Spanish. Persisted usage was 99 tokens per greeting except `good morning`, which used 101. These counts describe the greeting call only; inbox triage adds separate usage.

Disabling triage only in the synthetic workspace reduced the observed median by about 90%. This isolates a useful opportunity; it is **not** evidence that the full agent system is 10× faster or cheaper, and disabling routing globally is not the proposed production fix. Provider invoices and a matched full-agent greeting baseline were not measured.

The reason is visible in [the widget automation path](../../server/internal/service/support_inbox_widget.go): `runWidgetPostMessageAutomation` awaits `EvaluateAndRoute` before publishing the AI request. The greeting shortcut runs afterward in [the support chat service](../../server/internal/service/support_chat_common.go). Its savings currently leave that preceding routing call intact.

[Successful greeting screenshot](support-greeting-browser-assets/greeting.png).

## Conversation and handoff checks

| Scenario | Observed result |
| --- | --- |
| Reload after a successful greeting | The persisted customer message and one AI greeting reappeared, with no duplicate reply. |
| First `hello`, then a billing question containing `TEST-742` | Greeting succeeded in 5.311 s. The billing question bypassed the greeting shortcut but produced no answer within 90 s. |
| First `hello`, then another `hi` | Opening greeting succeeded in 1.567 s with triage off. The second `hi` correctly attempted the ordinary agent path; no answer appeared within 20 s. |
| `Hi, my payment failed. Can you help me?` | Correctly avoided the greeting shortcut. No answer within 90 s; widget remained at “Looking into this…”. Triage was off for this scenario. |
| `I want to speak to a human` | No handoff or answer within 20 s; persisted state remained AI-pending with human takeover false. |
| `I want to talk to a human` | Handoff succeeded. At the five-second inspection the widget showed a team-notified reply and “Waiting for a teammate”; persisted state was escalated with human takeover true. |

The failed ordinary-agent scenarios logged the same error:

```text
start support chat run: agent runtime is not configured for openrouter run credentials
```

The current server's [runtime-readiness check](../../server/internal/service/ai_profile_readiness.go) requires per-run credential capabilities that the checked-out runtime does not advertise. The test runtime also lacked a Temporal durable executor, which ordinary support runs require. Updating the runtime contract and configuring its worker are prerequisites to completing this integration test; merely choosing another greeting model does not solve them.

Conversation continuity is therefore **not verified end to end**. The browser and stored messages confirm that mid-conversation text and mixed greeting/problem messages avoid the shortcut; they do not prove that a working support agent receives and remembers the full history.

The human-request difference is explained by [the hard escalation matcher](../../server/internal/service/support_ai_escalation.go): it includes `talk to a human` and `speak to someone`, but not `speak to a human`. With the ordinary agent unavailable, the latter phrasing has no working fallback.

[Blocked support question screenshot](support-greeting-browser-assets/blocked-support-question.png) · [Successful handoff screenshot](support-greeting-browser-assets/human-handoff.png).

## Recommended next work

1. Apply the same conservative opening-greeting decision before inbox AI triage, while preserving deterministic routing rules, ownership, channel policy, and publication-time checks. Re-measure the complete widget path afterward.
2. Align the runtime's per-run model credentials with the server and configure an isolated Temporal worker. Re-run real questions, invoice-reference recall, mid-conversation greetings, and mixed messages before claiming continuity works.
3. Cover common human-request paraphrases and ensure agent-launch failures lead to a visible recovery or handoff instead of an indefinite progress indicator.
4. After those fixes, repeat in staging with the real deployment configuration. Exercise provider timeout/failure, attachments, rapid successive messages, takeover races, private-draft mode, concurrent load, and actual billing. Those cases were not browser-validated here.

The [sanitized measurements](support-greeting-browser-assets/measurements.json) include message text, browser timings, route metadata, and observed errors. Prior unit/build checks are separate evidence; they do not turn these failed browser scenarios into passes. No deployment was performed.
