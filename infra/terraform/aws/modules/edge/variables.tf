variable "name_prefix" {
  type = string
}

variable "kms_key_arn" {
  type = string
}

variable "api_gateway_id" {
  type = string
}

variable "api_gateway_stage" {
  type    = string
  default = "live"
}

variable "redirect_function_url" {
  type = string
}

variable "redirect_function_arn" {
  type = string
}

variable "domain_name" {
  type        = string
  description = "Public hostname. Empty skips custom ACM/Cloudflare DNS."
  default     = ""
}

variable "cloudflare_zone_id" {
  type        = string
  description = "Cloudflare zone for ACM validation and the apex CNAME. Required when domain_name is set."
  default     = ""
}

variable "origin_verify_secret" {
  type        = string
  description = "Shared secret Cloudflare adds as X-Origin-Verify; CloudFront rejects requests without it. Required when domain_name is set."
  sensitive   = true
  default     = ""
}

variable "log_retention_days" {
  type    = number
  default = 30
}

variable "alarm_actions" {
  type    = list(string)
  default = []
}

variable "tags" {
  type = map(string)
}
