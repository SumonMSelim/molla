terraform {
  backend "s3" {
    bucket       = "molla-tfstate-dev"
    key          = "aws/dev/terraform.tfstate"
    region       = "us-east-1"
    use_lockfile = true
    encrypt      = true
  }
}
