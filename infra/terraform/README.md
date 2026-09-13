# Terraform

AWS is molla's first deployment target. Modules live under `aws/modules/{data,api,edge,analytics}` and are composed by `aws/envs/{dev,prod}` with encrypted remote state.

Validate from the repository root (no account, no plan/apply):

```sh
make tf-check
```

Lambda zip artifacts for `api`, `redirect`, `invalidate`, and `aggregate`:

```sh
make build-lambda
```

Writes `dist/*.zip` (`provided.al2023` / arm64). Environment roots default `artifact_dir` to checked-in placeholders so `validate` works without a build; deploy pipelines pass `dist`.

Production changes require reviewed `terraform plan` output, short-lived AWS credentials, and explicit approval. CI never runs plan or apply.
