output "distribution_id" {
  value = aws_cloudfront_distribution.this.id
}

output "distribution_arn" {
  value = aws_cloudfront_distribution.this.arn
}

output "distribution_domain" {
  value = aws_cloudfront_distribution.this.domain_name
}

output "ui_bucket" {
  value = aws_s3_bucket.ui.id
}

output "waf_arn" {
  value = var.enable_waf ? aws_wafv2_web_acl.edge[0].arn : null
}
