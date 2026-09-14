data "aws_region" "current" {}
data "aws_caller_identity" "current" {}

resource "aws_s3_bucket" "ui" {
  bucket = "${var.name_prefix}-ui-${data.aws_caller_identity.current.account_id}"
  tags   = var.tags
}

resource "aws_s3_bucket_public_access_block" "ui" {
  bucket                  = aws_s3_bucket.ui.id
  block_public_acls       = true
  block_public_policy     = true
  ignore_public_acls      = true
  restrict_public_buckets = true
}

resource "aws_s3_bucket_server_side_encryption_configuration" "ui" {
  bucket = aws_s3_bucket.ui.id
  rule {
    apply_server_side_encryption_by_default {
      sse_algorithm     = "aws:kms"
      kms_master_key_id = var.kms_key_arn
    }
    bucket_key_enabled = true
  }
}

resource "aws_s3_bucket_policy" "ui" {
  bucket = aws_s3_bucket.ui.id
  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Sid       = "DenyInsecureTransport"
        Effect    = "Deny"
        Principal = "*"
        Action    = "s3:*"
        Resource  = [aws_s3_bucket.ui.arn, "${aws_s3_bucket.ui.arn}/*"]
        Condition = { Bool = { "aws:SecureTransport" = "false" } }
      },
      {
        Sid       = "AllowCloudFrontOAC"
        Effect    = "Allow"
        Principal = { Service = "cloudfront.amazonaws.com" }
        Action    = "s3:GetObject"
        Resource  = "${aws_s3_bucket.ui.arn}/*"
        Condition = {
          StringEquals = { "AWS:SourceArn" = aws_cloudfront_distribution.this.arn }
        }
      }
    ]
  })
}

resource "aws_cloudfront_origin_access_control" "ui" {
  name                              = "${var.name_prefix}-ui"
  origin_access_control_origin_type = "s3"
  signing_behavior                  = "always"
  signing_protocol                  = "sigv4"
}

resource "aws_cloudfront_origin_access_control" "redirect" {
  name                              = "${var.name_prefix}-redirect"
  origin_access_control_origin_type = "lambda"
  signing_behavior                  = "always"
  signing_protocol                  = "sigv4"
}

resource "aws_cloudfront_cache_policy" "redirect" {
  name        = "${var.name_prefix}-redirect-5s"
  comment     = "Honor origin Cache-Control; cap at 5s. No stale-while-revalidate."
  default_ttl = 0
  max_ttl     = 5
  min_ttl     = 0
  parameters_in_cache_key_and_forwarded_to_origin {
    enable_accept_encoding_brotli = true
    enable_accept_encoding_gzip   = true
    cookies_config { cookie_behavior = "none" }
    headers_config { header_behavior = "none" }
    query_strings_config { query_string_behavior = "none" }
  }
}

resource "aws_cloudfront_cache_policy" "api" {
  name        = "${var.name_prefix}-api-no-cache"
  default_ttl = 0
  max_ttl     = 0
  min_ttl     = 0
  parameters_in_cache_key_and_forwarded_to_origin {
    cookies_config { cookie_behavior = "none" }
    headers_config { header_behavior = "none" }
    query_strings_config { query_string_behavior = "none" }
  }
}

# Forwarded to the origin but excluded from the cache key: forwarding it via
# the cache policy instead would fragment the 5s redirect cache per client IP.
resource "aws_cloudfront_origin_request_policy" "redirect" {
  name = "${var.name_prefix}-redirect-origin"
  cookies_config { cookie_behavior = "none" }
  headers_config {
    header_behavior = "whitelist"
    headers { items = ["CF-Connecting-IP"] }
  }
  query_strings_config { query_string_behavior = "none" }
}

resource "aws_cloudfront_origin_request_policy" "api" {
  name = "${var.name_prefix}-api-origin"
  cookies_config { cookie_behavior = "none" }
  headers_config {
    header_behavior = "whitelist"
    headers { items = ["X-Api-Key", "Idempotency-Key", "Content-Type", "Origin"] }
  }
  query_strings_config { query_string_behavior = "all" }
}

resource "aws_cloudfront_response_headers_policy" "security" {
  name = "${var.name_prefix}-security"
  security_headers_config {
    content_type_options { override = true }
    frame_options {
      frame_option = "DENY"
      override     = true
    }
    referrer_policy {
      referrer_policy = "no-referrer"
      override        = true
    }
    strict_transport_security {
      access_control_max_age_sec = 31536000
      override                   = true
    }
  }
}

