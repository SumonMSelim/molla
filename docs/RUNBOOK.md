# Operations runbook

Operator procedures for mol.la on AWS. `ci.yml` never plans or applies
Terraform and never runs k6 against any environment; it only builds and
tests. Terraform plan and apply run in `terraform-plan.yml` (every PR
touching `infra/terraform/aws`, read-only) and `deploy.yml` (a `vX.Y.Z` tag
push or a manual `workflow_dispatch`, never a plain merge to `main`), both
gated by the `production` GitHub Environment's required reviewer.

## Launch SLOs

| Path | Target | Abort |
| --- | --- | --- |
| Redirect | 15,432 QPS, p99 origin < 100 ms in `test/load/redirect.js` | k6 `abortOnFail` or http_req_failed ≥ 0.1% |
| Create | 154 QPS, p99 < 200 ms in `test/load/create.js` | same |
| Availability | 99.9% monthly excluding accepted regional-risk window | page on 5xx and origin latency alarms |

Load tests run only from an operator workstation against **dev** or a dedicated load account. Never default `BASE_URL` to production.

## Deploy (first time, by hand -- bootstraps GitHub Actions)

`deploy.yml` authenticates to AWS by assuming an IAM role via OIDC, and that
role is itself created by this Terraform configuration (`modules/ci`). The
very first apply has to happen from an operator workstation with short-lived
credentials; every apply after that can run from Actions.

1. Create the Terraform state bucket and DynamoDB lock table named in `infra/terraform/aws/envs/prod/backend.tf`.
2. `make build-lambda` then pass `-var artifact_dir=../../../../../dist` (from `envs/prod`) or copy zips next to the env.
3. `permutation_key`, `privacy_key`, `redis_auth_token`, and `origin_verify_secret` are optional: leave them unset and Terraform generates and stores random values on first apply (see `random_password` resources in `envs/prod/main.tf`). Set them only to pin a specific value.
4. Set tfvars: `admin_principal_arns`, `github_repository` (`SumonMSelim/molla`). Prod also needs `alarm_email`, `budget_limit`, and central trail/config/guardduty IDs.
5. ACM + DNS: set `domain_name` (`mol.la`) and `cloudflare_zone_id` in tfvars and export `CLOUDFLARE_API_TOKEN` (scopes: Zone:DNS:Edit, Zone:Zone Settings:Edit, Zone:Transform Rules:Edit on the mol.la zone). Terraform creates the ACM validation records, the proxied apex CNAME to CloudFront, sets SSL to Full (strict), and a Transform Rule that stamps `X-Origin-Verify`. Empty `domain_name` keeps the CloudFront hostname with no origin check. Cloudflare WAF/rate limiting is the firewall; there is no AWS WAF.
6. `terraform init` and reviewed `terraform plan` in `envs/prod`, then `apply` with short-lived credentials and explicit approval.
7. `make web-build` and `aws s3 sync web/dist s3://$(terraform output -raw ui_bucket)/app --delete`. Invalidate CloudFront `/app/*`.
8. Confirm `GET https://mol.la/app/` and `POST /api/v1/links` (no auth header needed -- create and stats are public). Confirm `GET https://<distribution>.cloudfront.net/` returns 403 (Cloudflare bypass blocked).
9. Wire up GitHub Actions for every deploy after this one (see below).

## GitHub Actions setup (one-time, after the first manual apply)

1. In the repo's Settings → Environments, create `production` with a required reviewer. Every `terraform-plan.yml` and `deploy.yml` run waits for that approval before the OIDC role can be assumed.
2. Set these as Environment **variables** (not secrets -- OIDC needs no long-lived AWS credentials):
   - `AWS_REGION` -- `us-east-1`.
   - `AWS_PLAN_ROLE_ARN` -- `terraform output -raw gha_plan_role_arn`.
   - `AWS_APPLY_ROLE_ARN` -- `terraform output -raw gha_apply_role_arn`.
   - `ADMIN_PRINCIPAL_ARNS` -- HCL list syntax, e.g. `["arn:aws:iam::123456789012:role/ops"]` (passed straight to `-var`).
   - `ALARM_EMAIL`, `DOMAIN_NAME` (`mol.la`), `CLOUDFLARE_ZONE_ID`.
3. Set `CLOUDFLARE_API_TOKEN` as an Environment **secret** (Cloudflare has no OIDC federation, so this is a real static credential, scoped as in step 5 above).
4. Deploy by pushing a `vX.Y.Z` tag, or by running `deploy.yml` manually via `workflow_dispatch` (type `deploy` to confirm). A plain merge to `main` never triggers a deploy.

## Rotate the origin secret

Change `origin_verify_secret` and apply. Cloudflare's rule and the CloudFront function update in one plan; expect a few seconds of 403s while the function propagates. Rotate if the value leaks (CloudFront function code is readable by anyone with `cloudfront:DescribeFunction`).

## API authentication (currently disabled)

`POST /api/v1/links` and `GET /api/v1/links/{code}/stats` are public and unauthenticated. Anyone can shorten a URL or read a code's click count; there is no owner and no per-caller quota beyond the shared rate limits below. `DELETE /api/v1/links/{code}` does not exist; only an operator can remove a link (see Takedown).

The `Credentials` DynamoDB table, `platform.CredentialStore`, and `go run ./cmd/admin issue`/`revoke` still exist but nothing in the API checks them today — they are scaffolding for a later UI-driven token system where a registered user gets their own key. Do not issue credentials expecting them to gate anything yet.

## Rate limiting

Two independent layers, since there is no per-caller auth to key a limit on:

- **Cloudflare** rate-limits `POST /api/v1/links` per client IP (`modules/edge` ruleset). This is the real abuse control for anonymous create.
- **API Gateway** stage throttle (`throttle_rate_limit`/`throttle_burst_limit`) caps total throughput across all callers; it is not per-IP.

Tune the Cloudflare rule first if create abuse shows up; the stage throttle is a blunt aggregate ceiling.

## Takedown

```sh
export MOLLA_INVALIDATE_FUNCTION=$(terraform output -raw invalidate_function_name)
go run ./cmd/admin takedown --code SHORTCODE --reason malware
```

Actor always comes from STS caller identity; it cannot be overridden. Soft-delete + Redis tombstone. Audit JSON on stderr.

## Restore

PITR is on in prod (`enable_pitr`). Restore a table to a new name, verify item counts and a sample GetItem, then swap names in a reviewed change. Dev has PITR off; rebuild from apply + data load if needed. After restore, replay invalidation for deleted codes still in Redis.

## Regional failure

Single-region launch. CloudFront may still serve cached 302s until TTL. Writes and uncached redirects fail with the region.

- Page: origin 5xx, Lambda errors, DynamoDB user errors, Redis CPU/evictions, Kinesis iterator age.
- Failover is a **new region deploy**, not automatic. Restore PITR into the surviving region only as a documented emergency; permutation/privacy keys and DNS cutover are required.
- Do not claim multi-region RTO until a second region exists.
- Cloudflare outage: DNS and proxy fail together; there is no direct path to CloudFront by design.

## Abort a load or canary

Stop k6 (`abortOnFail` or Ctrl-C). For Lambda, roll the alias to the previous published version. Redirect uses provisioned concurrency on alias `live`. Re-run a tiny smoke (`GET /{code}`, `POST /api/v1/links`) before traffic.

## UI bucket

Objects are private; CloudFront OAC reads them. Sync only `web/dist` from a trusted builder. No public ACLs. After sync, invalidate `/app/*` (and `/app/index.html`).
