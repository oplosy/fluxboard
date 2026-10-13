-- GitHub identities cannot satisfy the narrower check; drop them first. Their
-- users keep their accounts and can still sign in by password or Google.
DELETE FROM oauth_identities WHERE provider = 'github';
ALTER TABLE oauth_identities DROP CONSTRAINT oauth_identities_provider_check;
ALTER TABLE oauth_identities
  ADD CONSTRAINT oauth_identities_provider_check CHECK (provider IN ('google'));
