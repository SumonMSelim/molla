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
