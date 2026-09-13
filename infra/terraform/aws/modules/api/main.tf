resource "aws_api_gateway_rest_api" "this" {
  name = "${var.name_prefix}-api"
  endpoint_configuration { types = ["REGIONAL"] }
  tags = var.tags
}

resource "aws_api_gateway_resource" "proxy" {
  rest_api_id = aws_api_gateway_rest_api.this.id
  parent_id   = aws_api_gateway_rest_api.this.root_resource_id
  path_part   = "{proxy+}"
}

resource "aws_api_gateway_method" "proxy" {
  rest_api_id   = aws_api_gateway_rest_api.this.id
  resource_id   = aws_api_gateway_resource.proxy.id
  http_method   = "ANY"
  authorization = "NONE"
}

resource "aws_api_gateway_integration" "proxy" {
  rest_api_id             = aws_api_gateway_rest_api.this.id
  resource_id             = aws_api_gateway_resource.proxy.id
  http_method             = aws_api_gateway_method.proxy.http_method
  integration_http_method = "POST"
  type                    = "AWS_PROXY"
  uri                     = aws_lambda_function.api.invoke_arn
}

resource "aws_api_gateway_deployment" "this" {
  rest_api_id = aws_api_gateway_rest_api.this.id
  triggers = {
    redeploy = sha1(jsonencode([
      aws_api_gateway_integration.proxy.id,
      aws_api_gateway_method.proxy.id,
    ]))
  }
  lifecycle { create_before_destroy = true }
  depends_on = [aws_api_gateway_integration.proxy]
}

resource "aws_api_gateway_stage" "live" {
  rest_api_id          = aws_api_gateway_rest_api.this.id
  deployment_id        = aws_api_gateway_deployment.this.id
  stage_name           = "live"
  xray_tracing_enabled = true
  access_log_settings {
    destination_arn = aws_cloudwatch_log_group.apigw.arn
    format = jsonencode({
      requestId = "$context.requestId"
      status    = "$context.status"
    })
  }
  tags = var.tags
}

resource "aws_cloudwatch_log_group" "apigw" {
  name              = "/aws/apigateway/${var.name_prefix}"
  retention_in_days = var.log_retention_days
  kms_key_id        = var.kms_key_arn
  tags              = var.tags
}

resource "aws_api_gateway_method_settings" "all" {
  rest_api_id = aws_api_gateway_rest_api.this.id
  stage_name  = aws_api_gateway_stage.live.stage_name
  method_path = "*/*"
  settings {
    metrics_enabled        = true
    logging_level          = "ERROR"
    throttling_rate_limit  = var.throttle_rate_limit
    throttling_burst_limit = var.throttle_burst_limit
  }
}

resource "aws_api_gateway_gateway_response" "unauthorized" {
  rest_api_id   = aws_api_gateway_rest_api.this.id
  response_type = "UNAUTHORIZED"
  status_code   = "401"
  response_templates = {
    "application/json" = "{\"error\":\"UNAUTHORIZED\"}"
  }
}

resource "aws_ssm_parameter" "permutation_key" {
  name   = "/molla/${var.name_prefix}/permutation_key"
  type   = "SecureString"
  key_id = var.kms_key_arn
  value  = var.permutation_key
  tags   = var.tags
}

resource "aws_ssm_parameter" "privacy_key" {
  name   = "/molla/${var.name_prefix}/privacy_key"
  type   = "SecureString"
  key_id = var.kms_key_arn
  value  = var.privacy_key
  tags   = var.tags
}

resource "aws_ssm_parameter" "redis_auth_token" {
  name   = "/molla/${var.name_prefix}/redis_auth_token"
  type   = "SecureString"
  key_id = var.kms_key_arn
  value  = var.redis_auth_token
  tags   = var.tags
}

resource "aws_cloudwatch_metric_alarm" "api_5xx" {
  alarm_name          = "${var.name_prefix}-api-5xx"
  alarm_description   = "Owner: platform. Action: inspect API Lambda and DynamoDB."
  comparison_operator = "GreaterThanThreshold"
  evaluation_periods  = 1
  metric_name         = "5XXError"
  namespace           = "AWS/ApiGateway"
  period              = 60
  statistic           = "Sum"
  threshold           = 0
  treat_missing_data  = "notBreaching"
  alarm_actions       = var.alarm_actions
  dimensions          = { ApiName = aws_api_gateway_rest_api.this.name, Stage = aws_api_gateway_stage.live.stage_name }
  tags                = var.tags
}

