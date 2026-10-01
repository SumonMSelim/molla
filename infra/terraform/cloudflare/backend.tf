# State lives in a Cloudflare R2 bucket through the S3-compatible backend.
# Bucket, endpoint, and credentials are supplied at init time (see
# docs/RUNBOOK.md) so no account identifier is committed here:
#
#   terraform init -backend-config=bucket=... -backend-config=endpoints={s3=...}
#
# R2 supports conditional writes, which is what use_lockfile relies on.
terraform {
  backend "s3" {
    key                         = "molla/cloudflare/terraform.tfstate"
    region                      = "auto"
    use_lockfile                = true
    use_path_style              = true
    skip_credentials_validation = true
    skip_region_validation      = true
    skip_requesting_account_id  = true
    skip_metadata_api_check     = true
    skip_s3_checksum            = true
  }
}
