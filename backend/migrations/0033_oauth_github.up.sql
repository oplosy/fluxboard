-- 0033_oauth_github — allow GitHub as a sign-in provider next to Google
-- (FR-AUTH-009, ADR-032). DDL only: oauth_identities is a global auth table
-- outside RLS, so there is no policy half to this migration.
ALTER TABLE oauth_identities DROP CONSTRAINT oauth_identities_provider_check;
ALTER TABLE oauth_identities
  ADD CONSTRAINT oauth_identities_provider_check CHECK (provider IN ('google', 'github'));
