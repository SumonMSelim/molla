# molla

[![CI](https://github.com/SumonMSelim/molla/actions/workflows/ci.yml/badge.svg)](https://github.com/SumonMSelim/molla/actions/workflows/ci.yml)
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

Clone and verify the scaffold:

```sh
git clone https://github.com/SumonMSelim/molla.git
cd molla
make build
make test
```

The repository currently provides a buildable project skeleton. The API, redirect service, storage adapters, and infrastructure modules are not implemented yet.

## Local development

Toolchains run in official Docker images by default:

```sh
make build      # compile all Go packages
make test       # run Go tests with the race detector
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

There is no runnable local server yet; `cmd/server` and the AWS entrypoints are bootstrap stubs.

## Deployment

Production deployment is not available yet. `infra/terraform/aws` currently pins the supported Terraform and AWS provider versions; deployable modules and environment roots will be added incrementally.

Current infrastructure validation:

```sh
make tf-check
```

Terraform plan and apply instructions will be added only after the infrastructure is executable and tested.

## Project layout

```text
cmd/                    application entrypoints
internal/core/          provider-neutral domain logic
internal/platform/      interfaces shared by handlers and adapters
internal/handlers/      HTTP handlers
internal/adapters/      memory, Redis, and AWS integrations
infra/terraform/aws/    AWS infrastructure
```

## Contributing

Bug reports and feature requests are welcome through the repository's issue templates. Security vulnerabilities must be reported privately as described in [SECURITY.md](SECURITY.md).

By participating, you agree to follow the [Code of Conduct](CODE_OF_CONDUCT.md).

## License

[MIT](LICENSE)
