variable "name_prefix" {
  type = string
}

variable "kms_key_arn" {
  type = string
}

variable "artifact_dir" {
  type = string
}

variable "table_arns" {
  type        = map(string)
  description = "Map of logical table keys to ARNs from the data module."
}

variable "table_names" {
  type = map(string)
}

variable "stream_arn" {
  type = string
}

variable "stream_name" {
  type = string
}

variable "vpc_cidr" {
  type    = string
  default = "10.40.0.0/16"
}

variable "redis_node_type" {
  type    = string
  default = "cache.t4g.micro"
}

variable "redis_cluster_count" {
  type        = number
  description = "ElastiCache nodes. 1 = no replica/failover (cheapest); 2+ enables automatic_failover_enabled and multi_az_enabled."
  default     = 1
}

variable "redis_auth_token" {
  type      = string
  sensitive = true
}

variable "permutation_key" {
  type      = string
  sensitive = true
}

variable "privacy_key" {
  type      = string
  sensitive = true
}

variable "public_base" {
  type    = string
  default = "https://mol.la"
}

variable "provisioned_concurrency" {
  type        = number
  description = "Redirect alias provisioned concurrency. 0 disables it entirely (cold starts, no idle cost); size for tested peak (e.g. 160 * 1.2) once real traffic exists."
  default     = 0
}

variable "throttle_rate_limit" {
  type        = number
  description = "Stage-wide steady request rate. Prod: 185 (154 peak * 1.2)."
  default     = 20
}

variable "throttle_burst_limit" {
  type        = number
  description = "Stage-wide burst capacity. Prod: 370 (2x steady rate)."
  default     = 40
}

variable "log_retention_days" {
  type    = number
  default = 30
}

variable "alarm_actions" {
  type    = list(string)
  default = []
}

variable "admin_principal_arns" {
  type        = list(string)
  description = "IAM principals allowed to assume the operator admin role."
}

variable "tags" {
  type = map(string)
}
