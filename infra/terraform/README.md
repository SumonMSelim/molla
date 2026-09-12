# Terraform

AWS is molla's first deployment target. This directory currently contains only Terraform and AWS provider version constraints; deployable modules and environment roots have not been added.

Validate the current configuration from the repository root:

```sh
make tf-check
```

This runs formatting checks, initializes without a backend, and validates each available module and environment. It does not access an AWS account or apply infrastructure.

Terraform plan and apply instructions will be documented after deployable resources exist. Production changes will require reviewed plan output, short-lived AWS credentials, and explicit approval.
