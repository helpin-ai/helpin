# Back up and restore the same release

This guide is for operators. It explains what to back up and how to restore the
same 0.1 release onto a fresh host.

Schedule backups and practice restoring them on an isolated host. Quiesce writers
before taking a consistent snapshot. Back up `.env`, `apps.json`, the exact bundle
and image digests, all four databases, Garage metadata/object data, and the Redis/NATS volumes.
Do not print `.env` or embed it in a public support archive. Use restrictive file
permissions and encrypt backups off-host.

Run these operator commands from the bundle’s `community/` directory. Acceptance
test scripts are source-only and are not a backup tool for an existing installation:

```sh
umask 077
mkdir -p backups
# Stop application writers first; keep infrastructure available for database dumps.
docker compose stop helpin-frontend helpin-helpcenter helpin-worker agent-runtime-worker agent-runtime helpin-api
for database in helpin agent_runtime temporal temporal_visibility; do
  docker compose exec -T postgres pg_dump -U postgres -Fc "$database" > "backups/$database.dump"
done
cp .env apps.json backups/
# Stop infrastructure before filesystem-level volume snapshots.
docker compose stop
```

Use your host's volume backup tooling to snapshot `postgres_data`, `garage_data`,
`redis_data`, and `nats_data` while stopped, then `./setup.sh start`. Record which
method was used; do not mix logical SQL restore and old Postgres volume contents.
The release acceptance tests exercise a cold volume restore, including the whole Postgres
volume. For a logical restore, first restore the same `.env` and start only
Postgres on a fresh volume (`docker compose up -d --wait postgres`); its initializer
creates matching users and databases. Restore each dump before starting writers:

```sh
for database in helpin agent_runtime temporal temporal_visibility; do
  docker compose exec -T postgres pg_restore -U postgres --clean --if-exists \
    --exit-on-error --dbname "$database" < "backups/$database.dump"
done
```

Restore the matching object and queue/cache volumes before starting the full
stack. Keep this logical procedure separate from the tested cold-volume method
and rehearse whichever method your own backup system uses.

A failed schema change is not reversed by changing an image tag. Restore the
matching pre-change database/object backup with its keys. 0.1 verifies clean
installs and same-release restore only; tested cross-version upgrades are planned
for 0.2 (see [known limitations](../../ROADMAP.md)).

