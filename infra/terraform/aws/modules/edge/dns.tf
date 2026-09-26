# mol.la is hosted and proxied by Cloudflare. Cloudflare terminates viewer TLS
# and runs the firewall; CloudFront is the origin behind it. Cloudflare flattens
# the apex CNAME automatically. ACM validation records must stay unproxied.

locals {
  cert_validation = var.domain_name == "" ? {} : {
    for dvo in aws_acm_certificate.this[0].domain_validation_options : dvo.domain_name => {
      name  = trimsuffix(dvo.resource_record_name, ".")
      type  = dvo.resource_record_type
      value = trimsuffix(dvo.resource_record_value, ".")
    }
  }
}

# CloudFront requires the certificate in us-east-1; the environment roots
# default the AWS provider to that region.
resource "aws_acm_certificate" "this" {
  count             = var.domain_name == "" ? 0 : 1
  domain_name       = var.domain_name
  validation_method = "DNS"
  tags              = var.tags
  lifecycle { create_before_destroy = true }
}

resource "cloudflare_dns_record" "cert_validation" {
  for_each = local.cert_validation
  zone_id  = var.cloudflare_zone_id
  name     = each.value.name
  type     = each.value.type
  content  = each.value.value
  ttl      = 60
  proxied  = false
}

resource "aws_acm_certificate_validation" "this" {
  count                   = var.domain_name == "" ? 0 : 1
  certificate_arn         = aws_acm_certificate.this[0].arn
  validation_record_fqdns = [for r in cloudflare_dns_record.cert_validation : r.name]
}

resource "cloudflare_dns_record" "apex" {
  count   = var.domain_name == "" ? 0 : 1
  zone_id = var.cloudflare_zone_id
  name    = var.domain_name
  type    = "CNAME"
  content = aws_cloudfront_distribution.this.domain_name
  ttl     = 1
  proxied = true
}

# Full (strict): Cloudflare validates the ACM certificate on CloudFront. Any
# weaker mode loops against CloudFront's redirect-to-https.
resource "cloudflare_zone_setting" "ssl" {
  count      = var.domain_name == "" ? 0 : 1
  zone_id    = var.cloudflare_zone_id
  setting_id = "ssl"
  value      = "strict"
}

# Cloudflare overwrites the origin's HSTS header with this one. The preload
# list requires a max-age of at least one year.
resource "cloudflare_zone_setting" "security_header" {
  count      = var.domain_name == "" ? 0 : 1
  zone_id    = var.cloudflare_zone_id
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

# Bot protection is off by choice. JavaScript detections inject an inline
# script that the site's CSP (script-src 'self') blocks, and Bot Fight Mode
# challenges scanners with a page carrying Cloudflare's own CSP. The API resets
# omitted fields on update, so every protection is pinned explicitly.
resource "cloudflare_bot_management" "this" {
  count                   = var.domain_name == "" ? 0 : 1
  zone_id                 = var.cloudflare_zone_id
  enable_js               = false
  fight_mode              = false
  ai_bots_protection      = "disabled"
  crawler_protection      = "disabled"
  content_bots_protection = "disabled"
}

# Cloudflare stamps every origin request with a shared secret; the CloudFront
# viewer-request function rejects requests without it, so traffic cannot bypass
# the Cloudflare firewall by hitting the *.cloudfront.net hostname directly.
resource "cloudflare_ruleset" "origin_verify" {
  count   = var.domain_name == "" ? 0 : 1
  zone_id = var.cloudflare_zone_id
  name    = "${var.name_prefix}-origin-verify"
  kind    = "zone"
  phase   = "http_request_late_transform"
  rules = [{
    description = "Stamp origin requests with the CloudFront shared secret"
    expression  = "true"
    enabled     = true
    action      = "rewrite"
    action_parameters = {
      headers = {
        "X-Origin-Verify" = {
          operation = "set"
          value     = var.origin_verify_secret
        }
      }
    }
  }]
}

# POST /api/v1/links is unauthenticated (no owner, no per-caller API key), so
# Cloudflare's per-IP rate limit is the abuse control on public link creation.
# API Gateway's stage throttle only bounds aggregate throughput, not one caller.
# The zone is on the Free plan, which allows one rule with a fixed 10 second
# period and 10 second mitigation timeout; 3 per 10s is roughly 10 per minute.
resource "cloudflare_ruleset" "api_rate_limit" {
  count   = var.domain_name == "" ? 0 : 1
  zone_id = var.cloudflare_zone_id
  name    = "${var.name_prefix}-api-rate-limit"
  kind    = "zone"
  phase   = "http_ratelimit"
  rules = [{
    description = "Rate-limit public link creation per client IP"
    expression  = "(http.request.uri.path eq \"/api/v1/links\" and http.request.method eq \"POST\")"
    enabled     = true
    action      = "block"
    ratelimit = {
      characteristics     = ["ip.src", "cf.colo.id"] # cf.colo.id is mandatory for rate-limit rules
      period              = 10
      requests_per_period = 3
      mitigation_timeout  = 10
    }
  }]
}

# www is not a CloudFront alias; Cloudflare answers it and redirects to the
# apex before anything reaches the origin. The record must stay proxied for
# the redirect rule to run.
resource "cloudflare_dns_record" "www" {
  count   = var.domain_name == "" ? 0 : 1
  zone_id = var.cloudflare_zone_id
  name    = "www.${var.domain_name}"
  type    = "CNAME"
  content = var.domain_name
  ttl     = 1
  proxied = true
}

# Single Redirects (the cloudflare_ruleset phase http_request_dynamic_redirect)
# needs a token permission this zone's token does not grant; a Page Rule
# achieves the same 301 with the Page Rules permission the token already has.
resource "cloudflare_page_rule" "www_redirect" {
  count    = var.domain_name == "" ? 0 : 1
  zone_id  = var.cloudflare_zone_id
  target   = "www.${var.domain_name}/*"
  priority = 1
  status   = "active"

  actions = {
    forwarding_url = {
      url         = "https://${var.domain_name}/$1"
      status_code = 301
    }
  }
}
