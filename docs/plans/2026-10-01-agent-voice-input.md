# Agent voice input

Use `openai/gpt-transcribe` through Helpin’s existing OpenRouter integration for Ask Agent, dock run replies, and expanded run replies. The accepted flow is record, stop, transcribe, edit, and manually send. Recordings never send a message automatically.

## UI references

Inspected actual screenshots of [ChatGPT mobile](https://www.applesfera.com/trucos/probe-pregunta-viral-chatgpt-ahora-se-como-seria-mi-vida-sonada-puede-cambiar-tuya), [Claude mobile](https://www.testingcatalog.com/claudes-voice-dictation-debuts-on-ios-and-android-for-hands-free-use/), and [Copilot desktop](https://b2b-contenthub.com/wp-content/uploads/2025/03/ms-copilot-tips-05-voice-chat.jpg). ChatGPT and Claude put cancel, waveform, timer, and completion inside the composer. Adopt that compact arrangement with an explicit Stop label; preserve Helpin's Quiet Hairline styling. Copilot's separate listening screen is less suitable for editing an agent prompt.

## Implementation

- [x] Add a bounded OpenRouter transcription adapter and authenticated workspace endpoint, governed audit, and duration-based usage accounting.
- [x] Capture at most five minutes of mono PCM audio; validate WAV duration on the server before spending provider credits. Keep audio in memory only.
- [x] Add shared recording controls with actual input levels, cancel, timer, Stop, transcription progress, and actionable errors.
- [x] Integrate all three composers; preserve typed drafts and prevent sends while recording/transcribing. Cancel on navigation, inactivity, and unmount.
- [x] Verify client, service, metering, lifecycle cleanup, draft preservation, keyboard, and rendered narrow/wide light/dark states.

OpenRouter's [model reference](https://openrouter.ai/openai/gpt-transcribe) lists $0.0045/minute. Snapshot duration pricing separately from token pricing. Server credentials remain private; no automatic retries of billable requests. Browser microphone access requires HTTPS or localhost.

## Validation

Implemented in the existing `waqar-fixes` worktree.

- Core Go suites passed: llm, aiusage, aipolicy, handler, router, service, repository, and API composition.
- Hosted usage/pricing regression suites and targeted duration reservation/reconciliation tests passed.
- Frontend composer regression tests passed (33 existing tests); 12 voice tests cover capture lifecycle, permission denial, cancellation, stale results, maximum duration, WAV encoding, preserving edits during transcription, and manual sending.
- Chromium used a synthetic microphone with the real AudioWorklet and WAV encoder. The API response was mocked. Checked recording, transcription, editable results, cancel, keyboard send gates, 375 px overflow, desktop light mode, and mobile dark mode. Inspected the resulting screenshots.
- Frontend TypeScript and new-code ESLint checks passed. The initial feature’s production build passed, including the recorder as a separate JavaScript asset (standard chunk-size warnings only).
- Documentation naming and maintained-link checks passed. Migration contract tests passed. The migration CLI could not run database validation because this shell has no `DATABASE_URL`; no migration was applied.

## Configuration and limits

The microphone is available when the API has `OPENROUTER_API_KEY` configured; `OPENROUTER_BASE_URL` is honored. Apply `202610010002_voice_input_usage.sql` through the normal migration process. Audio stays in browser/server memory and is sent to OpenRouter only after stopping. Duration is validated from PCM samples, independently of client claims. Hosted usage reserves the validated duration and reconciles provider-reported duration, falling back to the validated PCM sample count if OpenRouter omits it; Community records duration without charging.

No paid OpenRouter request or physical microphone test was performed. Safari and Firefox were not exercised. Temporary browser checks and screenshots are in `/tmp/helpin-voice-preview/` for this session.

The user selected a five-minute cap. Browser capture, PCM encoding, server upload validation, advertised capabilities, and metering admission share that limit. Five minutes of 16 kHz mono PCM is about 9.6 MB, below OpenRouter’s [25 MB multipart upload limit](https://openrouter.ai/docs/guides/overview/multimodal/stt). OpenRouter’s normalized response has optional `usage.seconds` and does not require OpenAI’s `usage.type` field.

OpenRouter/five-minute follow-up validation: 12 frontend voice tests passed; targeted transcription client, service, handler, action-policy, and hosted metering tests passed; API composition compiled; changed UI code passed ESLint; documentation checks and `git diff --check` passed. No paid upstream request was made.
