locals {
  tables = {
    links = {
      hash_key = "short_code"
      ttl      = "purge_at"
    }
    stats = {
      hash_key = "short_code"
      ttl      = null
    }
    counters = {
      hash_key = "region"
      ttl      = null
    }
    idempotency = {
      hash_key = "owner_key"
      ttl      = "ttl"
    }
    credentials = {
      hash_key = "token_hash"
      ttl      = null
    }
  }
}

resource "aws_dynamodb_table" "this" {
  for_each = local.tables

  name         = "${var.name_prefix}-${each.key}"
  billing_mode = "PAY_PER_REQUEST"
  hash_key     = each.value.hash_key

  attribute {
    name = each.value.hash_key
    type = "S"
  }

  dynamic "ttl" {
    for_each = each.value.ttl == null ? [] : [each.value.ttl]
    content {
      attribute_name = ttl.value
      enabled        = true
    }
  }

  server_side_encryption {
    enabled     = true
    kms_key_arn = var.kms_key_arn
  }

  point_in_time_recovery {
    enabled = var.enable_pitr
  }

  tags = merge(var.tags, { Name = "${var.name_prefix}-${each.key}" })
}

resource "aws_cloudwatch_log_group" "alarms" {
  name              = "/molla/${var.name_prefix}/data"
  retention_in_days = var.log_retention_days
  kms_key_id        = var.kms_key_arn
  tags              = var.tags
}

resource "aws_cloudwatch_metric_alarm" "throttles" {
  for_each = aws_dynamodb_table.this

  alarm_name          = "${each.value.name}-throttles"
  alarm_description   = "Owner: platform. Action: page on-call and raise on-demand capacity investigation."
  comparison_operator = "GreaterThanThreshold"
  evaluation_periods  = 1
  metric_name         = "ThrottledRequests"
  namespace           = "AWS/DynamoDB"
  period              = 60
  statistic           = "Sum"
  threshold           = 0
  treat_missing_data  = "notBreaching"
  alarm_actions       = var.alarm_actions
  ok_actions          = var.alarm_actions

  dimensions = {
    TableName = each.value.name
  }

  tags = var.tags
}
