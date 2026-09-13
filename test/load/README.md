# Load tests

k6 scripts for the launch SLO envelope (15,432 redirect QPS, 154 create QPS). **Never run in CI.** Do not point at production by default.

## Prerequisites

- [k6](https://k6.io/) on the operator workstation
- A non-production environment (dev or a dedicated load account)
- `BASE_URL` = CloudFront origin URL (`https://…cloudfront.net` or the custom domain)
- Redirect: a live `SHORT_CODE` that 302s
- Create: a developer API key that exists in both API Gateway and DynamoDB (`X-Api-Key`)

## Redirect

```sh
BASE_URL=https://dxxxxx.cloudfront.net SHORT_CODE=abc1234 k6 run test/load/redirect.js
```

Abort if p99 exceeds the script threshold or http_req_failed ≥ 0.1%.

## Create

```sh
BASE_URL=https://dxxxxx.cloudfront.net API_KEY=… k6 run test/load/create.js
```

Writes 154 POSTs/s to `/api/v1/links`. Use a disposable owner; delete load-generated codes after the run.

## Notes

- Scripts use `constant-arrival-rate`. Raise `preAllocatedVUs` / `maxVUs` if k6 reports dropped iterations.
- Regional failure and restore drills live in `docs/RUNBOOK.md`, not here.
