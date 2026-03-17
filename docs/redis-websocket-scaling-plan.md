# Redis WebSocket Scaling Plan — Multi-Pod Event Broadcasting & Shared State

**Status:** Proposed
**Date:** 2026-03-17
**Scope:** Entire WebSocket system — all modules (Support, PM, CRM, Docs, Notifications, Agents)
**Impact:** Infrastructure, backend websocket package, deployment manifests

---

## 1. Problem Statement

The application runs on Kubernetes with **2+ replicas in production** (`k8s/prod/server.yaml`). The current WebSocket architecture is fundamentally single-pod:

- **`Hub.Broadcast`** only reaches clients connected to the **same pod**
- **`Publisher.Publish`** calls `go hub.Broadcast(event)` **locally** — events never leave the pod
- **Presence state** (`PresenceState`, `onlineVisitors`) is **in-memory per pod** — pod 2 has no idea what pod 1 knows
- **Client registry** (`clients` map) is pod-local — a broadcast on pod 1 can't reach clients on pod 2

### What breaks today

| Scenario | Expected | Actual |
|----------|----------|--------|
| Agent A (pod-1) sends message, Agent B (pod-2) has inbox open | Agent B sees new message in real-time | Agent B sees nothing until manual refresh |
| Customer connects widget (pod-1), Agent opens inbox (pod-2) | Green online dot shows | No dot — visitor tracked on pod-1 only |
| Agent A views conversation (pod-1), Agent B checks list (pod-2) | Agent B sees Agent A's avatar | No avatar — presence is pod-1 local |
| PM story updated (pod-1), Agent has board open (pod-2) | Board refreshes | Stale until manual refresh |

### Existing partial solution

`pglistener.go` implements PostgreSQL LISTEN/NOTIFY as a cross-pod bridge. However:

- **Nobody writes to PG NOTIFY from the API server** — `Publisher.Publish` bypasses it entirely
- **It was designed for Temporal worker → API server** event bridging only
- **It cannot carry presence state** — NOTIFY is fire-and-forget events, not shared state
- **PG NOTIFY has a 8000-byte payload limit** — fine for events, but limiting

---

## 2. Recommended Architecture: Redis Pub/Sub + Shared State

### Why Redis over extending PG NOTIFY

| Concern | PG NOTIFY | Redis |
|---------|-----------|-------|
| Cross-pod events | Yes (8KB payload limit) | Yes (512MB limit) |
| Shared state (presence, online) | No — events only | Yes — Hashes, Sets, Sorted Sets |
| Auto-expiry / TTL | No | Yes — key expiry for crash cleanup |
| Throughput | Shares DB connection pool | Dedicated, purpose-built |
| Existing infra | Already have PostgreSQL | Need to add (but standard K8s sidecar) |
| Temporal worker support | Worker needs DB connection | Worker needs Redis URL |

Redis gives us **both** cross-pod broadcasting AND shared presence state in one dependency.

---

## 3. Target Architecture

```
┌─────────────┐     ┌─────────────┐     ┌─────────────┐
│   Pod 1     │     │   Pod 2     │     │   Pod 3     │
│             │     │             │     │             │
│  Hub (local)│     │  Hub (local)│     │  Hub (local)│
│  ├ clients  │     │  ├ clients  │     │  ├ clients  │
│  └ broadcast│     │  └ broadcast│     │  └ broadcast│
│       │     │     │       │     │     │       │     │
│  RedisRelay │     │  RedisRelay │     │  RedisRelay │
│   ├ publish │     │   ├ publish │     │   ├ publish │
│   └ subscribe│    │   └ subscribe│    │   └ subscribe│
└──────┬──────┘     └──────┬──────┘     └──────┬──────┘
       │                   │                   │
       └───────────┬───────┴───────────────────┘
                   │
            ┌──────┴──────┐
            │    Redis    │
            │             │
            │  Pub/Sub    │  ← cross-pod event fan-out
            │  Hashes     │  ← presence state (viewing, typing)
            │  Sets       │  ← online visitors
            │  Key TTL    │  ← auto-cleanup on crash
            └─────────────┘
```

### Core principle

**Hub stays local.** Each pod's Hub manages its own WebSocket connections and local broadcast. Redis sits between pods as the coordination layer.

---

## 4. Component Design

### 4.1 Redis Relay (`server/internal/websocket/redis_relay.go`)

