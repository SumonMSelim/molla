variable "account_id" {
  type        = string
  description = "Cloudflare account that owns the zone and the Worker."
}

variable "zone_id" {
  type        = string
  description = "Cloudflare zone for the public hostname."
}

variable "domain_name" {
  type        = string
  description = "Public hostname served by the Worker route."
  default     = "mol.la"
}

variable "access_team_name" {
  type        = string
  description = "Zero Trust team name; the Access issuer is https://<team>.cloudflareaccess.com."
}

variable "operator_emails" {
  type        = list(string)
  description = "Identities allowed to call the takedown route interactively, in addition to the service token."
  default     = []
}

variable "name_prefix" {
  type    = string
  default = "molla"
}
