# Operations runbook

Operator procedures for mol.la on AWS. CI never plans or applies Terraform and never runs k6 against any environment.

## Launch SLOs

| Path | Target | Abort |
| --- | --- | --- |
| Redirect | 15,432 QPS, p99 origin < 100 ms in `test/load/redirect.js` | k6 `abortOnFail` or http_req_failed ≥ 0.1% |
| Create | 154 QPS, p99 < 200 ms in `test/load/create.js` | same |
| Availability | 99.9% monthly excluding accepted regional-risk window | page on 5xx and origin latency alarms |

Load tests run only from an operator workstation against **dev** or a dedicated load account. Never default `BASE_URL` to production.

## Deploy (first time)

1. Create the Terraform state bucket and DynamoDB lock table named in `infra/terraform/aws/envs/*/backend.tf`.
2. `make build-lambda` then pass `-var artifact_dir=../../../../../dist` (from the env dir) or copy zips next to the env as agreed by the pipeline.
3. Set tfvars: `permutation_key`, `privacy_key`, `redis_auth_token` (≥16 chars), `admin_principal_arns`, `origin_verify_secret` (≥32 random chars). Prod also needs alarm email, budget, and central trail/config/guardduty IDs.
4. `terraform init` and reviewed `terraform plan` in `envs/dev`, then `apply` only with short-lived credentials and explicit approval.
5. ACM + DNS: set `domain_name` (`mol.la`) and `cloudflare_zone_id` in tfvars and export `CLOUDFLARE_API_TOKEN` (scopes: Zone:DNS:Edit, Zone:Zone Settings:Edit, Zone:Transform Rules:Edit on the mol.la zone). Terraform creates the ACM validation records, the proxied apex CNAME to CloudFront, sets SSL to Full (strict), and a Transform Rule that stamps `X-Origin-Verify`. Empty `domain_name` keeps the CloudFront hostname with no origin check. Cloudflare WAF/rate limiting is the firewall; there is no AWS WAF.
6. Seed the first developer key (below).
7. `make web-build` and `aws s3 sync web/dist s3://$(terraform output -raw ui_bucket)/app --delete`. Invalidate CloudFront `/app/*`.
8. Confirm `GET https://mol.la/app/` and `POST /api/v1/links` with both the API Gateway key and `X-Api-Key`. Confirm `GET https://<distribution>.cloudfront.net/` returns 403 (Cloudflare bypass blocked).

## Rotate the origin secret

Change `origin_verify_secret` and apply. Cloudflare's rule and the CloudFront function update in one plan; expect a few seconds of 403s while the function propagates. Rotate if the value leaks (CloudFront function code is readable by anyone with `cloudfront:DescribeFunction`).

## Rotate developer keys

Go authenticates SHA-256 hashes in DynamoDB (`Credentials`). API Gateway meters a separate key value. Both must match the raw token the client sends as `X-Api-Key`.

**Issue new, then revoke old.** Max lifetime 90 days.

```sh
# Assume the admin role from terraform output admin_role_arn.
export MOLLA_INVALIDATE_FUNCTION=$(terraform output -raw invalidate_function_name)
export MOLLA_LINKS_TABLE=molla-dev-links   # match terraform table names
export MOLLA_CREDENTIALS_TABLE=molla-dev-credentials

# Bind an existing API Gateway key value into DynamoDB:
aws apigateway get-api-key --api-key "$(terraform output -raw api_key_id)" --include-value
go run ./cmd/admin issue --owner OWNER --token THE_GATEWAY_VALUE --days 90

# Or generate a token, then create a Gateway key with that exact value and attach it to usage_plan_id:
go run ./cmd/admin issue --owner OWNER --days 90
# stdout prints the raw token once. Create API key with that value; associate with the usage plan.
```

Revoke the previous DynamoDB item (status revoked / delete row) only after clients use the new token. Disable the old API Gateway key.

## Takedown

```sh
export MOLLA_INVALIDATE_FUNCTION=$(terraform output -raw invalidate_function_name)
go run ./cmd/admin takedown --code SHORTCODE --reason malware
```

Actor comes from STS unless `--actor` is set. Soft-delete + Redis tombstone. Audit JSON on stderr.

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