New file. Replaces the role of `pglistener.go` with a bidirectional Redis bridge. Manages dynamic per-workspace channel subscriptions with ref-counting.

```go
type RedisRelay struct {
    rdb       *redis.Client
    hub       *Hub
    podID     string
    pubsub    *redis.PubSub           // managed subscription handle
    mu        sync.Mutex
    wsRefCount map[string]int          // workspaceID → number of local clients
}

func NewRedisRelay(rdb *redis.Client, hub *Hub, podID string) *RedisRelay

// Start subscribes to the global channel and begins the receive loop.
// Called as a goroutine on startup. Blocks until ctx is cancelled.
func (r *RedisRelay) Start(ctx context.Context)

// Publish sends an event to the appropriate Redis channel.
func (r *RedisRelay) Publish(ctx context.Context, event Event) error {
    channel := "ws:events:global"
    if event.WorkspaceID != "" {
        channel = "ws:events:" + event.WorkspaceID
    }
    envelope, _ := json.Marshal(relayEnvelope{OriginPod: r.podID, Event: event})
    return r.rdb.Publish(ctx, channel, envelope).Err()
}

// EnsureWorkspaceSubscription subscribes to a workspace channel when the
// first local client for that workspace connects. Thread-safe, ref-counted.
func (r *RedisRelay) EnsureWorkspaceSubscription(workspaceID string) {
    r.mu.Lock()
    defer r.mu.Unlock()
    r.wsRefCount[workspaceID]++
    if r.wsRefCount[workspaceID] == 1 {
        // First client for this workspace on this pod — subscribe
        r.pubsub.Subscribe(context.Background(), "ws:events:"+workspaceID)
    }
}

// ReleaseWorkspaceSubscription unsubscribes from a workspace channel when
// the last local client for that workspace disconnects. Thread-safe.
func (r *RedisRelay) ReleaseWorkspaceSubscription(workspaceID string) {
    r.mu.Lock()
    defer r.mu.Unlock()
    r.wsRefCount[workspaceID]--
    if r.wsRefCount[workspaceID] <= 0 {
        delete(r.wsRefCount, workspaceID)
        r.pubsub.Unsubscribe(context.Background(), "ws:events:"+workspaceID)
    }
}
```

**Channel subscriptions are managed by Hub.Register/Unregister:**
```go
// In Hub.Register:
if h.relay != nil {
    h.relay.EnsureWorkspaceSubscription(c.WorkspaceID)
}

// In Hub.Unregister:
if h.relay != nil {
    h.relay.ReleaseWorkspaceSubscription(c.WorkspaceID)
}
```

**Receive loop** (inside `Start`):
```go
for msg := range r.pubsub.Channel() {
    var env relayEnvelope
    if json.Unmarshal([]byte(msg.Payload), &env) != nil { continue }
    if env.OriginPod == r.podID { continue } // skip self-origin
    r.hub.Broadcast(env.Event) // deliver to local clients only
}
```

**Hub wiring** — Hub gets a relay reference set during DI:
```go
func (h *Hub) SetRelay(relay *RedisRelay) { h.relay = relay }
```

This is called in `main.go` after both Hub and RedisRelay are created.

### 4.2 Redis Presence (`server/internal/websocket/redis_presence.go`)

New file. Replaces in-memory `PresenceState` and `onlineVisitors` with Redis-backed shared state.

```go
type RedisPresence struct {
    rdb   *redis.Client
    podID string
}

func NewRedisPresence(rdb *redis.Client, podID string) *RedisPresence
```

**Viewing presence** — Per-connection keys + aggregate set:

Each connection registers its own key:
```
Key:    support:viewing:conn:{workspaceID}:{conversationID}:{userID}:{connID}
Value:  1
TTL:    60s (refreshed on each viewing:start)
```

Aggregate set for fast lookup:
```
Key:    support:viewing:{workspaceID}:{conversationID}
Type:   Set
Member: {userID}
```

On `SetViewing`: add to set + create conn key. On `ClearViewing`: delete conn key, then check if any other conn keys exist for that user — if not, remove from set. On TTL expiry: a background cleanup goroutine or lazy check removes stale users from the set.

This ensures **one tab cannot clear another tab's viewing state** — each connection owns its own key.

**Typing state** — Per-user key (NOT per-connection):
```
Key:    support:typing:{workspaceID}:{conversationID}:{userID}
Value:  {content}   (draft preview text)
TTL:    15s (refreshed on each keystroke)
```

