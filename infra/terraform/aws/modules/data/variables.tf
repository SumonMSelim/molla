variable "name_prefix" {
  type        = string
  description = "Prefix for table names (e.g. molla-prod)."
}

variable "enable_pitr" {
  type        = bool
  description = "Enable DynamoDB point-in-time recovery (production)."
  default     = false
}

variable "kms_key_arn" {
  type        = string
  description = "CMK ARN for table encryption."
}

variable "log_retention_days" {
  type        = number
  description = "CloudWatch log retention for table-related alarms."
  default     = 30
}

variable "alarm_actions" {
  type        = list(string)
  description = "SNS topic ARNs for throttle alarms."
  default     = []
}

variable "tags" {
  type        = map(string)
  description = "Workload, environment, owner, and cost-allocation tags."
}
