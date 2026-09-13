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

variable "tags" {
  type = map(string)
}