**Deliberate design decision: typing is single-active-tab per user.** Rationale:
- A user realistically types in one tab at a time. The latest keystroke from any tab is the correct draft preview.
- Last-writer-wins is correct — if tab A is idle and tab B is typing, tab B's content should show.
- On graceful disconnect, only clear the typing key if the disconnecting connection was the **last active typer** for that user. Implementation: store `connID` in the Redis value alongside content (e.g. `{connID}:{content}`), and only `DEL` on disconnect if the stored connID matches the closing connection. If another tab has since overwritten the key, the disconnect is a no-op.

```
Key:    support:typing:{workspaceID}:{conversationID}:{userID}
Value:  {connID}|{content}
TTL:    15s
```

```go
// On disconnect:
val := rdb.Get(ctx, key)
if strings.HasPrefix(val, closingConnID+"|") {
    rdb.Del(ctx, key)  // this connection owns the key
}
// else: another tab has overwritten — leave it alone
```

This avoids the complexity of per-connection typing keys while preventing one tab's close from wiping another tab's active draft.

**Online visitors** — Per-connection keys + aggregate set:

Each widget connection registers:
```
Key:    support:visitors:conn:{workspaceID}:{anonymousID}:{podID}:{connID}
Value:  1
TTL:    90s (refreshed by widget keepalive ping every 60s — see Section 8.4)
```

Aggregate set:
```
Key:    support:visitors:online:{workspaceID}
Type:   Set
Member: {anonymousID}
```

Same pattern as viewing — add to set on connect, remove from set only when the **last** connection key for that visitor expires or is deleted. This handles multiple widget tabs correctly.

**Methods**:
```go
// Viewing
func (p *RedisPresence) SetViewing(ctx, workspaceID, conversationID, userID string) (changed bool, err error)
func (p *RedisPresence) ClearViewing(ctx, workspaceID, conversationID, userID string) (changed bool, err error)
func (p *RedisPresence) GetViewers(ctx, workspaceID, conversationID string) ([]string, error)

// Typing
func (p *RedisPresence) SetTyping(ctx, workspaceID, conversationID, userID, content string) error
func (p *RedisPresence) ClearTyping(ctx, workspaceID, conversationID, userID string) error
func (p *RedisPresence) GetTypers(ctx, workspaceID, conversationID string) (map[string]string, error)

// Online visitors
func (p *RedisPresence) SetVisitorOnline(ctx, workspaceID, anonymousID, connID string) error
func (p *RedisPresence) SetVisitorOffline(ctx, workspaceID, anonymousID, connID string) error
func (p *RedisPresence) IsVisitorOnline(ctx, workspaceID, anonymousID string) (bool, error)
func (p *RedisPresence) GetOnlineVisitors(ctx, workspaceID string) ([]string, error)

// Snapshot (for agent WS connect)
func (p *RedisPresence) GetPresenceSnapshot(ctx, workspaceID, conversationID string) (PresenceSnapshot, error)
```

### 4.3 Updated Publisher (`server/internal/websocket/publisher.go`)

Publisher gains a Redis relay reference. On `Publish`:

1. `go hub.Broadcast(event)` — local pod delivery (unchanged)
2. `go relay.Publish(ctx, event)` — cross-pod delivery via Redis

```go
type Publisher struct {
    hub   *Hub
    relay *RedisRelay // nil-safe, like hub
}

func (p *Publisher) Publish(event Event) {
    if p == nil { return }
    go p.hub.Broadcast(event)
    if p.relay != nil {
        go p.relay.Publish(context.Background(), event)
    }
}
```

### 4.3.1 CRITICAL: Direct Broadcast Callsites Must Also Use Relay

**Problem**: Not all events go through `Publisher.Publish()`. Several callsites bypass the Publisher and call `hub.Broadcast()` directly. These would remain pod-local unless migrated.

**Direct broadcast callsites that MUST be updated:**

| File | What it broadcasts | Current path |
|------|--------------------|--------------|
| `handler.go` lines 124-178 | `viewing_started/stopped`, `typing_started/stopped` (agent WS) | `go h.hub.Broadcast(event)` |
| `widget_handler.go` lines 313-416 | `typing_started/stopped`, `visitor_online/offline` (widget WS) | `go h.hub.Broadcast(event)` |
| `hub.go` lines 115-140 | `viewing_stopped`, `typing_stopped` (disconnect cleanup) | `go h.Broadcast(event)` |

