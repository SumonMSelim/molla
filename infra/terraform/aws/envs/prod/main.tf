locals {
  name_prefix = "molla-prod"
  tags = {
    Workload       = "molla"
    Environment    = "prod"
    Owner          = "platform"
    CostCenter     = "molla"
    CostAllocation = "prod"
  }
  # Quota headroom ≥20% above DESIGN.md tested peak (15,432 redirect QPS, 154 write QPS, 160 redirect concurrency).
  quota_headroom = {
    lambda_redirect_provisioned = 192
    kinesis_shards              = 20
    api_gateway_stage_rate      = 12
    dynamodb_on_demand          = "adaptive"
    vpc_interface_endpoints     = 2
    waf_rate_limit              = 2000
  }
  account_controls = {
    cloudtrail      = var.central_cloudtrail_arn
    config          = var.central_config_recorder_arn
    guardduty       = var.central_guardduty_detector_id
    security_hub    = var.central_security_hub_arn
    access_analyzer = var.central_access_analyzer_arn
  }
}

resource "aws_kms_key" "this" {
  description             = "molla prod"
  deletion_window_in_days = 30
  enable_key_rotation     = true
  tags                    = local.tags
}

resource "aws_kms_alias" "this" {
  name          = "alias/${local.name_prefix}"
  target_key_id = aws_kms_key.this.id
}

resource "aws_sns_topic" "alarms" {
  name              = "${local.name_prefix}-alarms"
  kms_master_key_id = aws_kms_key.this.id
  tags              = local.tags
}

resource "aws_sns_topic_subscription" "email" {
  topic_arn = aws_sns_topic.alarms.arn
  protocol  = "email"
  endpoint  = var.alarm_email
}

module "data" {
  source        = "../../modules/data"
  name_prefix   = local.name_prefix
  enable_pitr   = true
  kms_key_arn   = aws_kms_key.this.arn
  alarm_actions = [aws_sns_topic.alarms.arn]
  tags          = local.tags
}

module "analytics" {
  source           = "../../modules/analytics"
  name_prefix      = local.name_prefix
  kms_key_arn      = aws_kms_key.this.arn
  artifact_dir     = var.artifact_dir
  stats_table_arn  = module.data.table_arns["stats"]
  stats_table_name = module.data.stats_table_name
  shard_count      = local.quota_headroom.kinesis_shards
  alarm_actions    = [aws_sns_topic.alarms.arn]
  tags             = local.tags
}

module "api" {
  source                  = "../../modules/api"
  name_prefix             = local.name_prefix
  kms_key_arn             = aws_kms_key.this.arn
  artifact_dir            = var.artifact_dir
  table_arns              = module.data.table_arns
  table_names             = module.data.table_names
  stream_arn              = module.analytics.stream_arn
  stream_name             = module.analytics.stream_name
  redis_node_type         = "cache.r7g.large"
  redis_auth_token        = var.redis_auth_token
  permutation_key         = var.permutation_key
  privacy_key             = var.privacy_key
  provisioned_concurrency = local.quota_headroom.lambda_redirect_provisioned
  admin_principal_arns    = var.admin_principal_arns
  alarm_actions           = [aws_sns_topic.alarms.arn]
  tags                    = local.tags
}

module "edge" {
  source                = "../../modules/edge"
  name_prefix           = local.name_prefix
  kms_key_arn           = aws_kms_key.this.arn
  api_gateway_id        = module.api.rest_api_id
  redirect_function_url = module.api.redirect_function_url
  redirect_function_arn = module.api.redirect_function_arn
  enable_waf            = true
  alarm_actions         = [aws_sns_topic.alarms.arn]
  tags                  = local.tags
}

resource "aws_cloudwatch_dashboard" "workload" {
  dashboard_name = local.name_prefix
  dashboard_body = jsonencode({
    widgets = [
      {
        type   = "metric"
        x      = 0
        y      = 0
        width  = 12
        height = 6
        properties = {
          title  = "Redirect availability and origin latency"
          region = var.region
          metrics = [
            ["AWS/CloudFront", "5xxErrorRate", "DistributionId", module.edge.distribution_id],
            [".", "OriginLatency", ".", "."],
          ]
        }
      },
      {
        type   = "text"
        x      = 0
        y      = 6
        width  = 24
        height = 3
        properties = {
          markdown = join("\n", [
            "# Account controls (centrally managed)",
            "- CloudTrail: ${local.account_controls.cloudtrail}",
            "- Config: ${local.account_controls.config}",
            "- GuardDuty: ${local.account_controls.guardduty}",
            "- Security Hub: ${local.account_controls.security_hub}",
            "- Access Analyzer: ${local.account_controls.access_analyzer}",
            "- Quota headroom: provisioned concurrency ${local.quota_headroom.lambda_redirect_provisioned}, Kinesis shards ${local.quota_headroom.kinesis_shards} (≥20% above 15,432 QPS / 160 concurrent).",
          ])
        }
      }
    ]
  })
}

resource "aws_budgets_budget" "monthly" {
  name         = local.name_prefix
  budget_type  = "COST"
  limit_amount = var.budget_limit
  limit_unit   = "USD"
  time_unit    = "MONTHLY"
  cost_filter {
    name   = "TagKeyValue"
    values = ["user:Workload$molla"]
  }
  notification {
    comparison_operator        = "GREATER_THAN"
    threshold                  = 80
    threshold_type             = "PERCENTAGE"
    notification_type          = "ACTUAL"
    subscriber_email_addresses = [var.alarm_email]
  }
}

resource "aws_ce_anomaly_monitor" "workload" {
  name              = local.name_prefix
  monitor_type      = "DIMENSIONAL"
  monitor_dimension = "SERVICE"
}

resource "aws_ce_anomaly_subscription" "workload" {
  name             = local.name_prefix
  frequency        = "DAILY"
  monitor_arn_list = [aws_ce_anomaly_monitor.workload.arn]
  subscriber {
    type    = "SNS"
    address = aws_sns_topic.alarms.arn
  }
  threshold_expression {
    dimension {
      key           = "ANOMALY_TOTAL_IMPACT_ABSOLUTE"
      match_options = ["GREATER_THAN_OR_EQUAL"]
      values        = ["50"]
    }
  }
}

resource "aws_cloudwatch_metric_alarm" "budget_anomaly_proxy" {
  alarm_name          = "${local.name_prefix}-cost-anomaly"
  alarm_description   = "Owner: finance+platform. Action: review Cost Anomaly Detection and AWS Budgets."
  comparison_operator = "GreaterThanThreshold"
  evaluation_periods  = 1
  metric_name         = "NumberOfMessagesPublished"
  namespace           = "AWS/SNS"
  period              = 86400
  statistic           = "Sum"
  threshold           = 0
  treat_missing_data  = "notBreaching"
  alarm_actions       = [aws_sns_topic.alarms.arn]
  dimensions          = { TopicName = aws_sns_topic.alarms.name }
  tags                = local.tags
}
