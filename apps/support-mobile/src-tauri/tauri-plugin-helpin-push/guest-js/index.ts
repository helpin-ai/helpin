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

interface GetPushTokenResponse {
  token: string | null
}

interface TokenChangedEvent {
  token: string
}

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
 * Subscribes to notification taps — fires with the FCM/APNs data payload
 * both for a cold start (app launched from a terminated state by tapping a
 * notification) and a warm tap (app already running, backgrounded or
 * foregrounded).
 *
 * SPIKE-VERIFY: the native side must buffer the cold-start payload until
 * this listener is registered (the webview + JS runtime aren't ready at
 * process launch) and flush it once registration happens — confirm no
 * race where the buffered event fires before `onPushTapped` has been
 * awaited by the app's startup code.
 *
 * @since 0.1.0 (pre-spike, unreleased)
 */
export async function onPushTapped(cb: (data: PushTapPayload) => void): Promise<UnlistenFn> {
  const listener = await addPluginListener<PushTapPayload>('helpin-push', 'push-tapped', cb)
  return toUnlistenFn(listener)
}