**Fix**: Add a `BroadcastAll(event)` method to Hub that does local broadcast + Redis relay:

```go
func (h *Hub) BroadcastAll(event Event) {
    go h.Broadcast(event)         // local pod
    if h.relay != nil {
        go h.relay.Publish(context.Background(), event)  // other pods
    }
}
```

Then replace all direct `hub.Broadcast` calls with `hub.BroadcastAll`. The Hub needs a relay reference (set during DI wiring).

**Alternatively**: Route all presence events through the Publisher too, but that changes handler signatures. `BroadcastAll` is simpler.

### 4.4 Updated Hub

Hub's `Presence` field changes from in-memory `*PresenceState` to an interface:

```go
type PresenceProvider interface {
    SetViewing(ctx context.Context, workspaceID, conversationID, userID string) (bool, error)
    ClearViewing(ctx context.Context, workspaceID, conversationID, userID string) (bool, error)
    SetTyping(ctx context.Context, workspaceID, conversationID, userID, content string) error
    ClearTyping(ctx context.Context, workspaceID, conversationID, userID string) error
    ClearAllForUser(ctx context.Context, workspaceID, userID string) (viewingCleared, typingCleared []string)
    GetSnapshot(ctx context.Context, workspaceID, conversationID string) (PresenceSnapshot, error)
    SetVisitorOnline(ctx context.Context, workspaceID, anonymousID, connID string) error
    SetVisitorOffline(ctx context.Context, workspaceID, anonymousID, connID string) error
    IsVisitorOnline(ctx context.Context, workspaceID, anonymousID string) (bool, error)
    GetOnlineVisitors(ctx context.Context, workspaceID string) ([]string, error)
}
```

Both `PresenceState` (in-memory, for tests/dev) and `RedisPresence` (production) implement this interface.

### 4.4.1 Connection Identity

Every conn-scoped Redis key depends on a unique `connID`. Add this to the `Client` struct:

```go
type Client struct {
    Conn           *websocket.Conn
    ConnID         string  // unique per connection, e.g. UUID generated on accept
    UserID         string
    WorkspaceID    string
    IsWidget       bool
    ConversationID *string
    AnonymousID    string
}
```

Generated in both `handler.go` (agent) and `widget_handler.go` (widget) at connection accept time:
```go
connID := uuid.NewString() // or use a counter + podID
client := &Client{ConnID: connID, ...}
```

All PresenceProvider methods that need conn-scoping receive `connID` as a parameter.

### 4.4.2 Viewing Heartbeat

**Problem**: The frontend currently sends `support:viewing:start` once on mount and `support:viewing:stop` on cleanup. With a 60s TTL on viewing conn keys, an agent who keeps a thread open will disappear after one minute.

**Fix**: The agent WS handler (`handler.go`) must refresh the viewing conn key server-side. Two options:

**Option A (recommended): Server-side refresh in the read loop.**
The agent handler's read loop already processes `support:viewing:start`. On receiving it, refresh the TTL. But the frontend only sends it once per mount.

So add a **server-side keepalive**: when the handler processes ANY message from the agent (typing, viewing, or even a periodic `ping`), refresh all that agent's active viewing conn keys:

```go
// In handler.go read loop, after processing any message:
if viewingConvID := h.hub.Presence.GetActiveViewing(workspaceID, client.UserID, client.ConnID); viewingConvID != "" {
    h.hub.Presence.RefreshViewing(ctx, workspaceID, viewingConvID, client.UserID, client.ConnID)
}
```

**Option B: Frontend heartbeat.**
Add a 30s interval in `MessageThread.tsx` that re-sends `support:viewing:start`. Simpler but adds client-side complexity we already removed.

**Recommendation**: Option A — server-side refresh. The frontend already sends typing events while the agent is active. For truly idle agents (reading, not typing), add a lightweight `support:ping` client message sent every 45s from the frontend `useWebSocket` hook. The handler refreshes all presence keys on any message receipt.

### 4.5 Operational Modes & Failure Handling

**Development mode** (`REDIS_URL` empty):
- In-memory `PresenceState` used — single-process, no Redis needed
- Local-only Hub broadcast — fine for `go run` and single-pod dev

