# Terraform

AWS is molla's first deployment target. Modules live under `aws/modules/{data,api,edge,analytics,ci}` and are composed by `aws/envs/{dev,prod}` with encrypted remote state. DNS for mol.la lives in Cloudflare (proxied); the `edge` module manages those records with the `cloudflare` provider, authenticated via `CLOUDFLARE_API_TOKEN`. The `ci` module (prod only) creates the OIDC IAM roles GitHub Actions assumes to plan and apply.

Validate from the repository root (no account, no plan/apply):

```sh
make tf-check
```

Lambda zip artifacts for `api`, `redirect`, `invalidate`, and `aggregate`:

```sh
make build-lambda
```

Writes `dist/*.zip` (`provided.al2023` / arm64). Environment roots default `artifact_dir` to checked-in placeholders so `validate` works without a build; deploy pipelines pass `dist`.

Outputs from `envs/dev` and `envs/prod` include `ui_bucket`, `distribution_id`, `api_invoke_url`, `usage_plan_id`, `api_key_id`, and `admin_role_arn`.

Apply, key seeding, UI sync, takedown, restore, and regional failure: `docs/RUNBOOK.md`.

`ci.yml` never plans or applies. Plan runs read-only in `terraform-plan.yml`
on every PR touching this directory; apply runs in `deploy.yml`, triggered
only by a `vX.Y.Z` tag push or a manual `workflow_dispatch` gated by a
required reviewer on the `production` GitHub Environment -- never by a
merge to `main`. Both authenticate via OIDC (`modules/ci`), so no AWS keys
are stored in GitHub. The very first apply still has to happen by hand,
since it creates those OIDC roles: see `docs/RUNBOOK.md`.
