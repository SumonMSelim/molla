variable "region" {
  type    = string
  default = "us-east-1"
}

variable "permutation_key" {
  type      = string
  sensitive = true
}

variable "privacy_key" {
  type      = string
  sensitive = true
}

variable "redis_auth_token" {
  type      = string
  sensitive = true
}

variable "admin_principal_arns" {
  type = list(string)
}

# No default: prod must be given real build artifacts (placeholders are dev/validate only).
variable "artifact_dir" {
  type = string
}

variable "budget_limit" {
  type    = string
  default = "20000"
}

variable "alarm_email" {
  type = string
}

# Centrally managed account controls (not created in this workload).
variable "central_cloudtrail_arn" {
  type    = string
  default = ""
}

variable "central_config_recorder_arn" {
  type    = string
  default = ""
}

variable "central_guardduty_detector_id" {
  type    = string
  default = ""
}

variable "central_security_hub_arn" {
  type    = string
  default = ""
}

variable "central_access_analyzer_arn" {
  type    = string
  default = ""
}

variable "domain_name" {
  type        = string
  description = "Public hostname (mol.la). Empty uses the CloudFront domain only."
  default     = ""
}

variable "cloudflare_zone_id" {
  type        = string
  description = "Cloudflare zone for ACM validation and the apex CNAME. Required when domain_name is set."
  default     = ""
}

variable "origin_verify_secret" {
  type        = string
  description = "Shared secret Cloudflare stamps on origin requests. Required when domain_name is set."
  sensitive   = true
  default     = ""
}
