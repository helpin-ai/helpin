# Chat-Backed Agent Run Sharing

## Goal

Allow a public link to be created for an Agent Run that belongs to an Ask Agent chat without weakening the parent chat's access controls. Preserve direct Ask chat sharing and standalone Agent Run sharing.

## Design

When authorizing an Agent Run share, load the run in its workspace. A standalone run is shareable by an authenticated workspace member with the existing command-bar permission. When the run has a `dock_chat_id`, authorize the actor through `DockChatService.GetChat`, which applies the parent chat's owner, workspace, or module visibility rules. If that check fails, return the existing not-found result without revealing the resource.

The public projection remains an Agent Run projection. A chat-backed run is not silently replaced with a parent-chat share because the user selected the Agent Run action and expects the run's events, interactions, and artifacts.

The share menu must preserve API errors. If creation returns no data, show the returned API error when present and fall back to `Unable to create public link` only when no useful error exists.

## Verification

- Service tests cover an accessible chat-backed run, an inaccessible chat-backed run, and a standalone run.
- Frontend tests cover surfacing the API creation error.
- Existing public-share service, handler, repository, and relevant frontend tests remain green.
