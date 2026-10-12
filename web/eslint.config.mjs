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
      // New React Compiler rules in eslint-plugin-react-hooks v7 (Next 16).
      // They flag 27 pre-existing patterns in 21 files; kept visible as
      // warnings until those are refactored (ADR-026 follow-up).
      "react-hooks/set-state-in-effect": "warn",
      "react-hooks/purity": "warn",
      "react-hooks/refs": "warn",
    },
  },
  globalIgnores([".next/**", ".next-dev/**", ".pnpm-store/**", "out/**", "next-env.d.ts"]),
]);
