terraform {
  backend "s3" {
    bucket         = "molla-tfstate-prod"
    key            = "aws/prod/terraform.tfstate"
    region         = "us-east-1"
    dynamodb_table = "molla-tf-locks-prod"
    encrypt        = true
  }
}
