output "dlq_arn" {
  value = aws_sqs_queue.aggregate_dlq.arn
}

output "aggregate_function_name" {
  value = aws_lambda_function.aggregate.function_name
}
