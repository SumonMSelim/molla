# AGENTS.md

Guidance for AI coding agents (and humans) working in this repo. molla is the
mol.la URL shortener: one Cloudflare Worker (redirects, JSON API, static UI),
D1 for storage, Terraform for zone config. Details live in `README.md` and
`docs/RUNBOOK.md`; this file holds what is easy to get wrong.

## Verify before you finish

```sh
make check          # worker lint+test, web lint+test, terraform fmt+validate
```

Individual targets: `worker-lint`, `worker-test`, `web-lint`, `web-test`,
`web-build`, `tf-check`. Everything runs in Docker by default; CI overrides
with `NPM=npm NPM_WORKER=npm TF=terraform`. `worker-test` builds the SPA
first (`make assets`), because tests serve the real built assets.

## Layout

- `worker/src/` Worker. `core/` is pure logic, `handlers/` one file per
  endpoint, `store.ts` the D1 adapter, `index.ts` the router.
- `worker/site/` root-level discovery files (`robots.txt`, `sitemap.xml`,
  `llms.txt`, `openapi.json`), copied to the site root by `make assets`.
- `web/` Vite + React SPA served under `/app/`. Static pages are prerendered
  at build time (`web/src/prerender.tsx`, `web/scripts/prerender.mjs`).
- `infra/terraform/cloudflare/` zone rulesets, DNS, Access.

## Rules that are not obvious

- **Deploys happen only through a GitHub release**: `gh release create vX.Y.Z`
  triggers `deploy.yml`. Never deploy from a merge, a bare tag, or locally.
- **Never run `terraform apply`**, and never run `wrangler deploy` or D1
  migrations against prod. CI plans (read-only); a human applies.
- **Zone rulesets must stay scoped to the apex host** (`http.host eq
  "mol.la"`). An unscoped `true` expression once broke `id.mol.la`'s own CSP.
  Cloudflare allows one zone ruleset per phase.
- **The CSP string exists in three places that must match**:
  `worker/src/http.ts`, `web/index.html`, `infra/terraform/cloudflare/main.tf`
  (plus the dev-only rewrite regex in `web/vite.config.ts`).
- **API contract has three copies**: handlers, `worker/site/openapi.json`, and
  the Developers page (`web/src/pages/Developers.tsx`). `worker/test/agent.test.ts`
  fails when the spec's error codes or response fields drift from the code.
  Error codes and messages live in `ERROR_MESSAGES` (`worker/src/http.ts`);
  add a code there and in the spec's `Error.error.enum`.
- **`/.well-known/security.txt` has an `Expires` date** in
  `worker/src/wellknown.ts`; renew it before it lapses (RUNBOOK).
- **Short links count clicks per request.** Do not `curl` a short link to
  test it in prod; use `/api/v1/links/{code}/stats`. `robots.txt` blocks
  crawlers from them for the same reason.
- The `aws` branch is a retired reference copy of the old Go/Lambda stack.
  Do not merge it or revive its infrastructure.
- Workflows pin actions to commit SHAs; keep the version comment and let
  Dependabot bump them. Do not add `pull_request_target` or broaden
  `permissions`.

## Conventions

- Conventional Commits, lowercase subject, no attribution trailers.
- Add or update tests with every logic change (vitest; worker tests run
  inside workerd via `@cloudflare/vitest-pool-workers`).
- Secrets live in GitHub Environments and Wrangler secrets. Never read or
  create `.env*` or `cloudflare.env` files in the repo.
- `main` is protected: open a PR; required checks must pass.
