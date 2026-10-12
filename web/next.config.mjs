/** @type {import('next').NextConfig} */
const nextConfig = {
  reactStrictMode: true,
  // Keep container dev output separate from host production builds.
  distDir: process.env.NEXT_DIST_DIR ?? '.next',
};

export default nextConfig;