resource "aws_cloudfront_function" "viewer_request" {
  count   = var.domain_name == "" ? 0 : 1
  name    = "${var.name_prefix}-viewer-request"
  runtime = "cloudfront-js-2.0"
  comment = "Reject requests that did not come through Cloudflare."
  publish = true
  code    = templatefile("${path.module}/viewer_request.js.tftpl", { secret = var.origin_verify_secret })
}

# One viewer-request function per behavior is allowed, and /app/* needs both the
# Cloudflare origin check and the SPA rewrite, so they are combined here.
resource "aws_cloudfront_function" "spa_rewrite" {
  name    = "${var.name_prefix}-spa-rewrite"
  runtime = "cloudfront-js-2.0"
  comment = "Serve the SPA shell for /app deep links."
  publish = true
  code = templatefile("${path.module}/spa_rewrite.js.tftpl", {
    verify = var.domain_name == "" ? "" : local.origin_verify_js
  })
}

locals {
  origin_verify_js = <<-EOT
      var header = request.headers['x-origin-verify'];
      if (!header || header.value !== '${var.origin_verify_secret}') {
        return {
          statusCode: 403,
          statusDescription: 'Forbidden',
          headers: { 'cache-control': { value: 'no-store' } }
        };
      }
      delete request.headers['x-origin-verify'];
  EOT

  redirect_domain = replace(replace(var.redirect_function_url, "https://", ""), "/", "")
  api_domain      = "${var.api_gateway_id}.execute-api.${data.aws_region.current.region}.amazonaws.com"
}

resource "aws_cloudfront_distribution" "this" {
  enabled             = true
  is_ipv6_enabled     = true
  comment             = var.name_prefix
  price_class         = "PriceClass_100"
  aliases             = var.domain_name == "" ? [] : [var.domain_name]
  default_root_object = ""
  tags                = var.tags

  origin {
    domain_name              = local.redirect_domain
    origin_id                = "redirect"
    origin_access_control_id = aws_cloudfront_origin_access_control.redirect.id
    custom_origin_config {
      http_port              = 80
      https_port             = 443
      origin_protocol_policy = "https-only"
      origin_ssl_protocols   = ["TLSv1.2"]
    }
  }

  origin {
    domain_name = local.api_domain
    origin_id   = "api"
    origin_path = "/${var.api_gateway_stage}"
    custom_origin_config {
      http_port              = 80
      https_port             = 443
      origin_protocol_policy = "https-only"
      origin_ssl_protocols   = ["TLSv1.2"]
    }
  }

  origin {
    domain_name              = aws_s3_bucket.ui.bucket_regional_domain_name
    origin_id                = "ui"
    origin_access_control_id = aws_cloudfront_origin_access_control.ui.id
  }

  default_cache_behavior {
    target_origin_id           = "redirect"
    viewer_protocol_policy     = "redirect-to-https"
    allowed_methods            = ["GET", "HEAD"]
    cached_methods             = ["GET", "HEAD"]
    compress                   = true
    cache_policy_id            = aws_cloudfront_cache_policy.redirect.id
    origin_request_policy_id   = aws_cloudfront_origin_request_policy.redirect.id
    response_headers_policy_id = aws_cloudfront_response_headers_policy.security.id
    dynamic "function_association" {
      for_each = aws_cloudfront_function.viewer_request
      content {
        event_type   = "viewer-request"
        function_arn = function_association.value.arn
      }
    }
  }

  ordered_cache_behavior {
    path_pattern               = "/api/*"
    target_origin_id           = "api"
    viewer_protocol_policy     = "https-only"
    allowed_methods            = ["GET", "HEAD", "OPTIONS", "PUT", "POST", "PATCH", "DELETE"]
    cached_methods             = ["GET", "HEAD"]
    compress                   = true
    cache_policy_id            = aws_cloudfront_cache_policy.api.id
    origin_request_policy_id   = aws_cloudfront_origin_request_policy.api.id
    response_headers_policy_id = aws_cloudfront_response_headers_policy.security.id
    dynamic "function_association" {
      for_each = aws_cloudfront_function.viewer_request
      content {
        event_type   = "viewer-request"
        function_arn = function_association.value.arn
      }
    }
  }

  ordered_cache_behavior {
    path_pattern               = "/app/*"
    target_origin_id           = "ui"
    viewer_protocol_policy     = "redirect-to-https"
    allowed_methods            = ["GET", "HEAD"]
    cached_methods             = ["GET", "HEAD"]
    compress                   = true
    cache_policy_id            = aws_cloudfront_cache_policy.redirect.id
    response_headers_policy_id = aws_cloudfront_response_headers_policy.security.id
    function_association {
      event_type   = "viewer-request"
      function_arn = aws_cloudfront_function.spa_rewrite.arn
    }
  }

  # Root path serves the landing page (create-a-link UI), not the redirect
  # Lambda; any other bare path (a real or unknown short code) still falls
  # through to the default behavior. spa_rewrite treats "/" the same as any
  # other extensionless path and rewrites it to /app/index.html.
  ordered_cache_behavior {
    path_pattern               = "/"
    target_origin_id           = "ui"
    viewer_protocol_policy     = "redirect-to-https"
    allowed_methods            = ["GET", "HEAD"]
    cached_methods             = ["GET", "HEAD"]
    compress                   = true
    cache_policy_id            = aws_cloudfront_cache_policy.redirect.id
    response_headers_policy_id = aws_cloudfront_response_headers_policy.security.id
    function_association {
      event_type   = "viewer-request"
      function_arn = aws_cloudfront_function.spa_rewrite.arn
    }
  }

  restrictions {
    geo_restriction { restriction_type = "none" }
  }

  viewer_certificate {
    cloudfront_default_certificate = var.domain_name == ""
    acm_certificate_arn            = var.domain_name == "" ? null : aws_acm_certificate.this[0].arn
    ssl_support_method             = var.domain_name == "" ? null : "sni-only"
    # With the default CloudFront certificate AWS forces TLSv1 and silently
    # ignores anything stricter, which otherwise shows as drift on every plan.
    minimum_protocol_version = var.domain_name == "" ? "TLSv1" : "TLSv1.2_2021"
  }
}

