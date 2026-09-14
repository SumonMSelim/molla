data "aws_caller_identity" "current" {}

resource "aws_s3_bucket" "archive" {
  bucket = "${var.name_prefix}-click-archive-${data.aws_caller_identity.current.account_id}"
  tags   = var.tags
}

resource "aws_s3_bucket_public_access_block" "archive" {
  bucket                  = aws_s3_bucket.archive.id
  block_public_acls       = true
  block_public_policy     = true
  ignore_public_acls      = true
  restrict_public_buckets = true
}

resource "aws_s3_bucket_server_side_encryption_configuration" "archive" {
  bucket = aws_s3_bucket.archive.id
  rule {
    apply_server_side_encryption_by_default {
      sse_algorithm     = "aws:kms"
      kms_master_key_id = var.kms_key_arn
    }
    bucket_key_enabled = true
  }
}

resource "aws_s3_bucket_lifecycle_configuration" "archive" {
  bucket = aws_s3_bucket.archive.id
  rule {
    id     = "expire-90d"
    status = "Enabled"
    filter {
    }
    expiration {
      days = 90
    }
  }
}

resource "aws_s3_bucket_policy" "archive" {
  bucket = aws_s3_bucket.archive.id
  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Sid       = "DenyInsecureTransport"
        Effect    = "Deny"
        Principal = "*"
        Action    = "s3:*"
        Resource = [
          aws_s3_bucket.archive.arn,
          "${aws_s3_bucket.archive.arn}/*",
        ]
        Condition = {
          Bool = { "aws:SecureTransport" = "false" }
        }
      }
    ]
  })
}

resource "aws_kinesis_stream" "clicks" {
  name             = "${var.name_prefix}-clicks"
  shard_count      = var.stream_mode == "PROVISIONED" ? var.shard_count : null
  encryption_type  = "KMS"
  kms_key_id       = var.kms_key_arn
  retention_period = 24
  stream_mode_details {
    stream_mode = var.stream_mode
  }
  tags = var.tags
}

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
        Effect = "Allow"
        Action = [
          "kinesis:DescribeStream",
          "kinesis:DescribeStreamSummary",
          "kinesis:GetRecords",
          "kinesis:GetShardIterator",
          "kinesis:ListShards",
          "kinesis:ListStreams",
          "kinesis:SubscribeToShard",
        ]
        Resource = aws_kinesis_stream.clicks.arn
      },
      {
        Effect   = "Allow"
        Action   = ["kinesis:ListStreams"]
        Resource = "*"
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
  reserved_concurrent_executions = var.aggregate_reserved_concurrency
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
  event_source_arn                   = aws_kinesis_stream.clicks.arn
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
}

resource "aws_iam_role" "firehose" {
  name = "${var.name_prefix}-firehose"
  tags = var.tags
  assume_role_policy = jsonencode({
    Version = "2012-10-17"
    Statement = [{
      Action    = "sts:AssumeRole"
      Effect    = "Allow"
      Principal = { Service = "firehose.amazonaws.com" }
    }]
  })
}

resource "aws_iam_role_policy" "firehose" {
  name = "firehose"
  role = aws_iam_role.firehose.id
  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Effect = "Allow"
        Action = [
          "s3:AbortMultipartUpload",
          "s3:GetBucketLocation",
          "s3:GetObject",
          "s3:ListBucket",
          "s3:ListBucketMultipartUploads",
          "s3:PutObject",
        ]
        Resource = [aws_s3_bucket.archive.arn, "${aws_s3_bucket.archive.arn}/*"]
      },
      {
        Effect   = "Allow"
        Action   = ["kinesis:DescribeStream", "kinesis:GetShardIterator", "kinesis:GetRecords", "kinesis:ListShards"]
        Resource = aws_kinesis_stream.clicks.arn
      },
      {
        Effect   = "Allow"
        Action   = ["kms:Decrypt", "kms:GenerateDataKey"]
        Resource = var.kms_key_arn
      }
    ]
  })
}

resource "aws_kinesis_firehose_delivery_stream" "archive" {
  name        = "${var.name_prefix}-click-archive"
  destination = "extended_s3"
  kinesis_source_configuration {
    kinesis_stream_arn = aws_kinesis_stream.clicks.arn
    role_arn           = aws_iam_role.firehose.arn
  }
  extended_s3_configuration {
    role_arn            = aws_iam_role.firehose.arn
    bucket_arn          = aws_s3_bucket.archive.arn
    prefix              = "year=!{timestamp:yyyy}/month=!{timestamp:MM}/day=!{timestamp:dd}/hour=!{timestamp:HH}/"
    error_output_prefix = "errors/"
    compression_format  = "GZIP"
    kms_key_arn         = var.kms_key_arn
    buffering_interval  = 300
    buffering_size      = 64
  }
  tags = var.tags
}

resource "aws_cloudwatch_metric_alarm" "iterator_age" {
  alarm_name          = "${var.name_prefix}-kinesis-iterator-age"
  alarm_description   = "Owner: platform. Action: scale shards or inspect aggregator errors."
  comparison_operator = "GreaterThanThreshold"
  evaluation_periods  = 3
  metric_name         = "GetRecords.IteratorAgeMilliseconds"
  namespace           = "AWS/Kinesis"
  period              = 60
  statistic           = "Maximum"
  threshold           = 60000
  alarm_actions       = var.alarm_actions
  dimensions          = { StreamName = aws_kinesis_stream.clicks.name }
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
