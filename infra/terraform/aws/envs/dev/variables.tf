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
