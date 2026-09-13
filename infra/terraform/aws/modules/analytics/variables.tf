variable "name_prefix" {
  type = string
}

variable "kms_key_arn" {
  type = string
}

variable "artifact_dir" {
  type        = string
  description = "Directory containing aggregate.zip from make build-lambda."
}

variable "stats_table_arn" {
  type = string
}

variable "stats_table_name" {
  type = string
}

variable "stream_mode" {
  type        = string
  description = "Kinesis capacity mode. ON_DEMAND bills per use with no idle shard-hour cost (cheapest at low/unknown volume); PROVISIONED needs shard_count sized for tested peak."
  default     = "ON_DEMAND"
  validation {
    condition     = contains(["ON_DEMAND", "PROVISIONED"], var.stream_mode)
    error_message = "stream_mode must be ON_DEMAND or PROVISIONED."
  }
}

variable "shard_count" {
  type        = number
  description = "Kinesis shards. Only used when stream_mode is PROVISIONED; prod sizes ~16 with 20% headroom over tested peak."
  default     = 1
}

variable "log_retention_days" {
  type    = number
  default = 30
}

variable "alarm_actions" {
  type    = list(string)
  default = []
}

variable "subnet_ids" {
  type        = list(string)
  description = "Unused; aggregator stays out of VPC (DynamoDB only)."
  default     = []
}

variable "tags" {
  type = map(string)
}
