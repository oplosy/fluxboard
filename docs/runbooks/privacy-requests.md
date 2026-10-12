# Privacy requests and the legal go-live checklist

How data-subject requests are handled (KVKK art. 11, GDPR art. 15–21) and which
operator facts must be set before the public site goes live (report B09,
ADR-031).

## Go-live checklist

The legal pages (`/legal/privacy`, `/legal/terms`, `/legal/dpa`) read their
operator facts from build-time variables. A release build
(`FLUXBOARD_RELEASE=1`, set by `.github/workflows/release.yml`) fails while any
is missing, so the site cannot ship with placeholders. Set them as repository
variables:

| Variable | Example | Shown as |
|---|---|---|
| `NEXT_PUBLIC_LEGAL_ENTITY` | `Acme Yazılım A.Ş.` | data controller, service provider |
| `NEXT_PUBLIC_LEGAL_ADDRESS` | registered address | next to the entity |
| `NEXT_PUBLIC_LEGAL_PRIVACY_EMAIL` | `privacy@acme.com` | privacy and terms contact; must not be `*.local` |
| `NEXT_PUBLIC_LEGAL_SECURITY_EMAIL` | `security@acme.com` | vulnerability reports (falls back to the privacy address) |
| `NEXT_PUBLIC_LEGAL_GOVERNING_LAW` | `the Republic of Türkiye` | governing law of the terms |

Before go-live:

1. Both mailboxes exist and someone reads them. Send a test message to each and
   confirm it arrives.
2. The API's `SMTP_FROM` is on the operator's domain. In prod the API refuses
   to boot with a `*.local` sender.
3. The sub-processor list in `web/lib/legal/operator.ts` matches what is
   actually deployed: AWS in eu-central-1 (hosting, storage, backups, SES), and
   Stripe. A new sub-processor goes into that list, and is announced 30 days
   ahead per the DPA, before it receives data.
4. KVKK transfer abroad: hosting in Frankfurt is a transfer abroad for users in
   Türkiye (art. 9). Sign the standard contract with the provider, and notify
   it to the Authority within the statutory period, before data flows. The
   privacy policy relies on this mechanism. Legal review of the final texts is
   recommended; this repository does not replace it.

## Requests from a user about their own data

| Request | How |
|---|---|
| Access / portability | **Self-service:** Account → Profile → *Download my data* (`GET /api/v1/me/export`): account, sign-in identities, organization memberships, sessions. Secrets (password verifier, 2FA secret, recovery codes, tokens) are never included. |
| Correction | Self-service on the profile page. The email address is corrected by support until the email-change flow exists (ADR-015). |
| Deletion | **Self-service:** Account → Danger zone → *Delete account* (`DELETE /api/v1/me`). Blocked while the user is the sole owner of an org; ownership must be transferred or the org deleted first. |
| Objection, restriction, other | Email to the privacy address. Answer within 30 days, free of charge. |

Rules for email requests:

- Verify identity before disclosing or deleting anything. Answer only to the
  account's registered address, or confirm through a signed-in session.
- Never send an export to a different address than the account's.
- Record the date received, the request, the identity check, the action taken
  and the date answered. Keep the record for the statutory period.

## Content inside an organization

Projects, tasks, comments and files belong to the organization, which is the
controller for that data; we process it as processor (DPA).

- A request about such content goes to the organization's owners and admins.
- If it reaches us, forward it to them without undue delay (DPA §7) and tell the
  requester we did.
- Owners can delete content, or delete the organization, which purges its data
  after the wind-down window.

## Backups after deletion

Deleted data stays in backups for a limited time (`deploy/aws`, ADR-028):

- database point-in-time recovery: 14 days;
- earlier file versions: 30 days;
- daily vault copies: 35 days.

It is gone from every backup within **35 days**. The privacy policy states this.
A restore from backup (`restore.md`) must not resurrect data a user deleted,
except as part of recovering the whole service. After such a restore, re-apply
deletions requested in the restored window.
