resource "aws_wafv2_web_acl" "edge" {
  count = var.enable_waf ? 1 : 0
  name  = "${var.name_prefix}-edge"
  scope = "CLOUDFRONT"
  default_action {
    allow {
    }
  }
  visibility_config {
    cloudwatch_metrics_enabled = true
    metric_name                = "${var.name_prefix}-edge"
    sampled_requests_enabled   = true
  }
  rule {
    name     = "ip-rate"
    priority = 1
    action {
      block {
      }
    }
    statement {
      rate_based_statement {
        limit              = 2000
        aggregate_key_type = "IP"
      }
    }
    visibility_config {
      cloudwatch_metrics_enabled = true
      metric_name                = "${var.name_prefix}-ip-rate"
      sampled_requests_enabled   = true
    }
  }
  tags = var.tags
}

resource "aws_cloudwatch_log_group" "waf" {
  count             = var.enable_waf ? 1 : 0
  name              = "aws-waf-logs-${var.name_prefix}"
  retention_in_days = var.log_retention_days
  kms_key_id        = var.kms_key_arn
  tags              = var.tags
}

resource "aws_wafv2_web_acl_logging_configuration" "edge" {
  count                   = var.enable_waf ? 1 : 0
  log_destination_configs = [aws_cloudwatch_log_group.waf[0].arn]
  resource_arn            = aws_wafv2_web_acl.edge[0].arn
}

resource "aws_cloudwatch_metric_alarm" "waf_blocks" {
  count               = var.enable_waf ? 1 : 0
  alarm_name          = "${var.name_prefix}-waf-blocks"
  alarm_description   = "Owner: security. Action: review blocked IPs and false positives."
  comparison_operator = "GreaterThanThreshold"
  evaluation_periods  = 1
  metric_name         = "BlockedRequests"
  namespace           = "AWS/WAFV2"
  period              = 300
  statistic           = "Sum"
  threshold           = 10000
  alarm_actions       = var.alarm_actions
  dimensions = {
    WebACL = aws_wafv2_web_acl.edge[0].name
    Region = "CloudFront"
    Rule   = "ALL"
  }
  tags = var.tags
}
