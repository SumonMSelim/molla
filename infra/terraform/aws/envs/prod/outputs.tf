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

output "admin_role_arn" {
  value = module.api.admin_role_arn
}

output "invalidate_function_name" {
  value = module.api.invalidate_function_name
}

output "invalidate_function_arn" {
  value = module.api.invalidate_function_arn
}
