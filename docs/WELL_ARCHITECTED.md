# AWS Well-Architected review

This local checklist evaluates molla against the six AWS Well-Architected Framework pillars and the Serverless Applications Lens. It is a design gate, not a claim of AWS certification.

## Launch review record

| Field | Value |
| --- | --- |
| Date | 2026-09-13 |
| Workload owner | platform |
| Security reviewer | platform (assign named reviewer before prod apply) |
| AWS Well-Architected Tool | record workload ID after the first Tool review; not a substitute for this file |
| Accepted risks | single-region launch; approximate click stats (see below) |

Review this file before production launch, after material architecture changes, and at least quarterly while the service is active. Record unresolved high-risk issues in the AWS Well-Architected Tool with an owner and target date. Operator steps: [RUNBOOK.md](RUNBOOK.md).

## Operational excellence

Aligned design:

- Infrastructure is defined in Terraform; pull requests validate without production credentials.
- API, redirect, invalidation, and analytics responsibilities are isolated.
- Application and audit logs are structured and correlated.

Required before production:

- Define workload owner, escalation path, service-level indicators, dashboards, and alarm actions.
- Add deployment runbooks with reviewed plan, canary or weighted Lambda alias rollout, automatic alarm-based rollback, and rollback verification.
- Make operational changes small, reversible, and observable.
- Run game days for Redis failover, DynamoDB throttling, Kinesis lag, invalidation failure, expired credentials, and restore procedures.
- Track post-incident actions and feed them back into design and runbooks.

## Security

Aligned design:

- Go authenticates hashed developer credentials and enforces ownership.
- API and redirect paths have separate roles and scaling controls.
- Storage, streams, cache traffic, logs, artifacts, and state require encryption.
- Public redirect traffic is protected by Cloudflare (proxy, WAF, rate limiting) in front of CloudFront, which rejects requests that bypass Cloudflare; API usage plans constrain developers.
- User destinations are parsed but never fetched synchronously.

Required before production:

- Use separate AWS accounts for production and non-production under AWS Organizations where available.
- Use short-lived federated operator access with MFA; prohibit long-lived deployment access keys.
- Enable organization/account CloudTrail, AWS Config, GuardDuty, Security Hub, WAF logging, and actionable findings routing.
- Enforce least-privilege IAM with Access Analyzer validation and explicit resource scopes.
- Inventory and rotate developer, Redis, permutation, and privacy-hashing secrets.
- Test credential revocation, operator takedown, and cache-tombstone propagation.
- Add dependency, secret, and Terraform security scanning to pull-request CI.

## Reliability

Aligned design:

- Managed regional services provide Multi-AZ resilience; Redis has automatic failover.
- DynamoDB is authoritative; Redis is disposable and cache failures fall back safely.
- Link creation and idempotency are one DynamoDB transaction.
- Deletes are durable, versioned, retryable, and report failure until cache invalidation succeeds.
- DynamoDB PITR and explicit RPO/RTO targets protect durable data.

Required before production:

- Enumerate Lambda concurrency, API Gateway, DynamoDB, Kinesis, VPC ENI, and WAF quotas; maintain at least 20% peak headroom and alarm on consumption.
- Configure bounded retries with jitter, timeouts shorter than caller budgets, reserved concurrency where isolation is required, and dead-letter/on-failure destinations for asynchronous consumers.
- Verify partial-batch failure handling for Kinesis consumers.
- Run sustained peak and burst load tests, including cache-cold and dependency-degraded cases.
- Automate periodic DynamoDB restore tests and verify data integrity plus measured RPO/RTO.
- Document the accepted regional-outage risk and recovery communication without implying automated regional failover.

## Performance efficiency

Aligned design:

- CloudFront and Redis absorb hot-link reads before DynamoDB.
- Generated IDs avoid a high-frequency central coordinator.
- Analytics writes are aggregated away from the redirect path.
- Go Lambda functions target arm64 and reuse SDK/cache clients across invocations.

Required before production:

- Benchmark code generation, cache serialization, redirect handling, and aggregation.
- Load-test p50, p95, and p99 latency at expected peak and burst traffic.
- Tune Lambda memory and provisioned concurrency using measured results and Lambda Power Tuning or Compute Optimizer.
- Measure CloudFront, Redis, and DynamoDB hit rates; replace design assumptions with production telemetry.
- Alarm on latency, throttles, errors, concurrency saturation, cache evictions, and Kinesis iterator age.

## Cost optimization

Aligned design:

- Serverless and on-demand services avoid idle application capacity at launch.
- Function URL avoids API Gateway request charges on the redirect path.
- Data lifecycle policies bound idempotency, logs, analytics, and deleted-link storage.
- Costly controls such as a second WAF layer are explicit design decisions; Cloudflare provides the firewall.

Required before production:

- Apply mandatory workload, environment, owner, and cost-center tags where supported.
- Configure AWS Budgets and Cost Anomaly Detection with named responders.
- Validate estimates with the AWS Pricing Calculator using measured payload sizes and request counts.
- Review CloudFront price class or flat-rate plans, WAF request charges, Kinesis mode/shards, log volume, provisioned concurrency, and Redis node size.
- Reassess DynamoDB on-demand versus provisioned capacity after stable traffic exists.

## Sustainability

Aligned design:

- Managed serverless services and arm64 compute reduce idle infrastructure.
- Caching and batched analytics reduce repeated compute and storage operations.
- TTL and lifecycle policies remove expired data.

Required before production:

- Right-size Lambda memory, concurrency, Redis, and stream capacity from measurements.
- Avoid duplicate telemetry and retain only fields required for operation, security, or product behavior.
- Prefer efficient serialization and batched network/storage operations.
- Review utilization quarterly and remove unused environments, alarms, streams, buckets, and retained artifacts.

## Accepted risk

The first release has one active AWS region. A regional outage can interrupt writes and redirects after edge entries expire. This prevents claiming regional fault tolerance; it does not waive Multi-AZ design, backup testing, quota management, fault injection, or operational readiness requirements.

Click statistics are approximate. Edge-cache hits can undercount and rare at-least-once processing retries can overcount. They are not suitable for billing or contractual reporting without a later reconciliation design.
