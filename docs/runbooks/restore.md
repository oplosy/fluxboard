# PostgreSQL backup and restore runbook

Manual logical backup and restore for the compose stack or a recovery
environment. The repository does not provision automated backups or PITR (see
`docs/10-INFRA-DEVOPS.md` §6); until it does, this is the only tested recovery
path and it covers the database only.

All commands run from the repo root and work unchanged in PowerShell and Bash.
In Git Bash on Windows, run `export MSYS_NO_PATHCONV=1` first; otherwise Git
Bash rewrites the container paths (`/tmp/...`) into Windows paths.

## Why the dump is copied as a file

`pg_dump -Fc` writes a binary archive. Never pipe it through a text stream
(`Get-Content -Raw | ...`, `>` redirection in Windows PowerShell 5.1, or a
terminal): text encodings and newline translation corrupt it, and the restore
fails or silently drops objects. Dumps are written to a file inside the
container and moved with `docker compose cp`, which copies bytes unchanged.

## 1. Take a backup

```powershell
docker compose -f deploy/docker-compose.yml exec -T postgres pg_dump -U postgres -d fluxboard -Fc -f /tmp/fluxboard.dump
docker compose -f deploy/docker-compose.yml cp postgres:/tmp/fluxboard.dump ./backups/fluxboard.dump
docker compose -f deploy/docker-compose.yml exec -T postgres rm /tmp/fluxboard.dump
```

Check the archive is readable before trusting it (lists the table of contents;
a corrupt file errors out):

```powershell
docker compose -f deploy/docker-compose.yml cp ./backups/fluxboard.dump postgres:/tmp/check.dump
docker compose -f deploy/docker-compose.yml exec -T postgres pg_restore --list /tmp/check.dump
docker compose -f deploy/docker-compose.yml exec -T postgres rm /tmp/check.dump
```

`backups/` is git-ignored; never commit a dump. It contains every tenant's data.

## 2. Restore into a disposable database

Use a database that is not serving application traffic. Do not restore over
the live database as a first validation step. The cluster roles
(`fluxboard_owner`, `fluxboard_app`, `fluxboard_admin_ro`) must already exist
(`deploy/postgres/init/01-roles.sql`); the dump carries ownership, grants and
RLS policies but not roles.

```powershell
docker compose -f deploy/docker-compose.yml cp ./backups/fluxboard.dump postgres:/tmp/fluxboard.dump
docker compose -f deploy/docker-compose.yml exec -T postgres createdb -U postgres -O fluxboard_owner fluxboard_restore
docker compose -f deploy/docker-compose.yml exec -T postgres pg_restore -U postgres -d fluxboard_restore --exit-on-error /tmp/fluxboard.dump
docker compose -f deploy/docker-compose.yml exec -T postgres rm /tmp/fluxboard.dump
```

## 3. Verify before switching traffic

Migration history is present and not dirty:

```powershell
docker compose -f deploy/docker-compose.yml exec -T postgres psql -U postgres -d fluxboard_restore -c "SELECT version, dirty FROM schema_migrations;"
```

Representative rows came back (compare with the source):

```powershell
docker compose -f deploy/docker-compose.yml exec -T postgres psql -U postgres -d fluxboard_restore -c "SELECT count(*) FROM organizations;"
```

RLS still isolates tenants. As the app role and without a tenant set, a
tenant-scoped table must return **0** rows even though it has data; with
`app.current_tenant` set it returns only that org's rows. A superuser query
alone does not validate tenant isolation.

```powershell
docker compose -f deploy/docker-compose.yml exec -T postgres psql -U fluxboard_app -d fluxboard_restore -c "SELECT count(*) FROM projects;"
docker compose -f deploy/docker-compose.yml exec -T postgres psql -U fluxboard_app -d fluxboard_restore -c "BEGIN; SELECT set_config('app.current_tenant', '<org-uuid>', true); SELECT count(*) FROM projects; COMMIT;"
```

The restored database must be at the migration version the application
expects before the application is started against it.

## 4. What this does not cover

- **Attachments, avatars, logos** live in MinIO/S3, not in Postgres. Restoring
  the database without the matching objects leaves rows pointing at missing
  files. Mirror the bucket separately (for example
  `mc mirror local/fluxboard <backup-target>`) and restore it alongside.
- **Point-in-time recovery.** A nightly dump loses everything since the dump.
  The production target (managed Postgres with PITR, off-host backup storage,
  measured RPO/RTO) is an open decision, not something this runbook provides.
- Record the date, dump size, restore duration and verification results each
  time this procedure is exercised; an unexercised backup is not a DR control.
