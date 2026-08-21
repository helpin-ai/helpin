// Copyright 2019-2026 Helpin
// SPDX-License-Identifier: Apache-2.0
// SPDX-License-Identifier: MIT
//
// guest-js bindings for `tauri-plugin-helpin-push`. Consumed directly by the
// app via the `@helpin/plugin-push` Vite/tsconfig alias (see
// `apps/support-mobile/vite.config.ts` and `tsconfig.app.json`) rather than a
// built npm package — see this plugin's README for why.

import { invoke, addPluginListener, type PluginListener } from '@tauri-apps/api/core'
import type { UnlistenFn } from '@tauri-apps/api/event'

/** The FCM/APNs data payload delivered on notification tap. */
export type PushTapPayload = Record<string, string>

/** Notification content + data delivered while the app is foregrounded. */
export type PushReceivedPayload = Record<string, string>

interface GetPushTokenResponse {
  token: string | null
}

interface TokenChangedEvent {
  token: string
}

interface TakePendingTapResponse {
  tap: PushTapPayload | null
}

/**
 * How long (ms) after delivering a tap payload an identical payload is
 * treated as a duplicate. Covers the same tap arriving via both the live
 * `push-tapped` event and the one-shot `take_pending_tap` drain — those two
 * always land within the same registration tick, so a short window is
 * plenty, while a user genuinely tapping the same notification content
 * twice minutes apart is still delivered.
 */
const TAP_DEDUPE_WINDOW_MS = 3_000

/**
 * Returns the current FCM registration token, or `null` if unavailable
 * (desktop targets, denied permission, or the native handshake hasn't
 * completed yet).
 *
 * Requests the OS notification permission on the first call:
 * - iOS: `UNUserNotificationCenter` authorization prompt.
 * - Android 13+ (API 33+): `POST_NOTIFICATIONS` runtime permission.
 *
 * SPIKE-VERIFY: confirm the permission prompt reliably fires from inside
 * the native `getPushToken` handler on both platforms rather than needing a
 * separate explicit authorization call before a token is obtainable.
 *
 * @since 0.1.0 (pre-spike, unreleased)
 */
export async function getPushToken(): Promise<string | null> {
  const response = await invoke<GetPushTokenResponse>('plugin:helpin-push|get_push_token')
  return response.token
}

function toUnlistenFn(listener: PluginListener): UnlistenFn {
  return () => {
    void listener.unregister()
  }
}

/**
 * Subscribes to native FCM/APNs token rotation (fresh install, restore,
 * token expiry/refresh).
 *
 * SPIKE-VERIFY: Android emits this from
 * `FirebaseMessagingService.onNewToken`; iOS from
 * `MessagingDelegate.didReceiveRegistrationToken`. Confirm both fire
 * reliably and exactly once per new token on real hardware.
 *
 * @since 0.1.0 (pre-spike, unreleased)
 */
export async function onPushTokenChanged(cb: (token: string) => void): Promise<UnlistenFn> {
  const listener = await addPluginListener<TokenChangedEvent>(
    'helpin-push',
    'push-token-changed',
    (payload) => cb(payload.token),
  )
  return toUnlistenFn(listener)
}

/**
 * Subscribes to notifications received while the native app is in the
 * foreground. Native code suppresses the operating-system banner and emits
 * this event instead so the webview can show a Helpin-styled, tappable toast.
 * The payload includes `title` and `body` alongside the same routing data as
 * {@link onPushTapped}.
 */
export async function onPushReceived(cb: (data: PushReceivedPayload) => void): Promise<UnlistenFn> {
  const listener = await addPluginListener<PushReceivedPayload>(
    'helpin-push',
    'push-received',
    cb,
  )
  return toUnlistenFn(listener)
}

/**
 * Subscribes to notification taps — fires with the FCM/APNs data payload
 * both for a cold start (app launched from a terminated state by tapping a
 * notification) and a warm tap (app already running, backgrounded or
 * foregrounded).
 *
 * Mechanism (deterministic pull model, no event-timing races):
 * 1. Registers the live `push-tapped` plugin listener first — covers warm
 *    taps from here on.
 * 2. Then invokes `take_pending_tap` exactly once — the native side buffers
 *    every tap payload (cold-start taps land in the buffer before any JS
 *    exists) and this command returns-and-clears it.
 * 3. The native side also `trigger()`s warm taps live (belt and braces), so
 *    the same payload can arrive through both paths within the same
 *    registration tick — identical payloads (JSON equality) within
 *    {@link TAP_DEDUPE_WINDOW_MS} are delivered once.
 *
 * SPIKE-VERIFY: the remaining device question is whether the tap payload is
 * actually present where the native side reads it on each platform — the
 * Android launch intent extras (cold start) / `onNewIntent` extras (warm),
 * and iOS `UNUserNotificationCenterDelegate.didReceive response.userInfo`
 * including for terminated-state launches. The buffering/drain mechanism
 * itself is deterministic and needs no timing verification.
 *
 * @since 0.1.0 (pre-spike, unreleased)
 */
export async function onPushTapped(cb: (data: PushTapPayload) => void): Promise<UnlistenFn> {
  let lastDelivered: { key: string; at: number } | null = null
  const deliver = (data: PushTapPayload) => {
    const key = JSON.stringify(Object.entries(data).sort())
    const now = Date.now()
    if (lastDelivered && lastDelivered.key === key && now - lastDelivered.at < TAP_DEDUPE_WINDOW_MS) {
      return
    }
    lastDelivered = { key, at: now }
    cb(data)
  }

  // Listener first, drain second: a tap arriving during this await is
  // caught either live (listener already registered) or by the drain
  // (still buffered) — never lost, at most deduped.
  const listener = await addPluginListener<PushTapPayload>('helpin-push', 'push-tapped', deliver)
  const pending = await invoke<TakePendingTapResponse>('plugin:helpin-push|take_pending_tap')
  if (pending?.tap) {
    deliver(pending.tap)
  }
  return toUnlistenFn(listener)
}