resource "aws_cloudwatch_metric_alarm" "api_latency" {
  alarm_name          = "${var.name_prefix}-api-latency"
  alarm_description   = "Owner: platform. Action: check Lambda duration and DynamoDB throttles."
  comparison_operator = "GreaterThanThreshold"
  evaluation_periods  = 5
  metric_name         = "Latency"
  namespace           = "AWS/ApiGateway"
  period              = 60
  extended_statistic  = "p99"
  threshold           = 1000
  alarm_actions       = var.alarm_actions
  dimensions          = { ApiName = aws_api_gateway_rest_api.this.name, Stage = aws_api_gateway_stage.live.stage_name }
  tags                = var.tags
}

resource "aws_cloudwatch_metric_alarm" "lambda_errors" {
  for_each = {
    api        = aws_lambda_function.api.function_name
    redirect   = aws_lambda_function.redirect.function_name
    invalidate = aws_lambda_function.invalidate.function_name
  }
  alarm_name          = "${each.value}-errors"
  alarm_description   = "Owner: platform. Action: inspect function logs and throttles."
  comparison_operator = "GreaterThanThreshold"
  evaluation_periods  = 1
  metric_name         = "Errors"
  namespace           = "AWS/Lambda"
  period              = 60
  statistic           = "Sum"
  threshold           = 0
  treat_missing_data  = "notBreaching"
  alarm_actions       = var.alarm_actions
  dimensions          = { FunctionName = each.value }
  tags                = var.tags
}

resource "aws_cloudwatch_metric_alarm" "lambda_throttles" {
  for_each = {
    api        = aws_lambda_function.api.function_name
    redirect   = aws_lambda_function.redirect.function_name
    invalidate = aws_lambda_function.invalidate.function_name
  }
  alarm_name          = "${each.value}-throttles"
  alarm_description   = "Owner: platform. Action: raise reserved concurrency (20% headroom above peak)."
  comparison_operator = "GreaterThanThreshold"
  evaluation_periods  = 1
  metric_name         = "Throttles"
  namespace           = "AWS/Lambda"
  period              = 60
  statistic           = "Sum"
  threshold           = 0
  treat_missing_data  = "notBreaching"
  alarm_actions       = var.alarm_actions
  dimensions          = { FunctionName = each.value }
  tags                = var.tags
}

resource "aws_cloudwatch_metric_alarm" "redis_cpu" {
  alarm_name          = "${var.name_prefix}-redis-cpu"
  alarm_description   = "Owner: platform. Action: fail over / scale cache.r7g nodes."
  comparison_operator = "GreaterThanThreshold"
  evaluation_periods  = 3
  metric_name         = "EngineCPUUtilization"
  namespace           = "AWS/ElastiCache"
  period              = 60
  statistic           = "Average"
  threshold           = 80
  alarm_actions       = var.alarm_actions
  dimensions          = { ReplicationGroupId = aws_elasticache_replication_group.redis.id }
  tags                = var.tags
}

resource "aws_cloudwatch_metric_alarm" "redis_evictions" {
  alarm_name          = "${var.name_prefix}-redis-evictions"
  alarm_description   = "Owner: platform. Action: review memory and eviction policy."
  comparison_operator = "GreaterThanThreshold"
  evaluation_periods  = 1
  metric_name         = "Evictions"
  namespace           = "AWS/ElastiCache"
  period              = 60
  statistic           = "Sum"
  threshold           = 100
  alarm_actions       = var.alarm_actions
  dimensions          = { ReplicationGroupId = aws_elasticache_replication_group.redis.id }
  tags                = var.tags
}

resource "aws_cloudwatch_metric_alarm" "invalidate_errors" {
  alarm_name          = "${var.name_prefix}-invalidation-failures"
  alarm_description   = "Owner: security. Action: retry takedown; tombstone may be missing."
  comparison_operator = "GreaterThanThreshold"
  evaluation_periods  = 1
  metric_name         = "Errors"
  namespace           = "AWS/Lambda"
  period              = 60
  statistic           = "Sum"
  threshold           = 0
  treat_missing_data  = "notBreaching"
  alarm_actions       = var.alarm_actions
  dimensions          = { FunctionName = aws_lambda_function.invalidate.function_name }
  tags                = var.tags
}
