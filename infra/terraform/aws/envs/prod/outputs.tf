output "ui_bucket" {
  value = module.edge.ui_bucket
}

output "distribution_id" {
  value = module.edge.distribution_id
}

output "distribution_domain" {
  value = module.edge.distribution_domain
}

output "api_invoke_url" {
  value = module.api.invoke_url
}

output "usage_plan_id" {
  value = module.api.usage_plan_id
}

output "api_key_id" {
  value = module.api.api_key_id
}

output "admin_role_arn" {
  value = module.api.admin_role_arn
}

output "invalidate_function_name" {
  value = module.api.invalidate_function_name
}

output "invalidate_function_arn" {
  value = module.api.invalidate_function_arn
}

output "gha_plan_role_arn" {
  description = "Set as the AWS_PLAN_ROLE_ARN GitHub Actions variable."
  value       = module.ci.plan_role_arn
}

output "gha_apply_role_arn" {
  description = "Set as the AWS_APPLY_ROLE_ARN GitHub Actions variable."
  value       = module.ci.apply_role_arn
}
