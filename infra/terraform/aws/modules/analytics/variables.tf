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

variable "clicks_stream_arn" {
  type        = string
  description = "DynamoDB Stream ARN of the Clicks table (modules/data), the aggregate Lambda's event source."
}

variable "log_retention_days" {
  type    = number
  default = 30
}

variable "aggregate_reserved_concurrency" {
  type        = number
  description = "Reserved concurrency for the aggregate Lambda. 0 leaves it unset (shares the account's unreserved pool), which is what a fresh account's default 10 total concurrent executions requires; prod sizes 20 for tested peak once the account limit is raised."
  default     = 0
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
