# Back up and restore Helpin

This guide explains how operators can snapshot a Community installation, restore
its data and encryption keys into a separate installation, and recover from a
failed upgrade. Use a CLI containing the `backup`, `restore`, and `upgrade`
commands; availability in source does not mean a public release has shipped.

## Create a backup

From any directory, specify the installed release bundle and a **new** backup
directory outside that installation:

```sh
helpin backup --dir /srv/helpin --backup /srv/helpin-backup
```

The command asks before stopping services. Add `--yes` for unattended execution.
Omitting `--backup` creates a timestamped sibling of the installation directory.
The CLI checks volume ownership, stops services with a 180-second grace period,
refuses a forced shutdown or remaining volume writer, and snapshots every named
volume. This includes PostgreSQL (all four databases), Garage metadata and
objects, Redis, NATS, and agent execution workspaces. It also saves the installed
bundle, `.env`, `apps.json`, Caddy configuration, and exact image references.

Backup resumes the services that were previously running and waits for their
health checks. Previously stopped services stay stopped. A failed snapshot also
attempts to resume those services and never produces a complete backup manifest.
A successful snapshot followed by a restart failure keeps the backup and reports
the restart error. Inspect `helpin status` and `helpin logs` before retrying start.

The backup is a directory containing `backup.json`, `installation.tar.gz`, and
one compressed archive per volume. The manifest records the release, Docker
architecture, source project, and file checksums. Files are private to the
operator (directory mode 700, files 600). **Backups contain unencrypted private
data and encryption keys.** Copy the complete directory to encrypted off-host
storage and manage retention there. The CLI does not schedule, encrypt, or prune
backups. A checksum detects corruption; restore only backups from a trusted source.

## Restore into a new installation

Choose a directory that does not exist. Restore never overwrites an existing
installation or reuses existing Docker volumes:

```sh
helpin restore --backup /srv/helpin-backup --dir /srv/helpin-restored --no-start
helpin configure --dir /srv/helpin-restored
helpin start --dir /srv/helpin-restored
helpin doctor --dir /srv/helpin-restored
```

Restore verifies every archive and its paths before creating volumes. It restores
the original release and settings, preserves encryption keys, and assigns a new
Compose project name based on the destination directory. The original volumes
remain intact. Without `--no-start`, it also starts the restored services, checks
API readiness, and validates the Helpin migration ledger. Add `--yes` to skip the
confirmation prompt when supplying explicit arguments.

On the same host, stop the source installation first or use `configure` to choose
different ports before starting the restored copy. On a replacement host, copy
the backup directory first and configure DNS and the HTTPS proxy separately.
Verify restored support messages, private attachments, and an agent run using an
existing AI connection before routing traffic to the restored installation.

A restore that fails while importing volumes removes only the new volumes it
created. An installation whose data was restored but whose startup failed is
kept for diagnosis and `helpin start` retry. Do not remove the backup until a
restore has been verified.

## Recover from an upgrade failure

`helpin upgrade` creates a complete backup before replacing the bundle or running
new migrations. A migration or readiness failure stops the target services and
prints the backup path and a restore command. Recover into a new directory:

```sh
helpin restore --backup /srv/pre-upgrade-backup --dir /srv/helpin-recovered --yes --no-start
helpin start --dir /srv/helpin-recovered
helpin doctor --dir /srv/helpin-recovered
```

Confirm the failed installation is stopped before starting the restored one on
the same ports. The restored installation uses the old release **and its matching
pre-upgrade data and keys**. Changing an image tag cannot undo a schema migration.
The CLI does not automatically run old binaries against migrated data or delete
the failed installation's volumes. Changes made after the snapshot are not part
of recovery.

## Supported backup layout

These commands support the bundled local Docker volumes and immutable release
images. Restore requires the same Docker architecture and access to the exact
images; backups contain image references, not copies of container images.
An immutable Alpine helper supplies `tar` and may need to be downloaded before
the snapshot starts. Helpers have no network access or host filesystem mounts.

External volumes, volume driver options, writable bind mounts, bind mounts outside
the installation, anonymous volumes, archive hard links, and special files are
rejected. Ordinary files, directories, and relative symlinks contained within a
volume are supported. Installation files must be ordinary files/directories and
total at most 128 MiB; each expanded volume archive is limited to 1 TiB. ACLs and
extended attributes are not covered. Use storage-specific procedures for custom
layouts, external databases, tablespaces, or larger volumes.

If a CLI is interrupted, inspect service state and the sibling installation lock
before retrying. A directory without `backup.json` is incomplete. Never delete a
lock while its process is running. Keep backups outside the installation tree.

## Manual database dumps

For a custom backup system, stop application writers before taking logical dumps
and preserve the matching environment, object storage, queues, and workspaces.
Run from the bundle's `community/` directory:

```sh
umask 077
mkdir -p /srv/helpin-manual-backup
docker compose stop helpin-frontend helpin-helpcenter helpin-worker agent-runtime-worker agent-runtime helpin-api
for database in helpin agent_runtime temporal temporal_visibility; do
  docker compose exec -T postgres pg_dump -U postgres -Fc "$database" > "/srv/helpin-manual-backup/$database.dump"
done
cp .env apps.json /srv/helpin-manual-backup/
docker compose stop
```

Snapshot the named volumes while stopped, then restart with `./setup.sh start`.
Do not mix logical database restore with an old PostgreSQL volume snapshot.
Manual dumps cannot be passed to `helpin restore`; that command requires its own
complete cold snapshot format. Rehearse the restore procedure for your backup
system on an isolated installation.