**Production mode** (`REDIS_URL` set):
- Redis is **required**, not optional. If Redis is unreachable at startup, the server should **fail to start** (crash loop until Redis is available). Silent fallback to local-only in a multi-pod cluster creates split-brain presence and partial event delivery — worse than downtime.
- If Redis becomes unreachable **after startup** (transient failure): log errors, retry with backoff, but do NOT silently switch to local-only mode. Presence operations should return errors that propagate as degraded UX (stale indicators) rather than incorrect UX (split-brain).

**Redis high availability**:
- Production should use Redis Sentinel or a managed Redis service (AWS ElastiCache, Upstash) with automatic failover — NOT a single bare replica.
- Staging can use a single replica since split-brain is acceptable there.

```yaml
# k8s/prod/redis.yaml — use Redis Sentinel or managed service
# Single-replica shown for staging only. Production MUST use HA.
```

---

## 5. Infrastructure Changes

### 5.1 Redis Deployment

Add to Kubernetes manifests:

```yaml
# k8s/prod/redis.yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: redis
spec:
  replicas: 1
  template:
    spec:
      containers:
        - name: redis
          image: redis:7-alpine
          ports:
            - containerPort: 6379
          resources:
            requests:
              memory: "64Mi"
              cpu: "50m"
            limits:
              memory: "128Mi"
              cpu: "100m"
---
apiVersion: v1
kind: Service
metadata:
  name: redis
spec:
  selector:
    app: redis
  ports:
    - port: 6379
```

**Staging**: Single replica is acceptable. **Production**: Use a managed Redis service (AWS ElastiCache, Upstash, or Redis Sentinel) for automatic failover. A single bare replica makes the entire realtime layer a SPOF.

### 5.2 Configuration

Add to `server/internal/config/config.go`:

```go
RedisURL string `env:"REDIS_URL"` // redis://redis:6379/0 — empty = local-only mode
```

Add to `.env.example`:
```
REDIS_URL=redis://localhost:6379/0
```

Add to Doppler secrets for staging/production.

### 5.3 Go Dependency

```bash
go get github.com/redis/go-redis/v9
```

### 5.4 DI Wiring (`cmd/api/main.go`)

```go
podID := os.Getenv("HOSTNAME") // K8s sets this to pod name

if cfg.RedisURL == "" {
    // Development mode — single-process, no Redis
    slog.Info("REDIS_URL not set — running in local-only mode (single pod)")
    presenceProvider = ws.NewPresenceState() // in-memory
    wsPublisher = ws.NewPublisher(wsHub, nil)
} else {
    // Production mode — Redis required, fail fast if unavailable
    opts, err := redis.ParseURL(cfg.RedisURL)
    if err != nil {
        slog.Error("invalid REDIS_URL", "error", err)
        os.Exit(1)
    }
    redisClient := redis.NewClient(opts)
    if err := redisClient.Ping(ctx).Err(); err != nil {
        slog.Error("Redis unreachable at startup — cannot run multi-pod", "error", err)
        os.Exit(1) // crash loop until Redis is available
    }

    redisRelay := ws.NewRedisRelay(redisClient, wsHub, podID)
    go redisRelay.Start(ctx)

    presenceProvider = ws.NewRedisPresence(redisClient, podID)
    wsPublisher = ws.NewPublisher(wsHub, redisRelay)
    slog.Info("Redis connected for WebSocket scaling", "url", cfg.RedisURL, "pod", podID)
}

wsHub.SetPresenceProvider(presenceProvider)
wsHub.SetRelay(redisRelay) // nil in dev mode — BroadcastAll becomes local-only
```

**No ambiguity**: `REDIS_URL` empty = dev mode (in-memory, local-only). `REDIS_URL` set = production mode (fail-to-start if Redis unreachable).

---

## 6. Migration Strategy

### Phase 1: Add Redis infrastructure (no behavior change)
- Add Redis to K8s manifests
- Add `go-redis` dependency
- Add `REDIS_URL` config
- Create `RedisRelay` and `RedisPresence` with interface
- Wire up in `main.go` with fallback to in-memory
- **No behavior change** — everything still works single-pod

### Phase 2: Enable cross-pod event broadcasting
- Publisher sends to both local Hub AND Redis Pub/Sub
- RedisRelay subscribes and forwards to local Hub (skipping self-origin)
- **Result**: All 20 event types (PM, CRM, Support, Docs, Notifications) work cross-pod
- PGListener kept as fallback for Temporal worker (or migrate Temporal to Redis too)

