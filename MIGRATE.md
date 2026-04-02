# Migration Runbook

## Purpose

This repo now has two schema-change paths:

- `AutoMigrate` for normal additive model changes
- `cmd/migrate` for explicit versioned SQL migrations, especially renames, drops, backfills, and contract-changing schema work

Use this file as the operational runbook for the migration runner and ArgoCD hook flow.

## Latest Finding

During live Story -> Task verification, the first hard-cut migration exposed a real dual-schema edge case:

- empty task-era tables had already been created in the environment
- `202604010002_story_to_task_hard_cut.sql` intentionally guarded its renames with `... AND pm_tasks IS NULL`
- because `pm_tasks` already existed, the rename step skipped the live `pm_stories` data
- result: both task-era and story-era tables coexisted until reconciled

The fix is now captured in:

- `server/internal/dbmigrate/sql/202604010003_story_task_reconcile_dual_schema.sql`

That reconciliation migration:

- backfills rows from legacy story-era tables into the task-era tables when both exist
- re-syncs the `pm_task_display_id_seq` sequence
- removes the leftover story-era tables afterward

This means the hard-cut is now resilient to environments where `AutoMigrate` or earlier code paths created task-era tables before the rename migration ran.

## Current Architecture

### Runtime migration runner

- Binary: `server/cmd/migrate`
- Package: `server/internal/dbmigrate`
- Migration source: embedded SQL files in `server/internal/dbmigrate/sql/`
- Manual rollback scripts: `server/internal/dbmigrate/rollback/`
- Version ledger: `schema_migrations`
- Safety: Postgres advisory lock prevents concurrent migration runners

Supported commands:

```bash
cd server
go run ./cmd/migrate up
go run ./cmd/migrate status
```

### API startup behavior

The API still supports GORM startup schema evolution, but it is now gated:

- Env var: `RUN_AUTO_MIGRATE`
- Default: `true`
- Code path: `server/cmd/api/main.go`

Meaning:

- `RUN_AUTO_MIGRATE=true`: startup still runs `db.AutoMigrate(...)`
- `RUN_AUTO_MIGRATE=false`: startup skips `AutoMigrate`

For destructive renames like Story -> Task, `RUN_AUTO_MIGRATE` must be `false` during the cutover release.

### ArgoCD / Kubernetes integration

Migration hook manifests:

- `k8s/stage/server-migrate.yaml`
- `k8s/prod/server-migrate.yaml`

Both are ArgoCD `PreSync` hook Jobs. They run before the new Deployment is applied.

Important:

- Hook delete policy currently includes `HookSucceeded`
- successful Jobs are deleted after completion
- `kubectl get jobs` returning nothing is expected after success
- the hook Jobs are now strict and always run `./migrate up`

## When To Use Which Path

### Use `AutoMigrate` for:

- new tables
- new nullable columns
- safe additive indexes/defaults that GORM can express

### Use `cmd/migrate` SQL for:

- table renames
- column renames
- sequence renames
- data backfills
- JSONB/text rewrites
- enum/check-constraint updates
- contract-changing migrations
- anything that must be applied in a controlled release order

### Use manual rollback SQL for:

- rehearsed emergency rollback scripts for hard-cut migrations
- reverse renames and reverse data rewrites that must not run automatically

Rollback scripts must not live under `server/internal/dbmigrate/sql/`, or the runtime runner will try to apply them as forward migrations.

## Release Flow

### Normal additive release

1. Merge to `main`
2. GitHub workflow builds server image from `server/Dockerfile`
3. ArgoCD sync runs the `PreSync` hook Job
4. API deploys with `RUN_AUTO_MIGRATE=true`

This is acceptable for non-destructive changes.

### Controlled migration release

Use this flow for hard-cut schema changes such as Story -> Task.

1. Add new SQL migration file(s) under `server/internal/dbmigrate/sql/`
2. Build and publish the server image that contains those files and the `./migrate` binary
3. Update manifests to the exact released server tag
4. Set API Deployment env `RUN_AUTO_MIGRATE=false`
5. Let ArgoCD run the `PreSync` hook Job
6. The hook Job runs `./migrate up`
7. Only after hook success should the new API pods roll out

Do not rely on API startup to perform the hard-cut migration.

## Verification

### 1. Verify Git/GitHub release state

