# Dev Setup Notes

Getting fluxboard running locally from a clean checkout.

## Steps

1. Install toolchain versions pinned by `.nvmrc` and the Makefile.
2. Copy `.env.example` to `.env` and fill in local values.
3. Use `make` targets from the root `Makefile` — they encode the right ordering for backend and web.
4. Seed/bootstrap the backend first; the web app assumes it is up.

## Common issues

- Port conflicts on first boot — check the Makefile for defaults.
- Stale local state: remove generated caches before re-running setup.

Add to this list when something costs you more than five minutes.