resource "aws_lambda_permission" "cloudfront_redirect" {
  statement_id  = "AllowCloudFrontOAC"
  action        = "lambda:InvokeFunctionUrl"
  function_name = var.redirect_function_arn
  qualifier     = "live"
  principal     = "cloudfront.amazonaws.com"
  source_arn    = aws_cloudfront_distribution.this.arn
}

# Function URLs created since October 2025 require both
# lambda:InvokeFunctionUrl and lambda:InvokeFunction to be granted.
resource "aws_lambda_permission" "cloudfront_redirect_invoke" {
  statement_id             = "AllowCloudFrontOACInvoke"
  action                   = "lambda:InvokeFunction"
  function_name            = var.redirect_function_arn
  qualifier                = "live"
  principal                = "cloudfront.amazonaws.com"
  source_arn               = aws_cloudfront_distribution.this.arn
  invoked_via_function_url = true
}

resource "aws_cloudwatch_log_group" "cf" {
  name              = "/aws/cloudfront/${var.name_prefix}"
  retention_in_days = var.log_retention_days
  kms_key_id        = var.kms_key_arn
  tags              = var.tags
}

resource "aws_cloudwatch_metric_alarm" "redirect_5xx" {
  alarm_name          = "${var.name_prefix}-redirect-5xx"
  alarm_description   = "Owner: platform. Action: inspect origin 5xx and Lambda errors. SLO 99.9%."
  comparison_operator = "GreaterThanThreshold"
  evaluation_periods  = 5
  metric_name         = "5xxErrorRate"
  namespace           = "AWS/CloudFront"
  period              = 60
  statistic           = "Average"
  threshold           = 0.1
  alarm_actions       = var.alarm_actions
  dimensions          = { DistributionId = aws_cloudfront_distribution.this.id }
  tags                = var.tags
}

resource "aws_cloudwatch_metric_alarm" "origin_latency" {
  alarm_name          = "${var.name_prefix}-origin-latency"
  alarm_description   = "Owner: platform. Action: check redirect p99 < 50ms at edge."
  comparison_operator = "GreaterThanThreshold"
  evaluation_periods  = 5
  metric_name         = "OriginLatency"
  namespace           = "AWS/CloudFront"
  period              = 60
  extended_statistic  = "p99"
  threshold           = 50
  alarm_actions       = var.alarm_actions
  dimensions          = { DistributionId = aws_cloudfront_distribution.this.id }
  tags                = var.tags
}