```bash
git fetch origin
git log --oneline -n 3 origin/main
git show origin/main:k8s/prod/server.yaml | sed -n '28,40p'
git show origin/main:k8s/prod/server-migrate.yaml | sed -n '28,40p'
git show origin/main:k8s/prod/temporal-worker.yaml | sed -n '30,40p'
```

Check that all server-based manifests point at the same released server image tag.

### 2. Verify ArgoCD hook execution

```bash
argocd app get <app-name>
argocd app get <app-name> --show-operation
argocd app history <app-name>
```

What to look for:

- synced revision is the expected manifest-update commit
- last operation succeeded
- `PreSync` hook completed successfully

### 3. Verify Kubernetes events

Because hook Jobs are deleted after success, inspect events instead:

```bash
kubectl -n helpin get events --sort-by=.lastTimestamp | grep helpin-server-migrate
```

Success pattern:

- `SuccessfulCreate`
- `Pulling image "ghcr.io/helpin-ai/helpin/server:<tag>"`
- `Started container migrate`
- `Completed job/helpin-server-migrate`

### 4. Verify deployed image and env

```bash
kubectl -n helpin get deploy helpin-server -o jsonpath='{.spec.template.spec.containers[0].image}{"\n"}'
kubectl -n helpin get deploy helpin-server -o jsonpath='{.spec.template.spec.containers[0].env[?(@.name=="RUN_AUTO_MIGRATE")].value}{"\n"}'
kubectl -n helpin get deploy temporal-worker -o jsonpath='{.spec.template.spec.containers[0].image}{"\n"}'
```

Interpretation:

- server image and temporal-worker image should match the released server tag
- current non-cutover releases may still show `RUN_AUTO_MIGRATE=true`
- hard-cut releases should show `RUN_AUTO_MIGRATE=false`

### 5. Verify the image contains the runner

```bash
docker run --rm --entrypoint /bin/sh ghcr.io/helpin-ai/helpin/server:<tag> -lc 'test -x ./migrate && echo migrate-present'
```

### 6. Verify migration ledger

Against an environment where the migration has been applied:

```bash
cd server
DATABASE_URL=... go run ./cmd/migrate status
```

Check that the expected version appears as `applied` and is recorded in `schema_migrations`.

### 7. Verify hard-cut schema outcome

For a rename cutover, do not stop at `migrations applied`. Also verify:

```bash
cd server
set -a && . .env
psql "$DATABASE_URL" -P pager=off -c "select version, name, applied_at from schema_migrations order by version;"
psql "$DATABASE_URL" -P pager=off -c "select count(*) as tasks from pm_tasks;"
psql "$DATABASE_URL" -P pager=off -c \"select count(*) as legacy_story_tables from pg_tables where schemaname = 'public' and tablename in ('pm_stories','pm_story_owners','pm_story_followers','pm_story_labels','pm_story_links','pm_story_templates','story_delivery_targets','story_git_links');\"
```

Expected after the reconciliation migration:

- `202604010001`, `202604010002`, and `202604010003` are all `applied`
- `pm_tasks` contains the migrated work-item rows
- `legacy_story_tables = 0`

## Findings Confirmed In Production

These were verified during rollout validation:

- ArgoCD `PreSync` hook Jobs were created and completed successfully
- successful hook Jobs were deleted afterward, so `kubectl get jobs -n helpin` returned nothing
- event history showed `helpin-server-migrate` running on the released image tag
- the prod release workflow updated `server.yaml`, `temporal-worker.yaml`, and `server-migrate.yaml` to the same server image tag
- current prod API is still running with `RUN_AUTO_MIGRATE=true`, which is acceptable for the current non-cutover release

## Before Story -> Task Cutover

These items must be done before the actual rename rollout:

1. Add the Story -> Task SQL migration files under `server/internal/dbmigrate/sql/`
2. Set `RUN_AUTO_MIGRATE=false` on the cutover API Deployment
3. Rehearse the full flow on staging
4. Verify `./migrate status` shows the applied Story -> Task migration version
5. Keep the paired rollback script ready under `server/internal/dbmigrate/rollback/` and validate the forward+reverse round trip before release
6. Verify whether task-era tables already exist before rollout; if they do, ensure the reconciliation migration is included in the release image

## Caveats

- Historical files under `server/migrations/` remain historical artifacts. New controlled migrations should go under `server/internal/dbmigrate/sql/`.
- Rollback scripts are manual runbooks, not runner inputs. Store them under `server/internal/dbmigrate/rollback/` and execute them only in a controlled rollback procedure.