### Phase 3: Migrate presence state to Redis
- Replace in-memory `PresenceState` with `RedisPresence`
- Replace in-memory `onlineVisitors` with Redis Sets
- Key TTLs handle crash cleanup automatically
- **Result**: Viewing presence, typing state, online visitors all shared across pods

### Phase 4: Retire PGListener
- **Temporal worker** (`cmd/temporal-worker/main.go` line 165): currently creates a local `wsPublisher` that cannot reach any Hub. Migrate to publish via Redis directly using a `RedisRelay.Publish()` call (no local Hub needed).
- **Agent run repository** (`service/agent.go` line 237): currently uses `pg_notify('ws_events', ...)` for agent run updates. Replace with `wsPublisher.Publish()` which now goes through Redis.
- **API server** (`cmd/api/main.go` line 338): stop starting `pgListener` goroutine.
- Remove `pglistener.go` entirely.
- Remove PostgreSQL LISTEN/NOTIFY dependency for WS events.

**Files to modify for PGListener retirement:**
| File | Change |
|------|--------|
| `cmd/temporal-worker/main.go` | Initialize `RedisRelay`, publish events via Redis instead of local Publisher |
| `internal/service/agent.go` | Replace `pg_notify()` calls with `wsPublisher.Publish()` |
| `internal/repository/*` | Remove any `pg_notify()` helper calls |
| `cmd/api/main.go` | Remove PGListener init + goroutine |
| `internal/websocket/pglistener.go` | Delete file |

---

## 7. Event Channel Design

### 7.1 Pub/Sub Channels

```
ws:events:{workspaceID}     — workspace-scoped events (PM, CRM, Support, etc.)
ws:events:global             — events without a workspace_id (fallback)
```

Two-tier channel design:
- **Per-workspace channels**: Most events have a `workspace_id`. Pods subscribe to channels for workspaces that have active clients. When the last client for a workspace disconnects, the pod unsubscribes from that channel.
- **Global channel**: Some events lack `workspace_id` (e.g. `crm_buyer_signal` from signal detection workflows). These go to the global channel. All pods subscribe to it.

**Publisher routing logic**:
```go
func (r *RedisRelay) Publish(ctx context.Context, event Event) error {
    channel := "ws:events:global"
    if event.WorkspaceID != "" {
        channel = "ws:events:" + event.WorkspaceID
    }
    return r.rdb.Publish(ctx, channel, payload).Err()
}
```

**PREREQUISITE for Phase 2 — enforce WorkspaceID on all client-visible events:**

The global channel is a fallback, NOT a first-class routing path. `Hub.Broadcast` routes events by `event.WorkspaceID` to find target clients — events with empty `WorkspaceID` have no target client set and are silently dropped.

Before Phase 2 ships, **every event producer must set WorkspaceID**. Specific fixes needed:

| File | Event | Fix |
|------|-------|-----|
| `signal_detection_workflow.go:108` | `crm_buyer_signal:created` | Add WorkspaceID from the signal's workspace context |
| Any other producer found by: `grep -r 'wsPublisher.Publish' --include="*.go" \| grep -v WorkspaceID` | — | Add WorkspaceID |

Events without WorkspaceID should be logged as errors at the Publisher level:
```go
func (p *Publisher) Publish(event Event) {
    if event.WorkspaceID == "" {
        slog.Error("event published without WorkspaceID — will not reach clients",
            "entity", event.Entity, "action", event.Action)
    }
    ...
}
```

The global channel exists only as a safety net for the transition period. Long-term goal: zero events on the global channel.

### 7.2 Message Envelope

```json
{
  "origin_pod": "server-abc123",
  "event": {
    "action": "created",
    "entity": "support_conversation_message",
    "entity_id": "msg-uuid",
    "workspace_id": "ws-uuid",
    "actor_id": "user-uuid",
    "parent_type": "support_conversation",
    "parent_id": "conv-uuid",
    "data": {}
  }
}
```

The `origin_pod` field lets the receiver skip rebroadcasting to the pod that originated the event (it was already broadcast locally).

---

## 8. Disconnect & Cleanup Strategy

### 8.1 Graceful Disconnect (normal close, tab close, navigation away)

On graceful disconnect, the current code already sends immediate cleanup events. This MUST be preserved with Redis:

