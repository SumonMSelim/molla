# molla

[![CI](https://github.com/SumonMSelim/molla/actions/workflows/ci.yml/badge.svg)](https://github.com/SumonMSelim/molla/actions/workflows/ci.yml)
[![CodeQL](https://github.com/SumonMSelim/molla/actions/workflows/codeql.yml/badge.svg)](https://github.com/SumonMSelim/molla/actions/workflows/codeql.yml)
[![codecov](https://codecov.io/gh/SumonMSelim/molla/graph/badge.svg)](https://codecov.io/gh/SumonMSelim/molla)
[![Cloudflare](https://img.shields.io/badge/Cloudflare-Workers%20%7C%20D1%20%7C%20Access-F38020?logo=cloudflare&logoColor=white)](https://workers.cloudflare.com)
[![Terraform](https://img.shields.io/badge/Terraform-1.16-844FBA?logo=terraform&logoColor=white)](https://www.terraform.io)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

**A scalable, reliable, and secure URL shortener.**

molla is an open-source URL-shortening service that runs entirely on
Cloudflare: one Worker serves redirects, the JSON API, and the static UI; D1
is the authoritative store; the edge cache absorbs hot redirects; Access
gates operator takedowns. It is sized to stay inside the Workers Free plan.
The original AWS (Go, Lambda, DynamoDB) implementation is kept for
reference on the `aws` branch; its infrastructure has been retired.

## Requirements

- Docker
- Optional: Node 24 and Terraform 1.16 for running toolchains directly

## Installation

Clone and verify:

```sh
git clone https://github.com/SumonMSelim/molla.git
cd molla
make worker-test
```

## Local development

Toolchains run in official Docker images by default:

```sh
make worker-lint   # wrangler types, oxlint, tsc
make worker-test   # vitest inside workerd (builds the SPA first)
make web-lint      # oxlint
make web-test      # vitest + jsdom
make web-build     # web/dist
make tf-check      # format-check and validate Terraform
```

Coverage (Istanbul for the worker, V8 for the web app) is written to
`coverage/lcov.info` in each package:

```sh
cd worker && npm run coverage
cd web && npm run coverage
```

Use installed host toolchains by overriding command variables:

```sh
make worker-test NPM=npm NPM_WORKER=npm
make tf-check TF=terraform
```

Local API + UI (see `web/README.md`):

```sh
make dev                          # wrangler dev on :8787 with a local D1
cd web && npm ci && npm run dev   # Vite at /app/, proxies /api
```

## Deployment

The Worker, its D1 database, cron trigger, static assets, and secrets are
declared in `worker/wrangler.jsonc`. Zone-level configuration (DNS, WAF rate
limit, response headers, Access application) is Terraform in
`infra/terraform/cloudflare`. `ci.yml` only lints and tests on every push
and PR; it never plans or applies. Terraform plan runs in
`terraform-plan.yml` on every PR touching `infra/terraform/cloudflare`
(read-only, posts the diff as a PR comment); apply plus `wrangler deploy`
run in `deploy.yml`, triggered only by a `vX.Y.Z` tag push or a manual
`workflow_dispatch`, never by a plain merge to `main`. Deploys run under
the `production` GitHub Environment, which only accepts `v*.*.*` tags and
has no required reviewer, so publishing a release deploys immediately.
Cloudflare has no OIDC federation, so the API token is a GitHub Environment
secret scoped to this zone and account.

```sh
make assets         # web/dist copied into worker/public
make worker-deploy  # d1 migrations apply + wrangler deploy --env prod
```

The very first deploy has to happen by hand from an operator workstation:
it creates the D1 database, the R2 state bucket, and the Access application
whose audience the Worker needs. See `docs/RUNBOOK.md` for that bootstrap,
the API token scopes, secrets, and the one-time GitHub Environment setup.

## Security and CI

- `ci.yml` lints, tests with coverage (uploaded to Codecov via the
  `CODECOV_TOKEN` repository secret), builds, validates Terraform and runs
  tfsec. `codeql.yml` scans the TypeScript weekly and on every PR.
- Every third-party action is pinned to a commit SHA; Dependabot keeps the
  actions, npm packages and Terraform providers current.
- Workflows default to read-only `GITHUB_TOKEN` permissions and do not
  persist checkout credentials. Dependencies install in a step that has no
  access to the Cloudflare API token.
- Production secrets live only in the `production` and `production-plan`
  GitHub Environments. `production` accepts `v*.*.*` tags only.
- Responses carry a strict CSP, HSTS and `nosniff`; the only third-party
  origin allowed is Cloudflare's Web Analytics beacon.
- `main` is protected by a ruleset: changes land through pull requests with
  passing checks, and force pushes and deletion are blocked.

## Project layout

```text
worker/src/core/            provider-neutral domain logic (Base62, permutation, validation)
worker/src/handlers/        create, redirect, stats, takedown
worker/src/store.ts         D1 adapter (links, idempotency, counters, stats, audit)
worker/src/access.ts        Cloudflare Access JWT verification
worker/migrations/          D1 schema
worker/test/                vitest (runs inside workerd); golden.json pins the Go output
infra/terraform/cloudflare/ zone configuration
web/                        static SPA (Vite) served at /app/*
docs/                       runbook and Well-Architected review
.github/workflows/          ci, codeql, terraform-plan, deploy
```

## Contributing

Bug reports and feature requests are welcome through the repository's issue templates. Security vulnerabilities must be reported privately as described in [SECURITY.md](SECURITY.md).

By participating, you agree to follow the [Code of Conduct](CODE_OF_CONDUCT.md).

## License

[MIT](LICENSE)
