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

variable "shard_count" {
  type        = number
  description = "Kinesis shards. Prod sizes ~16 with 20% headroom over tested peak."
  default     = 2
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
