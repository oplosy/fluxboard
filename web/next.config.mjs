// A release image (FLUXBOARD_RELEASE=1, set by the release workflow) must not
// ship the legal pages with "[not configured]" operator facts (report B09).
const LEGAL_REQUIRED = [
  'NEXT_PUBLIC_LEGAL_ENTITY',
  'NEXT_PUBLIC_LEGAL_ADDRESS',
  'NEXT_PUBLIC_LEGAL_PRIVACY_EMAIL',
  'NEXT_PUBLIC_LEGAL_GOVERNING_LAW',
];
if (process.env.FLUXBOARD_RELEASE === '1') {
  const missing = LEGAL_REQUIRED.filter((k) => !process.env[k]?.trim());
  if (missing.length > 0) {
    throw new Error(`Release build: set ${missing.join(', ')} (legal operator facts, docs/runbooks/privacy-requests.md)`);
  }
  if (process.env.NEXT_PUBLIC_LEGAL_PRIVACY_EMAIL.trim().endsWith('.local')) {
    throw new Error('Release build: NEXT_PUBLIC_LEGAL_PRIVACY_EMAIL must be a real, monitored address');
  }
}

/** @type {import('next').NextConfig} */
const nextConfig = {
  reactStrictMode: true,
  // Keep container dev output separate from host production builds.
  distDir: process.env.NEXT_DIST_DIR ?? '.next',
};

export default nextConfig;
