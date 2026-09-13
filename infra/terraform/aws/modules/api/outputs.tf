output "rest_api_id" {
  value = aws_api_gateway_rest_api.this.id
}

output "invoke_url" {
  value = aws_api_gateway_stage.live.invoke_url
}

output "redirect_function_url" {
  value = aws_lambda_function_url.redirect.function_url
}

output "redirect_function_arn" {
  value = aws_lambda_function.redirect.arn
}

output "redirect_alias_arn" {
  value = aws_lambda_alias.redirect_live.arn
}

output "invalidate_function_arn" {
  value = aws_lambda_function.invalidate.arn
}

output "admin_role_arn" {
  value = aws_iam_role.admin.arn
}

output "api_execution_role_arn" {
  value = aws_iam_role.api.arn
}

output "redis_endpoint" {
  value = aws_elasticache_replication_group.redis.primary_endpoint_address
}

output "vpc_id" {
  value = aws_vpc.this.id
}

output "invalidate_function_name" {
  value = aws_lambda_function.invalidate.function_name
}
