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

variable "enable_waf" {
  type        = bool
  description = "Production enables WAF; development disables it."
}

variable "domain_name" {
  type        = string
  description = "Public hostname. Empty skips custom ACM/Route53."
  default     = ""
}

variable "hosted_zone_id" {
  type    = string
  default = ""
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