1. **Agent WS disconnect** (`hub.go` Unregister):
   - Delete conn-level Redis keys for viewing: `DEL support:viewing:conn:{ws}:{conv}:{user}:{conn}`
   - Check if user has other connections — if not, remove from aggregate set + broadcast `viewing_stopped`
   - Delete typing key: `DEL support:typing:{ws}:{conv}:{user}` + broadcast `typing_stopped`
   - All via `hub.BroadcastAll()` so other pods see it immediately

2. **Widget WS disconnect** (`widget_handler.go` defer):
   - Delete conn-level visitor key: `DEL support:visitors:conn:{ws}:{anon}:{pod}:{conn}`
   - Check if visitor has other connections — if not, remove from aggregate set + broadcast `visitor_offline`
   - Immediate, not TTL-dependent

### 8.2 Crash Cleanup (pod dies without graceful shutdown)

TTL is the **safety net**, not the primary mechanism:

| Key Pattern | TTL | Purpose |
|-------------|-----|---------|
| `support:viewing:conn:{ws}:{conv}:{user}:{conn}` | 60s | Safety net if pod crashes |
| `support:typing:{ws}:{conv}:{user}` | 15s | Safety net if pod crashes |
| `support:visitors:conn:{ws}:{anon}:{pod}:{conn}` | 90s | Safety net if pod crashes |

A **background cleanup goroutine** (one per pod, every 30s) scans for expired conn keys and reconciles the aggregate sets:
- For each aggregate set (`support:viewing:{ws}:{conv}`, `support:visitors:online:{ws}`), verify that member entries still have at least one live conn key
- Remove stale members and broadcast stop events
- This handles the gap between "conn key expired" and "aggregate set still has stale member"

### 8.3 Key Refresh

Conn-level keys are refreshed by active operations:
- Viewing conn keys: refreshed on each `support:viewing:start` AND on each `support:ping` (45s interval from frontend). An idle agent viewing a thread sends pings that keep the key alive.
- Typing keys: refreshed on each keystroke (natural 300ms throttle)
- Visitor conn keys: refreshed by the widget keepalive protocol (see Section 8.4)

### 8.4 Widget Keepalive Protocol

**Problem**: The widget handler's read loop (`widget_handler.go:336`) blocks on `conn.Read()`. A quiet-but-open widget session (customer has the page open but isn't typing or clicking) generates zero messages. With a 90s TTL on the visitor conn key, the customer's online status will expire even though their WebSocket is alive.

**Solution**: An application-level keepalive protocol between the widget SDK and the server.

**Widget SDK changes** (`packages/sdk-js/src/core/widget.ts`):
```typescript
// In connectWebSocket(), after session:joined:
// Start keepalive ping every 60s
this.keepaliveTimer = setInterval(() => {
    if (this.wsConnection?.readyState === WebSocket.OPEN) {
        this.wsSend('ping', {});
    }
}, 60_000);

// In disconnectWebSocket():
if (this.keepaliveTimer) {
    clearInterval(this.keepaliveTimer);
    this.keepaliveTimer = null;
}
```

**Server handler changes** (`widget_handler.go`):
Add a `ping` case to the read loop:
```go
case "ping":
    // Refresh visitor online key TTL
    if session.AnonymousID != "" {
        h.hub.Presence.RefreshVisitorOnline(ctx, session.WorkspaceID, session.AnonymousID, client.ConnID)
    }
    SendToClient(conn, "pong", nil)
```

**Agent handler changes** (`handler.go`):
Add a `support:ping` case:
```go
case "support:ping":
    // Refresh all active presence keys for this agent
    h.hub.Presence.RefreshAllForUser(ctx, workspaceID, client.UserID, client.ConnID)
```

Frontend sends `support:ping` every 45s from `useWebSocket`:
```typescript
// In useWebSocket connect():
const pingInterval = setInterval(() => {
    if (ws.readyState === WebSocket.OPEN) {
        ws.send(JSON.stringify({ type: 'support:ping', data: {} }));
    }
}, 45_000);

// In cleanup:
clearInterval(pingInterval);
```

**PresenceProvider additions**:
```go
RefreshVisitorOnline(ctx, workspaceID, anonymousID, connID string) error  // EXPIRE the conn key
RefreshAllForUser(ctx, workspaceID, userID, connID string) error          // EXPIRE all viewing/typing keys
```

