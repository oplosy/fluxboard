-- 0065_chat_channels -- Slack / Teams notification channels (FR-NTF-005,
-- roadmap R-11, ADR-040). A channel is one incoming-webhook URL, stored
-- AES-GCM encrypted (url_enc) with a display-only hint; categories picks the
-- notification categories it receives. chat_deliveries is the per-message
-- outbox-driven delivery log (status + last error shown in org settings).
-- DDL only; RLS lands in 0066 (the DDL/RLS split). Times = timestamptz UTC.

CREATE TABLE chat_channels (                       -- [T]
  id         uuid PRIMARY KEY,
  org_id     uuid NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
  provider   text NOT NULL CHECK (provider IN ('slack', 'teams')),
  name       text NOT NULL,
  url_enc    text NOT NULL,                        -- base64(nonce||AES-256-GCM(url))
  url_hint   text NOT NULL,                        -- host + last 4 chars, safe to show
  categories text[] NOT NULL DEFAULT '{}',
  enabled    boolean NOT NULL DEFAULT true,
  created_by uuid NOT NULL REFERENCES users(id),
  created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX chat_channels_org_idx ON chat_channels (org_id, created_at);

CREATE TABLE chat_deliveries (                     -- [T]
  id         uuid PRIMARY KEY,
  org_id     uuid NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
  channel_id uuid NOT NULL REFERENCES chat_channels(id) ON DELETE CASCADE,
  category   text NOT NULL,                        -- notification category or 'test'
  title      text NOT NULL,
  body       text NOT NULL DEFAULT '',
  link       text NOT NULL DEFAULT '',
  status     text NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'sent', 'failed')),
  attempts   int NOT NULL DEFAULT 0,
  last_error text NOT NULL DEFAULT '',
  created_at timestamptz NOT NULL DEFAULT now(),
  sent_at    timestamptz
);
CREATE INDEX chat_deliveries_org_idx ON chat_deliveries (org_id, created_at DESC);
