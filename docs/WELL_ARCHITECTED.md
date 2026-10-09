# Architecture review

This local checklist evaluates molla on Cloudflare against the six pillars
of the AWS Well-Architected Framework, used here as a vendor-neutral rubric.
It is a design gate, not a certification.

## Launch review record

| Field | Value |
| --- | --- |
| Date | 2026-10-01 |
| Workload owner | platform |
| Accepted risks | Free-plan hard quota; single D1 primary; approximate click stats (see below) |

Review this file before production launch, after material architecture
changes, and at least quarterly while the service is active. Operator
steps: [RUNBOOK.md](RUNBOOK.md).

## Operational excellence

- Worker, D1 schema, cron, assets, and bindings are declared in `wrangler.jsonc`; zone configuration is Terraform. Pull requests validate without an account.
- Redirect, API, takedown, and cleanup responsibilities are separate handlers in one Worker.
- Logs are structured JSON via Workers Logs (3-day retention on the Free plan).
- Required: Cloudflare Notifications for Worker error rate and usage approaching limits, with a named responder.

## Security

- Takedown is behind Cloudflare Access and the Worker re-verifies the Access JWT, so a removed policy fails closed.
- Secrets live in Workers Secrets; nothing secret is in the repository or Terraform state except the Access service token output (sensitive).
- Security headers are set by the Worker and by a zone rule for static assets; HSTS preload is on.
- Public create is rate limited at the edge and in the Worker. User destinations are parsed but never fetched.
- Required: rotate the Access service token yearly (Terraform `duration`); scope the API token to the zone and account as listed in the runbook.

## Reliability

- D1 is authoritative; the edge cache is disposable and cache failures fall back to D1.
- Link creation and idempotency are one D1 batch (transaction).
- Deletes are versioned and retryable; cache invalidation failure is reported and audited.
- D1 Time Travel gives point-in-time restore with no setup: 7 days on the Free plan, 30 days on Workers Paid.
- Accepted: the Free plan's daily quota fails closed; the runbook defines the upgrade trigger.

## Performance efficiency

- Workers run at every edge location; the Cache API serves hot redirects for 60s per colo without touching D1.
- Generated IDs come from block leases, so a create costs one D1 write per 100 IDs plus the insert.
- Click counting runs after the response via `waitUntil`.

## Cost optimization

- Every service in use has a Free-plan tier: Workers, D1, Cache API, Static Assets, Access, Rate Limiting, Cron Triggers, Workers Logs, R2 (state).
- No per-service fixed cost; monthly spend is $0 within quota. The upgrade path is Workers Paid at $5/mo.
- Removed versus the AWS design: cache cluster, provisioned concurrency, KMS, API Gateway, CloudFront, click event stream.

## Sustainability

- No idle infrastructure; a single Worker bundle under 100 KB.
- TTL-driven cron purge removes expired replay records and purged links.

## Accepted risk

D1 has one primary location. A primary outage interrupts writes and
uncached redirects; edge-cached redirects continue for up to 60s. There is
no automated failover.

Click statistics are approximate. The counter increments after the
response is sent and is not retried if the isolate is torn down first.
They are not suitable for billing or contractual reporting.
