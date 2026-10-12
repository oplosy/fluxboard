# Release and rollback

How a version is cut, what it produces, and how to deploy or roll it back
(report B07). Hosting is not provisioned yet; the steps name the artefacts and
their order, not a provider's commands.

## Cut a release

1. `main` is green (all five required checks).
2. One-time: set the repository variable `NEXT_PUBLIC_API_BASE` to the public
   https API base (for example `https://api.example.com/api/v1`). The browser
   calls the API directly, so this URL is baked into the web image; the release
   workflow fails without it.
3. Tag the commit on `main` and push the tag:

   ```bash
   git tag -a v0.1.0 -m "v0.1.0"
   git push origin v0.1.0
   ```

`.github/workflows/release.yml` then:

- re-runs the full CI gate (`ci.yml`) on the tagged commit;
- builds and pushes four images to GHCR, each tagged `X.Y.Z`, `X.Y` and
  `sha-<commit>`, with an SBOM and max-mode provenance attestation:
  - `ghcr.io/<owner>/fluxboard-migrations`: this release's SQL with golang-migrate,
  - `ghcr.io/<owner>/fluxboard-api` and `fluxboard-worker`: distroless, nonroot,
  - `ghcr.io/<owner>/fluxboard-web`: Next.js production server;
- opens a GitHub Release whose notes list every image by digest.

Deploy by digest from the release notes, not by tag: a tag can be moved, a
digest cannot.

## Deploy order

1. **Migrations:** run the release's migrations image against the database as
   the owner role:

   ```bash
   docker run --rm <migrations-image@digest> -database "$DATABASE_URL_MIGRATE" up
   ```

2. **api** and **worker:** roll out the new images. A pod only takes traffic
   once `/readyz` is 200, which requires the schema at or past the newest
   migration in the binary (schema behind or dirty → 503).
3. **web:** roll out last.

## Migration rule: expand, then contract

A release's migrations must leave the schema usable by the previous release's
binaries, because old pods keep serving while new ones start and a rollback
runs old binaries on the new schema. `/readyz` accepts a schema newer than the
binary for exactly this reason.

- Allowed in one release (expand): new tables, nullable or defaulted columns,
  new indexes (`CONCURRENTLY` on large tables), new enum values, new RLS
  policies that do not narrow existing access.
- Never in the same release as the code that stops using it (contract): drop
  or rename a column/table, add `NOT NULL` without a default, narrow a type or
  constraint. Ship the code that no longer reads or writes it first; contract
  in a later release.
- Every migration has a working `.down.sql`; CI applies all migrations, rolls
  every one back and applies them again on each PR.

## Roll back

- **Application:** redeploy the previous release's api, worker and web digests.
  Because migrations are expand-only, the previous binaries run on the current
  schema; leave the schema where it is.
- **Schema:** only when a migration itself is broken. Down migrations can drop
  data that was written after the up ran, so take a backup or note the PITR
  timestamp first (see `restore.md`), then with the *newer* migrations image:

  ```bash
  docker run --rm <migrations-image@digest> -database "$DATABASE_URL_MIGRATE" down 1
  ```

  After a failed migration golang-migrate marks the version dirty, and
  `/readyz` stays 503 until it is resolved: fix the cause, then
  `force <last good version>` and run `up` again.
- **Data:** restoring a database is a recovery, not a rollback; follow
  `restore.md`.

## Not covered yet

- A staging environment that receives every release before production.
- Automated deployment of the digests; with no hosting yet, the deploy order
  above is run by hand.
