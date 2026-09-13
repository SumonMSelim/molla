resource "aws_cloudwatch_log_group" "api" {
  name              = "/aws/lambda/${var.name_prefix}-api"
  retention_in_days = var.log_retention_days
  kms_key_id        = var.kms_key_arn
  tags              = var.tags
}

resource "aws_cloudwatch_log_group" "redirect" {
  name              = "/aws/lambda/${var.name_prefix}-redirect"
  retention_in_days = var.log_retention_days
  kms_key_id        = var.kms_key_arn
  tags              = var.tags
}

resource "aws_cloudwatch_log_group" "invalidate" {
  name              = "/aws/lambda/${var.name_prefix}-invalidate"
  retention_in_days = var.log_retention_days
  kms_key_id        = var.kms_key_arn
  tags              = var.tags
}

resource "aws_lambda_function" "api" {
  function_name    = "${var.name_prefix}-api"
  role             = aws_iam_role.api.arn
  filename         = "${var.artifact_dir}/api.zip"
  source_code_hash = filebase64sha256("${var.artifact_dir}/api.zip")
  handler          = "bootstrap"
  runtime          = "provided.al2023"
  architectures    = ["arm64"]
  memory_size      = 128
  timeout          = 10
  kms_key_arn      = var.kms_key_arn
  tracing_config { mode = "Active" }
  environment {
    variables = {
      MOLLA_PERMUTATION_KEY   = var.permutation_key
      MOLLA_PUBLIC_BASE       = var.public_base
      MOLLA_LINKS_TABLE       = var.table_names["links"]
      MOLLA_STATS_TABLE       = var.table_names["stats"]
      MOLLA_COUNTERS_TABLE    = var.table_names["counters"]
      MOLLA_IDEMPOTENCY_TABLE = var.table_names["idempotency"]
    }
  }
  depends_on = [aws_cloudwatch_log_group.api]
  tags       = var.tags
}

resource "aws_lambda_function" "redirect" {
  function_name    = "${var.name_prefix}-redirect"
  role             = aws_iam_role.redirect.arn
  filename         = "${var.artifact_dir}/redirect.zip"
  source_code_hash = filebase64sha256("${var.artifact_dir}/redirect.zip")
  handler          = "bootstrap"
  runtime          = "provided.al2023"
  architectures    = ["arm64"]
  memory_size      = 128
  timeout          = 3
  publish          = true
  kms_key_arn      = var.kms_key_arn
  tracing_config { mode = "Active" }
  vpc_config {
    subnet_ids         = aws_subnet.private[*].id
    security_group_ids = [aws_security_group.lambda.id]
  }
  environment {
    variables = {
      MOLLA_PRIVACY_KEY  = var.privacy_key
      MOLLA_REDIS_ADDR   = "${aws_elasticache_replication_group.redis.primary_endpoint_address}:6379"
      MOLLA_REDIS_AUTH   = var.redis_auth_token
      MOLLA_REDIS_TLS    = "1"
      MOLLA_CLICK_STREAM = var.stream_name
      MOLLA_LINKS_TABLE  = var.table_names["links"]
    }
  }
  depends_on = [aws_cloudwatch_log_group.redirect]
  tags       = var.tags
}

resource "aws_lambda_alias" "redirect_live" {
  name             = "live"
  function_name    = aws_lambda_function.redirect.function_name
  function_version = aws_lambda_function.redirect.version
}

resource "aws_lambda_provisioned_concurrency_config" "redirect" {
  count                             = var.provisioned_concurrency > 0 ? 1 : 0
  function_name                     = aws_lambda_alias.redirect_live.function_name
  qualifier                         = aws_lambda_alias.redirect_live.name
  provisioned_concurrent_executions = var.provisioned_concurrency
}

resource "aws_lambda_function" "invalidate" {
  function_name    = "${var.name_prefix}-invalidate"
  role             = aws_iam_role.invalidate.arn
  filename         = "${var.artifact_dir}/invalidate.zip"
  source_code_hash = filebase64sha256("${var.artifact_dir}/invalidate.zip")
  handler          = "bootstrap"
  runtime          = "provided.al2023"
  architectures    = ["arm64"]
  memory_size      = 128
  timeout          = 10
  kms_key_arn      = var.kms_key_arn
  tracing_config { mode = "Active" }
  vpc_config {
    subnet_ids         = aws_subnet.private[*].id
    security_group_ids = [aws_security_group.lambda.id]
  }
  environment {
    variables = {
      MOLLA_REDIS_ADDR = "${aws_elasticache_replication_group.redis.primary_endpoint_address}:6379"
      MOLLA_REDIS_AUTH = var.redis_auth_token
      MOLLA_REDIS_TLS  = "1"
    }
  }
  depends_on = [aws_cloudwatch_log_group.invalidate]
  tags       = var.tags
}

resource "aws_lambda_function_url" "redirect" {
  function_name      = aws_lambda_alias.redirect_live.function_name
  qualifier          = aws_lambda_alias.redirect_live.name
  authorization_type = "AWS_IAM"
}

resource "aws_lambda_permission" "api_gateway" {
  statement_id  = "AllowAPIGateway"
  action        = "lambda:InvokeFunction"
  function_name = aws_lambda_function.api.function_name
  principal     = "apigateway.amazonaws.com"
  source_arn    = "${aws_api_gateway_rest_api.this.execution_arn}/*/*"
}
