# Notification System: Background Job Queue Comparison

**Date:** 2026-03-08
**Context:** Evaluating job queue libraries for a notification system in the Helpin project (Go + GORM + PostgreSQL/Neon + Temporal).

---

## Executive Summary

**Recommendation: River (riverqueue)** -- a PostgreSQL-native job queue that integrates with GORM, requires no new infrastructure, and provides all the primitives needed for a notification system (delayed jobs, retries, priorities, cron, batching).

Temporal is already in the project but is used for complex agent run workflows. Using it for lightweight notification delivery would be architectural overkill and would couple notification throughput to the Temporal cluster.

---

## Comparison Matrix

| Feature | River | Watermill | Asynq | Temporal (existing) | Custom pgq |
|---------|-------|-----------|-------|---------------------|------------|
| **Backing store** | PostgreSQL | PostgreSQL (or Kafka, Redis, etc.) | Redis (required) | Temporal Server (+ its own DB) | PostgreSQL |
| **New infra needed** | None | None (with SQL backend) | Redis | None (already deployed) | None |
| **Go generics / type safety** | Yes (Go 1.18+) | No ([]byte messages) | No ([]byte payloads) | Yes (typed activities) | Manual |
| **GORM integration** | Official docs, shared transactions | v4 supports pgx/ORMs | N/A | N/A | Manual |
| **Delayed/scheduled jobs** | Yes (ScheduledAt) | Yes (delayed requeuer) | Yes (ProcessAt) | Yes (timers, sleep) | Manual |
| **Retries with backoff** | Yes (configurable, default exponential) | Yes (middleware) | Yes (configurable) | Yes (built-in) | Manual |
| **Priority queues** | Yes (multiple named queues) | Manual (separate topics) | Yes (weighted queues) | Yes (task queue routing) | Manual |
| **Cron/periodic jobs** | Yes (built-in cron syntax) | No built-in (needs external) | Yes (cron scheduler) | Yes (cron schedules) | Manual |
| **Batch insertion** | Yes (COPY FROM, very fast) | No | No | No | Manual |
| **Unique/deduplicated jobs** | Yes (built-in) | No | Yes (unique tasks) | Via workflow IDs | Manual |
| **Transactional enqueue** | Yes (same Postgres tx) | Yes (SQL backend) | No (Redis) | No | Yes |
| **Web UI** | River UI (self-hosted) | No | AsynqMon | Temporal Web UI | No |
| **GitHub stars** | ~4,800+ | ~7,500+ | ~10,000+ | ~12,000+ (SDK) | Varies |
| **Maturity** | v0.x but production-ready, active | v1.5, stable, 4+ years | v0.x, stable but slow releases | Production-grade | N/A |
| **License** | MIT (Pro for advanced features) | MIT | MIT | MIT | N/A |

---

## Detailed Analysis

### 1. River (riverqueue/river)

**Architecture:** PostgreSQL-native job queue. Jobs are rows in a Postgres table. Workers poll using `SELECT ... FOR UPDATE SKIP LOCKED` (with `LISTEN/NOTIFY` for instant pickup when using pgx directly). Leverages Postgres transactions for atomic enqueue-with-business-data.