**Files to modify**:
| File | Change |
|------|--------|
| `packages/sdk-js/src/core/widget.ts` | Add 60s keepalive ping interval |
| `server/internal/websocket/widget_handler.go` | Handle `ping` message, refresh visitor key |
| `server/internal/websocket/handler.go` | Handle `support:ping`, refresh presence keys |
| `frontend/src/hooks/useWebSocket.ts` | Add 45s ping interval |
| `server/internal/websocket/redis_presence.go` | Add `RefreshVisitorOnline`, `RefreshAllForUser` |

---

## 9. Testing Strategy

### Unit Tests
- `redis_relay_test.go` — publish/subscribe with mock Redis (use miniredis)
- `redis_presence_test.go` — all presence operations with miniredis
- Existing `presence_test.go` stays as in-memory implementation tests
- Existing `hub_test.go` stays unchanged (tests local broadcast logic)

### Integration Tests
- Two Hub instances connected via Redis relay
- Event published on Hub-1 arrives at Hub-2
- Presence set on Hub-1 visible from Hub-2
- Pod disconnect clears Redis state via TTL

### Load Tests
- Verify Redis Pub/Sub handles 1000+ events/second
- Verify presence operations don't bottleneck under high typing activity

---

## 10. Rollout Plan

| Phase | Scope | Risk | Rollback |
|-------|-------|------|----------|
| 1. Infrastructure | Add Redis, no behavior change | Zero — fallback to in-memory | Remove REDIS_URL |
| 2. Event broadcasting | Cross-pod events via Redis | Low — local broadcast still works | Disable relay |
| 3. Presence migration | Shared state in Redis | Medium — presence accuracy | Switch back to in-memory provider |
| 4. Retire PGListener | Remove PG NOTIFY path | Low — Redis proven by now | Re-enable PGListener |

Each phase is independently deployable and reversible.

---

## 11. Files to Create/Modify

### New Files
| File | Purpose |
|------|---------|
| `server/internal/websocket/redis_relay.go` | Redis Pub/Sub bridge |
| `server/internal/websocket/redis_relay_test.go` | Relay tests with miniredis |
| `server/internal/websocket/redis_presence.go` | Redis-backed presence state |
| `server/internal/websocket/redis_presence_test.go` | Presence tests with miniredis |
| `server/internal/websocket/presence_provider.go` | Interface definition |
| `k8s/prod/redis.yaml` | Redis K8s deployment |
| `k8s/stage/redis.yaml` | Redis K8s deployment (staging) |

### Modified Files
| File | Change |
|------|--------|
| `server/internal/config/config.go` | Add `RedisURL` field |
| `server/internal/websocket/publisher.go` | Add relay reference, dual publish |
| `server/internal/websocket/hub.go` | Presence as interface, remove in-memory presence |
| `server/internal/websocket/handler.go` | Use presence interface for snapshot |
| `server/internal/websocket/widget_handler.go` | Use presence interface for visitor tracking |
| `server/cmd/api/main.go` | Redis client init, DI wiring |
| `server/go.mod` | Add `github.com/redis/go-redis/v9` |
| `.env.example` | Add `REDIS_URL` |

---

## 12. Estimated Effort

| Phase | Effort |
|-------|--------|
| Phase 1: Infrastructure | 1 day |
| Phase 2: Event broadcasting | 1-2 days |
| Phase 3: Presence migration | 2-3 days |
| Phase 4: Retire PGListener | 0.5 day |
| Testing & hardening | 1-2 days |
| **Total** | **5-8 days** |

---

## 13. Decision: Redis vs Extending PG NOTIFY

The codebase already has `pglistener.go` using PostgreSQL LISTEN/NOTIFY. Why not just extend it?

| Factor | PG NOTIFY | Redis |
|--------|-----------|-------|
| Event broadcasting | Yes (8KB payload limit) | Yes (512MB limit) |
| Shared state | ❌ Not possible | ✅ Hashes, Sets, Sorted Sets |
| Auto-expiry | ❌ Manual cleanup | ✅ Key TTL |
| Connection overhead | Shares DB pool | Dedicated connection |
| Operational complexity | Already deployed | New dependency |
| Scalability ceiling | ~10K events/sec (shares DB) | ~100K+ events/sec (dedicated) |

**Verdict**: Redis is required because PG NOTIFY cannot provide shared presence state. Since we need Redis anyway for presence, using it for Pub/Sub too simplifies the architecture.
