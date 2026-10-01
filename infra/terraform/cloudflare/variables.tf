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
  default = "molla-prod"
}

variable "manage_apex" {
  type        = bool
  description = "Create the proxied apex placeholder record. False while the apex still points at the previous deployment."
  default     = true
}

variable "access_hostnames" {
  type        = list(string)
  description = "Hostnames whose /admin/ path the Access application protects. Defaults to the domain."
  default     = []
}
