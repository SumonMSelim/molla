terraform {
  backend "s3" {
    bucket       = "molla-tfstate-prod"
    key          = "aws/prod/terraform.tfstate"
    region       = "us-east-1"
    use_lockfile = true
    encrypt      = true
  }
}
