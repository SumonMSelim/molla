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
  description = "GitHub Environment name for the read-only plan role. Separate from github_environment so a required reviewer on deploys doesn't also gate every PR's automatic plan check."
  default     = "production-plan"
}

variable "tags" {
  type = map(string)
}
