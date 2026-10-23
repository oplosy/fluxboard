-- 0057_dashboard_layouts -- Per-user org dashboard layout (FR-AN-005, roadmap
-- R-09). One row per (org, user) holding the widget list: [{id, type,
-- project_ids}], validated by the application (analytics.NormalizeLayout).
-- Only widget choices are stored, never data.
-- DDL only; RLS lands in 0058 (the DDL/RLS split).

CREATE TABLE dashboard_layouts (                   -- [T]
  org_id     uuid NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
  user_id    uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  widgets    jsonb NOT NULL DEFAULT '[]'::jsonb,
  updated_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (org_id, user_id)
);
