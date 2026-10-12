# AWS data plane (Terraform)

The production database and object storage with their recovery controls
(report B07, ADR-028). It does not create the network, the compute that runs
the api/worker/web images, DNS or TLS. Those come with the hosting setup and
plug in through `vpc_id`, `db_subnet_ids` and `app_security_group_ids`.

| Resource | Recovery control |
|---|---|
| RDS PostgreSQL 16 (private, encrypted, `rds.force_ssl`, Multi-AZ by default) | automated backups = **PITR** over `pitr_retention_days` (default 14; WAL shipped about every 5 min, inside the RPO ≤ 15 min target); final snapshot on delete; deletion protection; master password in Secrets Manager |
| S3 bucket for attachments, avatars and logos (private, SSE, TLS-only, CORS for `web_origins`) | **versioning**: overwritten or deleted objects stay recoverable for `noncurrent_version_retention_days` (default 30, must be ≥ the PITR window) |
| AWS Backup vault (own KMS key, governance-mode vault lock) | daily copy of the database and the bucket, kept `backup_vault_retention_days` (default 35), out of reach of the app's credentials |
| IAM policy `app_objects` | least-privilege bucket access for the api/worker identity |

RTO/RPO targets and how to restore: `docs/runbooks/restore.md`.

## Before the first apply

1. **Region:** `eu-central-1` (Frankfurt), the region the privacy policy names
   (ADR-031). Changing it means changing `web/lib/legal/operator.ts` and
   re-announcing the sub-processor list.
2. **Remote state:** add a `backend "s3"` block (bucket with versioning,
   `use_lockfile = true` or a DynamoDB lock table) to `versions.tf` per
   account. State holds resource ids and the master secret's ARN, not the
   password.
3. Copy `prod.tfvars.example` to `prod.tfvars` (git-ignored) and fill it in.

```bash
cd deploy/aws
terraform init
terraform plan -var-file=prod.tfvars
terraform apply -var-file=prod.tfvars
```

`terraform test` checks the input guards against a mocked provider without an
account; CI runs it with `fmt -check` and `validate`.

## After apply: database roles

The RDS master user (`fluxboard_master`) only bootstraps the three app roles.
Create them with the statements of `deploy/postgres/init/01-roles.sql`, using
**strong passwords from your secret store** instead of the dev passwords in
that file. The master user's password is in the secret named by the
`db_master_secret_arn` output. On RDS the master user is `rds_superuser`, not
a superuser: run `SET createrole_self_grant = 'set, inherit';` first in the
same session, so it may hand the `public` schema to `fluxboard_owner`. This
step has not been run against RDS yet; check the result with `\dn+` and `\du`.

Then point the app at it:

- `DATABASE_URL` → `fluxboard_app@<db_endpoint>/fluxboard?sslmode=verify-full`.
- `DATABASE_URL_MIGRATE` → `fluxboard_owner@…`.
- `MINIO_ENDPOINT=s3.<region>.amazonaws.com`, `MINIO_USE_SSL=true`.
- `MINIO_BUCKET=<objects_bucket>`.
- `STORAGE_CORS_ORIGINS` matching `web_origins`.
- Attach `app_objects_policy_arn` to the api/worker identity.

## Transactional email: Amazon SES (ADR-031)

Mail (verification, invitations, alerts) goes through SES in the same account
and region, so no further sub-processor is involved. SES is not in this
Terraform yet because it needs the operator's mail domain:

1. Verify the sending domain in SES (eu-central-1) and publish its DKIM records.
2. Ask AWS to lift the SES sandbox for the account.
3. Create SMTP credentials (an IAM user with `ses:SendRawEmail`). Store them in
   the secret store, never in this repository.
4. Set the app env:
   - `SMTP_HOST=email-smtp.eu-central-1.amazonaws.com`, `SMTP_PORT=587`;
   - `SMTP_USER`/`SMTP_PASSWORD` = the SMTP credentials;
   - `SMTP_FROM=no-reply@<domain>`. The API refuses a `*.local` sender in prod.
5. Use the same values for Alertmanager (`ALERT_EMAIL_TO` plus the SMTP settings;
   TLS is required by default in prod).
