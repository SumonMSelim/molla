resource "random_password" "permutation_key" {
  count   = var.permutation_key == "" ? 1 : 0
  length  = 32
  special = false
}

resource "random_password" "privacy_key" {
  count   = var.privacy_key == "" ? 1 : 0
  length  = 32
  special = false
}

resource "random_password" "redis_auth_token" {
  count   = var.redis_auth_token == "" ? 1 : 0
  length  = 32
  special = false
}

resource "random_password" "origin_verify_secret" {
  count   = var.domain_name != "" && var.origin_verify_secret == "" ? 1 : 0
  length  = 32
  special = false
}

locals {
  # Generated on first apply and then stable: an unset input keeps resolving
  # to the same random_password resource rather than a new value each plan.
  permutation_key      = var.permutation_key != "" ? var.permutation_key : one(random_password.permutation_key[*].result)
  privacy_key          = var.privacy_key != "" ? var.privacy_key : one(random_password.privacy_key[*].result)
  redis_auth_token     = var.redis_auth_token != "" ? var.redis_auth_token : one(random_password.redis_auth_token[*].result)
  origin_verify_secret = var.origin_verify_secret != "" ? var.origin_verify_secret : try(coalesce(one(random_password.origin_verify_secret[*].result), ""), "")
  name_prefix          = "molla-prod"
  tags = {
    Workload       = "molla"
    Environment    = "prod"
    Owner          = "platform"
    CostCenter     = "molla"
    CostAllocation = "prod"
  }
  # This deploy runs at hobby/learning scale, not DESIGN.md's tested load
  # targets (15,432 redirect QPS, 154 write QPS). Every module default is
  # already sized for near-zero cost (on-demand billing, smallest instance
  # sizes, no provisioned concurrency); nothing here overrides them upward.
  # Revisit quota_headroom in DESIGN.md if real traffic ever approaches
  # those numbers.
  account_controls = {
    cloudtrail      = var.central_cloudtrail_arn
    config          = var.central_config_recorder_arn
    guardduty       = var.central_guardduty_detector_id
    security_hub    = var.central_security_hub_arn
    access_analyzer = var.central_access_analyzer_arn
  }
}

data "aws_caller_identity" "current" {}
data "aws_region" "current" {}

resource "aws_kms_key" "this" {
  description             = "molla prod"
  deletion_window_in_days = 30
  enable_key_rotation     = true
  tags                    = local.tags
  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Sid       = "EnableRootPermissions"
        Effect    = "Allow"
        Principal = { AWS = "arn:aws:iam::${data.aws_caller_identity.current.account_id}:root" }
        Action    = "kms:*"
        Resource  = "*"
      },
      {
        Sid       = "AllowCloudWatchLogs"
        Effect    = "Allow"
        Principal = { Service = "logs.${data.aws_region.current.region}.amazonaws.com" }
        Action = [
          "kms:Encrypt",
          "kms:Decrypt",
          "kms:ReEncrypt*",
          "kms:GenerateDataKey*",
          "kms:DescribeKey",
        ]
        Resource = "*"
        Condition = {
          ArnLike = {
            "kms:EncryptionContext:aws:logs:arn" = "arn:aws:logs:${data.aws_region.current.region}:${data.aws_caller_identity.current.account_id}:log-group:*"
          }
        }
      }
    ]
  })
}

resource "aws_kms_alias" "this" {
  name          = "alias/${local.name_prefix}"
  target_key_id = aws_kms_key.this.id
}

# Account-wide, not per-workload: API Gateway needs this role set once per
# account before any stage can enable CloudWatch access logging.
resource "aws_iam_role" "apigateway_cloudwatch" {
  name = "${local.name_prefix}-apigateway-cloudwatch"
  tags = local.tags
  assume_role_policy = jsonencode({
    Version = "2012-10-17"
    Statement = [{
      Action    = "sts:AssumeRole"
      Effect    = "Allow"
      Principal = { Service = "apigateway.amazonaws.com" }
    }]
  })
}

resource "aws_iam_role_policy_attachment" "apigateway_cloudwatch" {
  role       = aws_iam_role.apigateway_cloudwatch.name
  policy_arn = "arn:aws:iam::aws:policy/service-role/AmazonAPIGatewayPushToCloudWatchLogs"
}

resource "aws_api_gateway_account" "this" {
  cloudwatch_role_arn = aws_iam_role.apigateway_cloudwatch.arn
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
  enable_pitr   = false # hobby scale: skip continuous backup cost, not worth it for this data
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
  alarm_actions    = [aws_sns_topic.alarms.arn]
  tags             = local.tags
}

module "api" {
  source               = "../../modules/api"
  name_prefix          = local.name_prefix
  kms_key_arn          = aws_kms_key.this.arn
  artifact_dir         = var.artifact_dir
  table_arns           = module.data.table_arns
  table_names          = module.data.table_names
  stream_arn           = module.analytics.stream_arn
  stream_name          = module.analytics.stream_name
  redis_auth_token     = local.redis_auth_token
  permutation_key      = local.permutation_key
  privacy_key          = local.privacy_key
  admin_principal_arns = var.admin_principal_arns
  alarm_actions        = [aws_sns_topic.alarms.arn]
  tags                 = local.tags
}

module "ci" {
  source            = "../../modules/ci"
  name_prefix       = local.name_prefix
  github_repository = var.github_repository
  tags              = local.tags
}

module "edge" {
  source                = "../../modules/edge"
  name_prefix           = local.name_prefix
  kms_key_arn           = aws_kms_key.this.arn
  api_gateway_id        = module.api.rest_api_id
  redirect_function_url = module.api.redirect_function_url
  redirect_function_arn = module.api.redirect_function_arn
  domain_name           = var.domain_name
  cloudflare_zone_id    = var.cloudflare_zone_id
  origin_verify_secret  = local.origin_verify_secret
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
            "- Sizing: hobby/learning scale, not DESIGN.md's tested load targets. No provisioned Lambda concurrency, on-demand Kinesis, single-node Redis, PITR off.",
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

# No aws_ce_anomaly_monitor here: AWS allows only one DIMENSIONAL/SERVICE cost
# anomaly monitor per account, and this account already has one
# ("AWS Service Monitor", auto-created outside Terraform). Subscribe that
# existing monitor to alarm_email by hand in the Cost Anomaly Detection
# console if per-service anomaly alerts are wanted; this budget alarm covers
# the "spending is above budget" signal instead.

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
