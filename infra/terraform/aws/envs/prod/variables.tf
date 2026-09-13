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
