variable "name_prefix" {
  type = string
}

variable "github_repository" {
  type        = string
  description = "GitHub repo as owner/name, e.g. SumonMSelim/molla."
}

variable "github_environment" {
  type        = string
  description = "GitHub Environment name gating deploys (workflow_dispatch and tag pushes both run under it)."
  default     = "production"
}

variable "github_plan_environment" {
  type        = string
  description = "GitHub Environment name for the read-only plan role. Separate from github_environment because that one only accepts v*.*.* tags, which would block every PR's automatic plan check."
  default     = "production-plan"
}

variable "kms_key_arn" {
  type        = string
  description = "Workload KMS key. The plan role needs kms:Decrypt on it to refresh SSM SecureString parameters."
}

variable "tags" {
  type = map(string)
}
