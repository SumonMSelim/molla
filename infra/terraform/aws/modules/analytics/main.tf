data "aws_caller_identity" "current" {}

# Click analytics: the Clicks table's DynamoDB Stream (owned by modules/data)
# feeds the aggregate Lambda, which folds a batch into one UpdateItem per code
# on LinkStats. Raw events live in the Clicks table itself under a 90-day TTL,
# so there is no Kinesis stream, no Firehose, and no archive bucket. Lambda
# reads from DynamoDB Streams are free, and the redirect Lambda writes clicks
# through the free gateway endpoint, so this path has no fixed monthly cost.

resource "aws_sqs_queue" "aggregate_dlq" {
  name                      = "${var.name_prefix}-aggregate-dlq"
  kms_master_key_id         = var.kms_key_arn
  message_retention_seconds = 1209600
  tags                      = var.tags
}

resource "aws_cloudwatch_log_group" "aggregate" {
  name              = "/aws/lambda/${var.name_prefix}-aggregate"
  retention_in_days = var.log_retention_days
  kms_key_id        = var.kms_key_arn
  tags              = var.tags
}

resource "aws_iam_role" "aggregate" {
  name = "${var.name_prefix}-aggregate"
  tags = var.tags
  assume_role_policy = jsonencode({
    Version = "2012-10-17"
    Statement = [{
      Action    = "sts:AssumeRole"
      Effect    = "Allow"
      Principal = { Service = "lambda.amazonaws.com" }
    }]
  })
}

resource "aws_iam_role_policy" "aggregate" {
  name = "aggregate"
  role = aws_iam_role.aggregate.id
  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Effect   = "Allow"
        Action   = ["logs:CreateLogStream", "logs:PutLogEvents"]
        Resource = "${aws_cloudwatch_log_group.aggregate.arn}:*"
      },
      {
        Sid    = "ClicksStream"
        Effect = "Allow"
        Action = [
          "dynamodb:DescribeStream",
          "dynamodb:GetRecords",
          "dynamodb:GetShardIterator",
          "dynamodb:ListStreams",
        ]
        Resource = var.clicks_stream_arn
      },
      {
        Sid      = "StatsTable"
        Effect   = "Allow"
        Action   = ["dynamodb:UpdateItem", "dynamodb:GetItem"]
        Resource = var.stats_table_arn
      },
      {
        Effect   = "Allow"
        Action   = ["kms:Decrypt", "kms:GenerateDataKey"]
        Resource = var.kms_key_arn
      },
      {
        Effect   = "Allow"
        Action   = ["sqs:SendMessage"]
        Resource = aws_sqs_queue.aggregate_dlq.arn
      }
    ]
  })
}

resource "aws_lambda_function" "aggregate" {
  function_name                  = "${var.name_prefix}-aggregate"
  role                           = aws_iam_role.aggregate.arn
  filename                       = "${var.artifact_dir}/aggregate.zip"
  source_code_hash               = filebase64sha256("${var.artifact_dir}/aggregate.zip")
  handler                        = "bootstrap"
  runtime                        = "provided.al2023"
  architectures                  = ["arm64"]
  memory_size                    = 128
  timeout                        = 60
  reserved_concurrent_executions = var.aggregate_reserved_concurrency > 0 ? var.aggregate_reserved_concurrency : null
  kms_key_arn                    = var.kms_key_arn
  tracing_config {
    mode = "Active"
  }
  environment {
    variables = {
      MOLLA_STATS_TABLE = var.stats_table_name
    }
  }
  depends_on = [aws_cloudwatch_log_group.aggregate]
  tags       = var.tags
}

resource "aws_lambda_event_source_mapping" "clicks" {
  event_source_arn                   = var.clicks_stream_arn
  function_name                      = aws_lambda_function.aggregate.arn
  starting_position                  = "LATEST"
  batch_size                         = 100
  maximum_retry_attempts             = 2
  bisect_batch_on_function_error     = true
  function_response_types            = ["ReportBatchItemFailures"]
  maximum_batching_window_in_seconds = 5
  destination_config {
    on_failure {
      destination_arn = aws_sqs_queue.aggregate_dlq.arn
    }
  }
  # Only INSERTs carry a click; TTL sweeps arrive as REMOVE and would be
  # billed invocations that count nothing. Filter them out at the source.
  filter_criteria {
    filter {
      pattern = jsonencode({ eventName = ["INSERT"] })
    }
  }
}

resource "aws_cloudwatch_metric_alarm" "iterator_age" {
  alarm_name          = "${var.name_prefix}-clicks-iterator-age"
  alarm_description   = "Owner: platform. Action: inspect aggregator errors and throttles; stream records are falling behind."
  comparison_operator = "GreaterThanThreshold"
  evaluation_periods  = 3
  metric_name         = "IteratorAge"
  namespace           = "AWS/Lambda"
  period              = 60
  statistic           = "Maximum"
  threshold           = 60000
  alarm_actions       = var.alarm_actions
  dimensions          = { FunctionName = aws_lambda_function.aggregate.function_name }
  tags                = var.tags
}

resource "aws_cloudwatch_metric_alarm" "dlq_visible" {
  alarm_name          = "${var.name_prefix}-aggregate-dlq-visible"
  alarm_description   = "Owner: platform. Action: inspect failed click batches in the encrypted SQS DLQ."
  comparison_operator = "GreaterThanThreshold"
  evaluation_periods  = 1
  metric_name         = "ApproximateNumberOfVisibleMessages"
  namespace           = "AWS/SQS"
  period              = 60
  statistic           = "Maximum"
  threshold           = 0
  treat_missing_data  = "notBreaching"
  alarm_actions       = var.alarm_actions
  dimensions          = { QueueName = aws_sqs_queue.aggregate_dlq.name }
  tags                = var.tags
}

resource "aws_cloudwatch_metric_alarm" "aggregate_errors" {
  alarm_name          = "${var.name_prefix}-aggregate-errors"
  alarm_description   = "Owner: platform. Action: inspect aggregator logs."
  comparison_operator = "GreaterThanThreshold"
  evaluation_periods  = 1
  metric_name         = "Errors"
  namespace           = "AWS/Lambda"
  period              = 60
  statistic           = "Sum"
  threshold           = 0
  treat_missing_data  = "notBreaching"
  alarm_actions       = var.alarm_actions
  dimensions          = { FunctionName = aws_lambda_function.aggregate.function_name }
  tags                = var.tags
}
