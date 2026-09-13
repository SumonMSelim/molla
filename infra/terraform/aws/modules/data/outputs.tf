output "table_names" {
  value = { for k, t in aws_dynamodb_table.this : k => t.name }
}

output "table_arns" {
  value = { for k, t in aws_dynamodb_table.this : k => t.arn }
}

output "links_table_name" {
  value = aws_dynamodb_table.this["links"].name
}

output "stats_table_name" {
  value = aws_dynamodb_table.this["stats"].name
}

output "counters_table_name" {
  value = aws_dynamodb_table.this["counters"].name
}

output "idempotency_table_name" {
  value = aws_dynamodb_table.this["idempotency"].name
}

output "credentials_table_name" {
  value = aws_dynamodb_table.this["credentials"].name
}
