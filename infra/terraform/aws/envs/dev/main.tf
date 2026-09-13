locals {
  name_prefix = "molla-dev"
  tags = {
    Workload       = "molla"
    Environment    = "dev"
    Owner          = "platform"
    CostCenter     = "molla"
    CostAllocation = "dev"
  }
}

resource "aws_kms_key" "this" {
  description             = "molla dev"
  deletion_window_in_days = 7
  enable_key_rotation     = true
  tags                    = local.tags
}

resource "aws_kms_alias" "this" {
  name          = "alias/${local.name_prefix}"
  target_key_id = aws_kms_key.this.id
}

module "data" {
  source      = "../../modules/data"
  name_prefix = local.name_prefix
  enable_pitr = false
  kms_key_arn = aws_kms_key.this.arn
  tags        = local.tags
}

module "analytics" {
  source           = "../../modules/analytics"
  name_prefix      = local.name_prefix
  kms_key_arn      = aws_kms_key.this.arn
  artifact_dir     = var.artifact_dir
  stats_table_arn  = module.data.table_arns["stats"]
  stats_table_name = module.data.stats_table_name
  shard_count      = 1
  tags             = local.tags
}

module "api" {
  source                  = "../../modules/api"
  name_prefix             = local.name_prefix
  kms_key_arn             = aws_kms_key.this.arn
  artifact_dir            = var.artifact_dir
  table_arns              = module.data.table_arns
  table_names             = module.data.table_names
  stream_arn              = module.analytics.stream_arn
  stream_name             = module.analytics.stream_name
  redis_node_type         = "cache.t4g.micro"
  redis_auth_token        = var.redis_auth_token
  permutation_key         = var.permutation_key
  privacy_key             = var.privacy_key
  provisioned_concurrency = 1
  admin_principal_arns    = var.admin_principal_arns
  tags                    = local.tags
}

module "edge" {
  source                = "../../modules/edge"
  name_prefix           = local.name_prefix
  kms_key_arn           = aws_kms_key.this.arn
  api_gateway_id        = module.api.rest_api_id
  redirect_function_url = module.api.redirect_function_url
  redirect_function_arn = module.api.redirect_function_arn
  enable_waf            = false
  tags                  = local.tags
}
