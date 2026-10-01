output "access_aud" {
  description = "Set as ACCESS_AUD in worker/wrangler.jsonc (env.prod.vars)."
  value       = cloudflare_zero_trust_access_application.takedown.aud
}

output "access_team_domain" {
  description = "Set as ACCESS_TEAM_DOMAIN in worker/wrangler.jsonc (env.prod.vars)."
  value       = "${var.access_team_name}.cloudflareaccess.com"
}

output "takedown_client_id" {
  value     = cloudflare_zero_trust_access_service_token.takedown.client_id
  sensitive = true
}

output "takedown_client_secret" {
  value     = cloudflare_zero_trust_access_service_token.takedown.client_secret
  sensitive = true
}
