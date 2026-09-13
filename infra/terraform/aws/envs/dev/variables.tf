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

variable "artifact_dir" {
  type    = string
  default = "../../placeholders"
}

variable "domain_name" {
  type        = string
  description = "Public hostname. Empty uses the CloudFront domain only."
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
