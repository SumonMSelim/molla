# Zone-level configuration for the Worker deployment. The Worker itself, its
# D1 database, cron trigger, static assets, and secrets are managed by
# wrangler (worker/wrangler.jsonc); this configuration owns everything the
# zone needs around it: DNS, firewall rules, headers, and operator access.

locals {
  takedown_path    = "/admin/"
  access_hostnames = length(var.access_hostnames) == 0 ? [var.domain_name] : var.access_hostnames
}

# A proxied placeholder record is required for the Worker route to receive
# traffic; the Worker answers every request so the target never serves.
resource "cloudflare_dns_record" "apex" {
  count   = var.manage_apex ? 1 : 0
  zone_id = var.zone_id
  name    = var.domain_name
  type    = "AAAA"
  content = "100::"
  ttl     = 1
  proxied = true
}

resource "cloudflare_dns_record" "www" {
  zone_id = var.zone_id
  name    = "www.${var.domain_name}"
  type    = "CNAME"
  content = var.domain_name
  ttl     = 1
  proxied = true
}

# www redirects to the apex with a Single Redirect rule. The legacy Page Rules
# API rejects account-owned API tokens, so it is not used.
resource "cloudflare_ruleset" "www_redirect" {
  zone_id = var.zone_id
  name    = "${var.name_prefix}-www-redirect"
  kind    = "zone"
  phase   = "http_request_dynamic_redirect"
  rules = [{
    description = "Redirect www to the apex"
    expression  = "(http.host eq \"www.${var.domain_name}\")"
    enabled     = true
    action      = "redirect"
    action_parameters = {
      from_value = {
        status_code           = 301
        preserve_query_string = true
        target_url = {
          expression = "concat(\"https://${var.domain_name}\", http.request.uri.path)"
        }
      }
    }
  }]
}

# Respect the Worker's own Cache-Control. The zone default (4 hours)
# overwrites the redirect's max-age=5, which would let browsers keep serving
# a taken-down link for hours.
resource "cloudflare_zone_setting" "browser_cache_ttl" {
  zone_id    = var.zone_id
  setting_id = "browser_cache_ttl"
  value      = 0
}

# The preload list requires a max-age of at least one year.
resource "cloudflare_zone_setting" "security_header" {
  zone_id    = var.zone_id
  setting_id = "security_header"
  value = {
    strict_transport_security = {
      enabled            = true
      max_age            = 31536000
      include_subdomains = true
      preload            = true
      nosniff            = true
    }
  }
}

# Bot protection stays off: its injected script violates the SPA's CSP
# (script-src 'self'). Every field is pinned because the API resets omitted
# ones on update.
resource "cloudflare_bot_management" "this" {
  zone_id                 = var.zone_id
  enable_js               = false
  fight_mode              = false
  ai_bots_protection      = "disabled"
  crawler_protection      = "disabled"
  content_bots_protection = "disabled"
}

# Static assets are served before the Worker runs, so they would otherwise
# miss the security headers the Worker sets on its own responses. This zone
# rule stamps the same set on every response, asset or Worker. It is scoped to
# this host: the zone also serves other apps whose own headers must not be
# overridden by this CSP.
resource "cloudflare_ruleset" "response_headers" {
  zone_id = var.zone_id
  name    = "${var.name_prefix}-response-headers"
  kind    = "zone"
  phase   = "http_response_headers_transform"
  rules = [{
    description = "Security headers on every response"
    expression  = "(http.host eq \"${var.domain_name}\")"
    enabled     = true
    action      = "rewrite"
    action_parameters = {
      headers = {
        "X-Content-Type-Options" = { operation = "set", value = "nosniff" }
        "X-Frame-Options"        = { operation = "set", value = "DENY" }
        "Referrer-Policy"        = { operation = "set", value = "no-referrer" }
        "Content-Security-Policy" = {
          operation = "set"
          value     = "default-src 'self'; script-src 'self' https://static.cloudflareinsights.com; style-src 'self'; connect-src 'self' https://cloudflareinsights.com; img-src 'self'; font-src 'self'; base-uri 'self'; form-action 'self'; frame-ancestors 'none'; object-src 'none'"
        }
      }
    }
  }]
}

# POST /api/v1/links is unauthenticated, so this per-IP limit is the outer
# abuse control on public link creation; the Worker's rate-limit binding is
# the finer inner layer. The Free plan allows one rule with a fixed 10 second
# period and 10 second mitigation timeout; 3 per 10s is roughly 10 per minute.
resource "cloudflare_ruleset" "api_rate_limit" {
  zone_id = var.zone_id
  name    = "${var.name_prefix}-api-rate-limit"
  kind    = "zone"
  phase   = "http_ratelimit"
  rules = [{
    description = "Rate-limit public link creation per client IP"
    expression  = "(http.host eq \"${var.domain_name}\" and http.request.uri.path eq \"/api/v1/links\" and http.request.method eq \"POST\")"
    enabled     = true
    action      = "block"
    ratelimit = {
      characteristics     = ["ip.src", "cf.colo.id"]
      period              = 10
      requests_per_period = 3
      mitigation_timeout  = 10
    }
  }]
}

# Operator takedown: /admin/* is a self-hosted Access application. Access
# blocks anything without a valid session or service token at the edge, and
# the Worker verifies the resulting JWT again (worker/src/access.ts).
resource "cloudflare_zero_trust_access_service_token" "takedown" {
  account_id = var.account_id
  name       = "${var.name_prefix}-takedown"
  duration   = "8760h"
}

resource "cloudflare_zero_trust_access_policy" "service_token" {
  account_id = var.account_id
  name       = "${var.name_prefix}-takedown-service-token"
  decision   = "non_identity"
  include = [{
    service_token = { token_id = cloudflare_zero_trust_access_service_token.takedown.id }
  }]
}

resource "cloudflare_zero_trust_access_policy" "operators" {
  count      = length(var.operator_emails) == 0 ? 0 : 1
  account_id = var.account_id
  name       = "${var.name_prefix}-takedown-operators"
  decision   = "allow"
  include    = [for email in var.operator_emails : { email = { email = email } }]
}

resource "cloudflare_zero_trust_access_application" "takedown" {
  account_id       = var.account_id
  name             = "${var.name_prefix}-takedown"
  type             = "self_hosted"
  session_duration = "1h"
  destinations = [for host in local.access_hostnames : {
    type = "public"
    uri  = "${host}${local.takedown_path}"
  }]
  policies = concat(
    [{ id = cloudflare_zero_trust_access_policy.service_token.id, precedence = 1 }],
    [for p in cloudflare_zero_trust_access_policy.operators : { id = p.id, precedence = 2 }],
  )
}
