# PostgreSQL backup and restore runbook

Manual logical backup and restore for the compose stack or a recovery
environment (§1–§4). Production on AWS (`deploy/aws`, ADR-028) adds RDS
point-in-time recovery, S3 versioning and AWS Backup copies; §5 covers those.
Only §1–§4 have been exercised so far.

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
- **Point-in-time recovery.** A dump loses everything written after it. On
  AWS, use §5 instead.
- Record the date, dump size, restore duration and verification results each
  time this procedure is exercised; an unexercised backup is not a DR control.

## 5. Production (AWS): point-in-time recovery

Targets (report B07, to be confirmed against cost and customer needs):
database **RPO ≤ 15 min** and service **RTO ≤ 4 h**. RDS ships WAL about every
5 minutes, so the latest restorable time trails real time by about that much.
Not yet exercised: run this on a staging copy, time it, and record the result
below before quoting these numbers.

1. **Pick the moment** just before the incident, in UTC, and write it down. It
   is the recovery manifest's anchor for the database and the objects.
   `aws rds describe-db-instances --db-instance-identifier <db_instance_id>`
   shows `LatestRestorableTime`.
2. **Restore into a new instance.** PITR never overwrites the source:

   ```bash
   aws rds restore-db-instance-to-point-in-time \
     --source-db-instance-identifier <db_instance_id> \
     --target-db-instance-identifier <db_instance_id>-pitr-<yyyymmddhhmm> \
     --restore-time <UTC timestamp, e.g. 2026-10-12T09:41:00Z> \
     --db-subnet-group-name fluxboard-<env>-db \
     --vpc-security-group-ids <db security group id> \
     --db-parameter-group-name fluxboard-<env>-pg16 \
     --multi-az --no-publicly-accessible
   aws rds wait db-instance-available --db-instance-identifier <new id>
   ```

3. **Verify** against the new endpoint, as in §3:
   - row counts;
   - as `fluxboard_app` with no tenant set: zero rows;
   - with one tenant set: only that tenant's rows;
   - `SELECT version, dirty FROM schema_migrations` is clean and at least the
     deployed release's newest migration.
4. **Objects.** Versioning keeps every overwritten or deleted object for
   `noncurrent_version_retention_days` (≥ the PITR window), so objects the
   restored rows reference still exist:
   - Restore a deleted object by deleting its delete marker
     (`aws s3api list-object-versions` → `delete-object --version-id <marker>`).
   - For a whole-bucket incident, restore the AWS Backup recovery point closest
     to the database's restore time from the vault into a new bucket, and point
     `MINIO_BUCKET` at it.
5. **Cut over.** Stop the worker, point `DATABASE_URL` / `DATABASE_URL_MIGRATE`
   at the new endpoint (or rename the instances), then start api and worker.
   Readiness gates traffic until the schema check passes. Keep the old instance
   (deletion protection on) until the incident is closed.
6. **Record** the incident time, restore time, every step's duration, the
   verification results and what was lost between the restore time and the
   incident.

The AWS Backup vault's daily copies are the fallback when the RDS automated
backups themselves are gone (for example, the instance was deleted along with
them): restore the recovery point with `aws backup start-restore-job`, then
continue from step 3.