**Strengths for notifications:**
- **Zero new infrastructure.** Uses the same Neon PostgreSQL database the app already runs on.
- **Transactional safety.** Enqueue a notification in the same transaction that creates the event (e.g., story assignment). If the transaction rolls back, the notification never fires. This eliminates ghost notifications.
- **GORM integration.** Official documentation for sharing `*sql.DB` and transactions with GORM. The project already uses GORM + pgx under the hood.
- **Batch insertion.** Uses PostgreSQL `COPY FROM` for bulk inserts -- ideal for fan-out (e.g., notify all 50 team members of an announcement).
- **Built-in periodic/cron jobs.** Perfect for digest emails ("daily summary at 9am") without a separate cron system.
- **Unique jobs.** Prevent duplicate notifications (e.g., don't send the same mention notification twice).
- **Type-safe generics.** Job args are strongly typed Go structs, not raw JSON blobs.

**Weaknesses:**
- Still v0.x (though actively maintained by Brandur Leach, ex-Stripe, and Blake Gentry).
- Advanced features (global concurrency, sequences, `database/sql` driver) are in River Pro (paid).
- GORM integration uses `riverdatabasesql` driver which runs in "poll only mode" (no `LISTEN/NOTIFY`). For maximum performance, a separate pgx-based River client should handle job working while GORM handles insertion. This is a minor architectural consideration, not a blocker.

**Integration effort:** Low. Add `riverqueue/river` + `riverdatabasesql` to `go.mod`. Share the existing `*sql.DB` from GORM. Define job structs. Register workers. Start the River client in `main.go` alongside the existing server.

---

### 2. Watermill (ThreeDotsLabs/watermill)

**Architecture:** Event-driven pub/sub abstraction layer. Messages flow through a Router with middleware. The SQL backend stores messages in a database table, with a subscriber polling for new records. Supports 12+ backends (Kafka, Redis, NATS, SQL, etc.).

**Strengths:**
- Extremely flexible -- can start with PostgreSQL and migrate to Kafka/NATS later.
- FanOut component for broadcasting messages to multiple subscribers.
- Middleware system (retries, throttling, poison queue, correlation IDs).
- Mature ecosystem (7.5k+ stars, 55+ contributors, 4+ years).
- SQL backend works with PostgreSQL.

**Weaknesses for this project:**
- **Pub/sub, not a job queue.** Watermill is designed for event streaming, not task processing. It lacks built-in support for: priority queues, cron/periodic jobs, unique/deduplicated messages, job-level retry configuration. These would need to be built on top.
- **No GORM integration out of the box.** While v4 added pgx adapter support, it's designed around its own SQL schema, not GORM models.
- **Delayed messages require extra setup.** Needs `NewDelayedPostgreSQLPublisher` and a separate requeuer process.
- **Overkill abstraction for a single-backend system.** The project only uses PostgreSQL -- Watermill's multi-backend abstraction adds complexity without benefit.
- **No built-in web UI** for monitoring job/message status.

**Integration effort:** Medium. Requires setting up Watermill's Router, SQL publisher/subscriber, and middleware chain. The pub/sub paradigm is a different mental model from the handler/service/repository pattern used in the project.

---

### 3. Asynq (hibiken/asynq)

**Architecture:** Redis-backed distributed task queue. Tasks are serialized as JSON, pushed to Redis lists, and consumed by worker processes. Supports multiple queues with weighted priorities.

**Strengths:**
- Simple, intuitive API for task definition and processing.
- Built-in cron scheduler, retry with backoff, priority queues, unique tasks.
- AsynqMon web UI for monitoring.
- ~10k GitHub stars, widely adopted.

**Weaknesses for this project:**
- **Requires Redis.** The project currently uses only PostgreSQL (Neon). Adding Redis means:
  - New infrastructure to provision, monitor, and pay for.
  - New failure mode (Redis goes down = no notifications).
  - Data split across two stores (business data in Postgres, jobs in Redis).
  - No transactional enqueue (can't atomically commit business data + notification in one transaction).
- **Still v0.x** with signs of reduced maintainer activity in 2024-2025.
- **Redis Cluster compatibility issues** with some Lua scripts.
- **No GORM integration** (different database entirely).

**Integration effort:** High (infrastructure). Requires provisioning Redis, adding Redis client to the Go app, and managing a separate data store.

---

### 4. Temporal (already in project)

**Architecture:** Durable workflow engine with separate Temporal Server, persistence layer, and worker processes. Workflows are long-running, fault-tolerant state machines. Activities are individual units of work.

**Current usage in the project:** Agent run orchestration -- complex, long-running workflows that execute AI agents, manage git operations, and coordinate multi-step processes. The Temporal setup includes a dedicated `temporal-worker` binary and custom task queues.

**Could Temporal handle notifications?**

Technically yes. You could define a `SendNotificationWorkflow` with activities for each delivery channel. Temporal handles retries, scheduling, and visibility natively.

**Why Temporal is NOT recommended for notifications:**

- **Architectural overkill.** Temporal workflows are designed for long-running, stateful orchestration (minutes to days). Notifications are short-lived fire-and-forget tasks (milliseconds to seconds). Using Temporal for "send an email" is like using Kubernetes to run a shell script.
- **Resource overhead.** Each workflow execution creates history events in Temporal's persistence store. High-volume notifications (hundreds per minute) would generate enormous workflow history, increasing storage costs and Temporal Server load.
- **Coupling risk.** If the Temporal cluster has issues (maintenance, scaling, outage), both agent runs AND notifications stop. Separate concerns = separate failure domains.
- **Latency.** Temporal's task dispatch has inherent latency from its consistency model. For real-time notifications (WebSocket push, in-app alerts), this adds unnecessary delay.
- **Operational complexity.** The existing Temporal setup is purpose-built for agent runs. Adding notification workers to the same cluster requires careful task queue management and worker scaling.

**When Temporal IS appropriate for notifications:** Complex notification workflows like multi-step onboarding sequences, escalation chains with human-in-the-loop approval, or saga-pattern notifications that need compensation logic. For these, create a dedicated notification task queue on the existing Temporal cluster.

---

### 5. Custom PostgreSQL Queue (pgq / DIY)

**Architecture:** A `notifications_queue` table with `SELECT ... FOR UPDATE SKIP LOCKED` for concurrent consumption. Workers poll the table, process jobs, and mark them complete/failed.

**Strengths:**
- Zero dependencies beyond PostgreSQL.
- Full control over schema, retry logic, and behavior.
- Can share GORM transactions.
- Simple to understand.

**Weaknesses:**
- **Significant engineering effort.** Must implement: polling, locking, retry with backoff, dead-letter queue, scheduled/delayed jobs, cron scheduling, unique job deduplication, batch operations, graceful shutdown, metrics/observability, concurrent worker management. This is essentially rebuilding River.
- **Edge cases are hard.** Zombie job detection, exactly-once delivery, worker crash recovery, connection pool management under load.
- **No ecosystem.** No web UI, no community middleware, no battle-tested edge case handling.
- **Maintenance burden.** Every feature and bug fix is on the team.

**When this makes sense:** If notification requirements are truly minimal (e.g., a single goroutine processing a queue with no retries, no scheduling, no priorities). But once you need any of those features, you're rebuilding River poorly.

---

## Recommendation: River

### Why River is the best fit

1. **No new infrastructure.** Uses the existing Neon PostgreSQL database. No Redis, no message broker, no new services to operate.

2. **Transactional safety.** Enqueue notifications atomically with business operations via GORM shared transactions. Example: when a story is assigned, insert the assignment AND the notification job in one transaction.

3. **All required primitives built-in:**
   - **Fan-out:** Batch insert notification jobs for all recipients using `COPY FROM`.
   - **Per-user delivery:** Each notification is a separate job with user-specific args.
   - **Digest/batching:** Use cron periodic jobs to aggregate and send daily digests.
   - **Retries:** Automatic exponential backoff (configurable per job type).
   - **Priorities:** Separate queues for urgent vs. digest notifications.
   - **Scheduling:** `ScheduledAt` for delayed notifications ("remind in 1 hour").

4. **GORM compatibility.** Official documentation for sharing `*sql.DB` connections and transactions. The project already uses GORM with pgx.

5. **Clean separation from Temporal.** Agent run workflows stay on Temporal. Notification delivery gets its own lightweight queue. Independent scaling, independent failure domains.

6. **Type-safe Go generics.** Job args are typed structs, matching the project's pattern of strongly typed models.

7. **Active development.** Maintained by experienced engineers (ex-Stripe), regular releases, growing community (~4,800 stars and rising).

### Suggested Architecture

```
Event Source (handler/service)
    |
    v
[PostgreSQL Transaction]
    |-- Business data write (GORM)
    |-- River job insert (same tx)
    |
    v
[River Job Queue] (PostgreSQL tables)
    |
    +-- NotificationWorker (in-app via WebSocket hub)
    +-- EmailWorker (via Postmark, existing email client)
    +-- DigestWorker (cron periodic job, batches per user)
    +-- PushWorker (future: mobile push notifications)
```

### Integration Steps (High-Level)

1. Add `riverqueue/river` and `riverdatabasesql` to `server/go.mod`.
2. Run River migrations (creates `river_job`, `river_leader`, `river_queue` tables in Neon).
3. Create a `riverClient` in `cmd/api/main.go`, sharing the existing `*sql.DB` from GORM.
4. Define notification job types as Go structs (e.g., `InAppNotificationArgs`, `EmailNotificationArgs`, `DigestArgs`).
5. Register workers for each job type.
6. Start River client alongside the HTTP server (it runs its own goroutines for polling).
7. In services, enqueue notification jobs within existing GORM transactions.

### Cost of River Pro

The free/open-source version covers all basic needs. River Pro adds: global concurrency limits, job sequences, `database/sql` driver for working (not just inserting). These are nice-to-have but not required for a notification system.

---

## Sources

- [River GitHub Repository](https://github.com/riverqueue/river)
- [River Documentation](https://riverqueue.com/docs)
- [River GORM Integration](https://riverqueue.com/docs/gorm)
- [River Periodic/Cron Jobs](https://riverqueue.com/docs/periodic-jobs)
- [River Job Retries](https://riverqueue.com/docs/job-retries)
- [River Blog - database/sql Support](https://riverqueue.com/blog/database-sql)
- [Watermill GitHub Repository](https://github.com/ThreeDotsLabs/watermill)
- [Watermill SQL Pub/Sub](https://watermill.io/pubsubs/sql/)
- [Watermill Delayed Messages](https://watermill.io/advanced/delayed-messages/)
- [Asynq GitHub Repository](https://github.com/hibiken/asynq)
- [Asynq Tutorial](https://dev.to/lovestaco/supercharging-go-with-asynq-scalable-background-jobs-made-easy-32do)
- [Temporal Email Notification Discussion](https://community.temporal.io/t/best-way-to-send-email-notification/8982)
- [Go Task Queue Comparison](https://medium.com/@geisonfgfg/task-queues-in-go-asynq-vs-machinery-vs-work-powering-background-jobs-in-high-throughput-systems-45066a207aa7)
- [PGQ for Go](https://github.com/dataddo/pgq)
- [River Introduction by Brandur](https://brandur.org/river)
