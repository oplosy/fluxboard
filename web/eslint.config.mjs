import { defineConfig, globalIgnores } from "eslint/config";
import nextVitals from "eslint-config-next/core-web-vitals";
import nextTs from "eslint-config-next/typescript";

// Next 16 dropped `next lint`; ESLint runs directly with the flat config.
export default defineConfig([
  ...nextVitals,
  ...nextTs,
  {
    rules: {
      "@typescript-eslint/no-explicit-any": "error",
    },
  },
  globalIgnores([".next/**", ".next-dev/**", ".next-prod/**", ".pnpm-store/**", "out/**", "next-env.d.ts"]),
]);
