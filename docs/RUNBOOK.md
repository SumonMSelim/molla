# Operations runbook

Operator procedures for mol.la on Cloudflare. `ci.yml` never plans, applies,
or deploys; it only lints and tests. Terraform plan runs in
`terraform-plan.yml` (every PR touching `infra/terraform/cloudflare`,
read-only) and apply plus `wrangler deploy` run in `deploy.yml` (a `vX.Y.Z`
tag push or a manual `workflow_dispatch`, never a plain merge to `main`).
Deploys run under the `production` GitHub Environment, which only accepts
`v*.*.*` tags and has no required reviewer.

## Plan and quotas

The account is on the Workers Free plan by design. Daily limits reset at
00:00 UTC and **fail closed** (requests return error 1027, nothing is billed):

| Resource | Free per day | Where it is spent |
| --- | --- | --- |
| Worker requests | 100,000 | redirects, API, SPA deep links; static assets do not count |
| Worker CPU | 10 ms per request | redirect is I/O wait; permutation is microseconds |
| D1 rows written | 100,000 | 1 per redirect (click counter; 2 on a code's first click), 3 per create plus 3 more with an `Idempotency-Key` (the web UI always sends one), 2 per takedown, 1 per 100 creates (ID block) |
| D1 rows read | 5,000,000 | redirect cache misses, stats |
| Workers Logs | 200,000 events, 3 days | console output |

D1 counts every index entry a write touches as another row written, including
the automatic index behind each `TEXT PRIMARY KEY`; the figures above include
them. Over its daily row limits, D1 rejects queries until 00:00 UTC.

Storage is not a daily limit: on the Free plan one D1 database holds at most
500 MB (5 GB per account). The cron purge removes expired links and replay
records, but `stats` rows for purged links are never deleted, so that table
only grows.

Set a Cloudflare Notification ("Workers usage approaching limit") and treat
~70,000 requests/day or ~70,000 D1 writes/day as the trigger to move to
Workers Paid ($5/mo), which turns the hard stop into overage billing.

There is no load-test suite: any sustained run burns the daily quota.

## Deploy (first time, by hand -- bootstraps GitHub Actions)

1. **API token**, an account-owned token (Manage Account → Account API
   Tokens) with two policies. Entire account: Workers Scripts:Edit, D1:Edit,
   Access: Apps and Policies:Edit, Access: Service Tokens:Edit, Account
   Settings:Read. Zone mol.la only: Zone:Read, DNS:Edit, Zone Settings:Edit,
   Zone WAF:Edit, Bot Management:Edit, Transform Rules:Edit, Dynamic URL
   Redirects:Edit, Workers Routes:Edit, Cache Purge:Purge. Export as
   `CLOUDFLARE_API_TOKEN`; export `CLOUDFLARE_ACCOUNT_ID` too. Page Rules
   are not used because that API rejects account-owned tokens.
2. **State bucket**: create an R2 bucket (e.g. `molla-tfstate`) and an R2
   API token (Object Read & Write on that bucket). Export its key pair as
   `AWS_ACCESS_KEY_ID`/`AWS_SECRET_ACCESS_KEY`; the S3-compatible backend
   reads those names.
3. **Zero Trust**: ensure the account has a team name (Zero Trust → Settings).
   Access is free for up to 50 users.
4. **Terraform** in `infra/terraform/cloudflare`:
   ```sh
   terraform init \
     -backend-config="bucket=molla-tfstate" \
     -backend-config="endpoints={s3=\"https://$CLOUDFLARE_ACCOUNT_ID.r2.cloudflarestorage.com\"}"
   terraform plan -var account_id=... -var zone_id=... -var access_team_name=... \
     -var 'operator_emails=["you@example.com"]'
   terraform apply ...
   terraform output access_aud access_team_domain
   ```
   Copy `access_aud` and `access_team_domain` into `env.prod.vars` in
   `worker/wrangler.jsonc`.
5. **D1**: `cd worker && npx wrangler d1 create molla-prod`; put the returned
   `database_id` into `env.prod.d1_databases` in `wrangler.jsonc`.
6. **Secrets** (`cd worker`, each prompts for the value):
   ```sh
   npx wrangler secret put PERMUTATION_KEY --env prod   # 32+ random bytes; never rotate (codes depend on it)
   npx wrangler secret put CF_ZONE_ID --env prod        # optional: global purge on takedown
   npx wrangler secret put CF_PURGE_TOKEN --env prod    # optional: token with Cache Purge only
   ```
   Stored codes redirect regardless of key, and a new ID that happens to
   permute onto an old code is caught by the uniqueness check and retried.
7. `make worker-deploy` (applies migrations, deploys to the `mol.la/*` route).
8. Confirm `GET https://mol.la/app/`, `POST /api/v1/links`, a 302 on the new
   code, and a 401 on `POST /admin/v1/links/x/takedown` without Access.
9. Wire up GitHub Actions for every deploy after this one (see below).

## GitHub Actions setup (one-time, after the first manual deploy)

1. Settings → Environments: create `production` (deployment tags
   `v*.*.*`, no required reviewer) and `production-plan`.
2. Environment **variables** on both: `CLOUDFLARE_ACCOUNT_ID`,
   `CLOUDFLARE_ZONE_ID`, `DOMAIN_NAME` (`mol.la`), `ACCESS_TEAM_NAME`,
   `OPERATOR_EMAILS` (HCL list syntax, e.g. `["you@example.com"]`),
   `TF_STATE_BUCKET`.
3. Environment **secrets** on both: `CLOUDFLARE_API_TOKEN`, `R2_ACCESS_KEY_ID`,
   `R2_SECRET_ACCESS_KEY`. `production` holds the write-scoped token and the
   R2 Object Read & Write key. `production-plan` runs on PRs, so it holds a
   read-only token (Read on the same permission groups) and an R2 Object Read
   only key scoped to the state bucket.
4. Deploy by pushing a `vX.Y.Z` tag or running `deploy.yml` manually (type
   `deploy` to confirm). A plain merge to `main` never deploys.

## API authentication (currently disabled)

`POST /api/v1/links` and `GET /api/v1/links/{code}/stats` are public and
unauthenticated. Anyone can shorten a URL or read a code's click count.
`DELETE /api/v1/links/{code}` does not exist; only an operator can remove a
link (see Takedown).

## Rate limiting

Two layers, since there is no per-caller auth to key a limit on:

- **Zone WAF rule** (`cloudflare_ruleset.api_rate_limit`): 3 creates per 10s
  per IP, blocked at the edge before the Worker runs (so it costs no quota).
- **Worker binding** (`CREATE_LIMITER` in `wrangler.jsonc`): 10 per 60s per
  IP inside the Worker, returns `429 RATE_LIMITED`.

Tune the zone rule first; it is the one that protects the daily request quota.

## Discovery files

`worker/site/` (robots, sitemap, llms.txt, OpenAPI) ships with the assets;
`/.well-known/*` is generated by `worker/src/wellknown.ts`. Renew the
`Expires` date in `security.txt` (`SECURITY_EXPIRES`) at least once a year;
it is currently 2027-10-01. When adding a public page, add it to
`web/src/prerender.tsx` (if static) and `worker/site/sitemap.xml`.

## Takedown

The route is `POST /admin/v1/links/{code}/takedown` with body
`{"reason": "..."}`. It sits behind Cloudflare Access; the Worker verifies
the Access JWT and records the attested identity as the actor.

In a browser, open `https://mol.la/admin/`. Cloudflare Access asks for a
one-time PIN sent to an operator email (`OPERATOR_EMAILS` repository
variable, applied by the next release); the page looks the link up, asks
for a reason, and confirms before calling the route. Sessions last one
hour; an expired session shows a "reload to sign in" error.

With the service token (from `terraform output -raw takedown_client_id` and
`takedown_client_secret`):

```sh
curl -X POST "https://mol.la/admin/v1/links/SHORTCODE/takedown" \
  -H "CF-Access-Client-Id: $CLIENT_ID" \
  -H "CF-Access-Client-Secret: $CLIENT_SECRET" \
  -H "Content-Type: application/json" \
  -d '{"reason":"malware"}'
```

Soft-delete in D1, edge cache delete for the current colo, zone purge of the
URL when `CF_PURGE_TOKEN` is set (otherwise other colos serve the cached 302
for at most 60s). The `audit` table records actor, reason, and outcome.

`GET /admin/v1/audit` (same Access gate) returns the newest 50 audit events;
the operator page lists them under "Recent takedowns".

`GET /admin/v1/top` (same Access gate) returns the 20 most-clicked live
links with their destinations; the operator page lists them under "Top
links" so phishing targets stand out.

## Restore

D1 Time Travel keeps 7 days of history on the Workers Free plan (30 days on
Workers Paid), so a bad write found later than that cannot be rolled back:

```sh
npx wrangler d1 time-travel info molla-prod --env prod
npx wrangler d1 time-travel restore molla-prod --env prod --timestamp=2026-10-01T00:00:00Z
```

Restore is in place and immediate; take a bookmark first
(`d1 time-travel info`) so you can roll forward again.

## Regenerating the golden test vectors

`worker/test/golden.json` pins the Go implementation's output. To regenerate
it, check out the `aws` branch and run `go run` on a small program that
calls `core.NewPermuter`, `Permute`, `EncodeBase62`, `ValidateURL`, and
`ValidateAlias` with the inputs listed in the file, with the key
`molla-slice-1-fixed-test-key`.

## Regional failure

Workers run in every Cloudflare location; D1 has one primary location.
During a D1 primary outage, redirects already in a colo's edge cache keep
serving for up to 60s; everything else returns 503. There is no automated
failover and no claim of one.
