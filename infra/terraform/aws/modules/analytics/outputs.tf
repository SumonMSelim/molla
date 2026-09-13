output "stream_name" {
  value = aws_kinesis_stream.clicks.name
}

output "stream_arn" {
  value = aws_kinesis_stream.clicks.arn
}

output "archive_bucket" {
  value = aws_s3_bucket.archive.id
}

output "dlq_arn" {
  value = aws_sqs_queue.aggregate_dlq.arn
}

output "aggregate_function_name" {
  value = aws_lambda_function.aggregate.function_name
}
