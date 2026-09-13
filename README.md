# molla

[![CI](https://github.com/SumonMSelim/molla/actions/workflows/ci.yml/badge.svg)](https://github.com/SumonMSelim/molla/actions/workflows/ci.yml)
[![codecov](https://codecov.io/gh/SumonMSelim/molla/graph/badge.svg)](https://codecov.io/gh/SumonMSelim/molla)
[![Go](https://img.shields.io/badge/Go-1.27-00ADD8?logo=go&logoColor=white)](https://go.dev)
[![AWS](https://img.shields.io/badge/AWS-Lambda%20%7C%20DynamoDB%20%7C%20CloudFront-FF9900?logo=amazonwebservices&logoColor=white)](https://aws.amazon.com)
[![Terraform](https://img.shields.io/badge/Terraform-1.16-844FBA?logo=terraform&logoColor=white)](https://www.terraform.io)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

**A scalable, reliable, and secure URL shortener.**

molla is an open-source URL-shortening service written in Go, with AWS as its first deployment target. It is designed around a small provider-neutral core, low-latency redirects, durable link storage, asynchronous analytics, and explicit security boundaries.

## Requirements

- Docker
- Optional: Go 1.27 and Terraform 1.16 for running toolchains directly

## Installation

Clone and verify:

```sh
git clone https://github.com/SumonMSelim/molla.git
cd molla
make build
make test
```

## Local development

Toolchains run in official Docker images by default:

```sh
make build      # compile all Go packages
make test       # run Go tests with the race detector
make coverage   # run race tests and write coverage.out
make vet        # run go vet
make lint       # check Go formatting
make bench      # run Go benchmarks
make tf-check   # format-check and validate Terraform
```

Use installed host toolchains by overriding command variables:

```sh
make test GO=go
make lint GOFMT=gofmt
make tf-check TF=terraform
```

Local API + UI (see `web/README.md`):

```sh
make dev-api          # Go net/http on :8080, in-memory adapters
cd web && npm ci && npm run dev   # Vite at /app/, proxies /api
```

## Deployment

Lambdas and Terraform modules are in-repo. **Operators** apply with short-lived AWS credentials after a reviewed plan. CI only runs `make tf-check` (fmt + init `-backend=false` + validate). Never plan/apply from CI.

```sh
make build-lambda          # dist/{api,redirect,invalidate,aggregate}.zip
make web-build             # web/dist for the /app/* SPA
make tf-check              # no cloud account
```

From `infra/terraform/aws/envs/dev` (or `prod`): configure the S3 backend, supply tfvars (`permutation_key`, `privacy_key`, `redis_auth_token`, `admin_principal_arns`), `terraform init`, reviewed `plan`, then `apply`. Point `artifact_dir` at `dist` for real zips (placeholders exist only so validate works).

After apply:

1. Seed DynamoDB credentials and the matching API Gateway key — `go run ./cmd/admin issue` (see `docs/RUNBOOK.md`).
2. `aws s3 sync web/dist s3://$(terraform output -raw ui_bucket) --delete`
3. Invalidate CloudFront `/app/*`
4. ACM/DNS for mol.la → distribution domain (`terraform output distribution_domain`)

Load envelope (operator workstation, not CI, not prod-by-default): `test/load/`. Full procedures: `docs/RUNBOOK.md`.

## Project layout

```text
cmd/                    application entrypoints
internal/core/          provider-neutral domain logic
internal/platform/      interfaces shared by handlers and adapters
internal/handlers/      HTTP handlers
internal/adapters/      memory, Redis, and AWS integrations
infra/terraform/aws/    AWS infrastructure
web/                    static SPA (Vite) served at /app/*
test/load/              k6 scripts (operator only)
```

## Contributing

Bug reports and feature requests are welcome through the repository's issue templates. Security vulnerabilities must be reported privately as described in [SECURITY.md](SECURITY.md).

By participating, you agree to follow the [Code of Conduct](CODE_OF_CONDUCT.md).

## License

[MIT](LICENSE)
