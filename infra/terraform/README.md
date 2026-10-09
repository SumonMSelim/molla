# Terraform

Zone-level configuration for mol.la on Cloudflare lives in `cloudflare/`. The Worker itself (code, D1 binding, cron trigger, static assets, rate-limit binding, secrets) is declared in `worker/wrangler.jsonc` and deployed with Wrangler, not Terraform.

`cloudflare/` manages:

- **DNS**: a proxied `AAAA 100::` placeholder on the apex (the Worker route answers every request; set `manage_apex = false` while the apex still points elsewhere) and a proxied `www` record.
- **`www` redirect**: a Single Redirect rule to the apex. Page Rules are not used because that API rejects account-owned tokens.
- **Zone settings**: `browser_cache_ttl = 0` so the Worker's `Cache-Control` is honoured, HSTS (one year, subdomains, preload) and `nosniff`.
- **Bot management**: pinned off, because its injected script violates the CSP (`script-src 'self'`).
- **Response headers**: a Transform Rule that stamps the security headers and CSP on static asset responses, which never pass through the Worker. Scoped to `http.host eq "<domain_name>"` so other apps on the zone keep their own headers.
- **Create rate limit**: a WAF rule, 3 `POST /api/v1/links` per 10 s per IP and data center, blocked before the Worker runs.
- **Access**: the self-hosted application on `<domain_name>/admin/`, its email and service-token policies, and the service token.

Cloudflare allows one zone ruleset per phase, so a new rule joins the existing ruleset for its phase instead of adding a second one.

## Inputs and outputs

| Variable | Purpose |
| --- | --- |
| `account_id` | Account that owns the zone and the Worker |
| `zone_id` | Zone for the public hostname |
| `domain_name` | Public hostname served by the Worker route |
| `access_team_name` | Zero Trust team name |
| `operator_emails` | Identities allowed through Access interactively |
| `manage_apex` | Create the apex placeholder record |
| `access_hostnames` | Hostnames whose `/admin/` path Access protects (defaults to the domain) |

Outputs `access_aud` and `access_team_domain` go into `env.prod.vars` in `worker/wrangler.jsonc`. `takedown_client_id` and `takedown_client_secret` are the service token for scripted takedowns (sensitive).

## State

State is stored in a Cloudflare R2 bucket through the S3-compatible backend, with `use_lockfile = true`. The bucket, endpoint and R2 credentials are passed at `terraform init` time, so no account identifier is committed. See `docs/RUNBOOK.md`.

## Validate, plan, apply

Validate from the repository root (no account, no backend):

```sh
make tf-check
```

`ci.yml` never plans or applies. Plan runs read-only in `terraform-plan.yml`
on every PR touching `cloudflare/` and posts the diff as a PR comment; apply
runs in `deploy.yml`, triggered only by a `vX.Y.Z` tag push or a manual
`workflow_dispatch`, under the `production` GitHub Environment (tags
`v*.*.*` only, no required reviewer), never by a merge to `main`.
Cloudflare has no OIDC federation, so both workflows use API tokens stored
as GitHub Environment secrets: `production-plan` holds a read-only token and
an R2 read-only key, `production` the write-scoped token. The very first
apply has to happen by hand: see `docs/RUNBOOK.md`.
