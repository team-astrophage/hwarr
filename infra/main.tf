############################
# Data Sources
############################

data "aws_caller_identity" "current" {}

data "aws_region" "current" {}

############################
# Local values
############################

locals {
  name_prefix = "${var.project_name}-${var.environment}"
  account_id  = data.aws_caller_identity.current.account_id
  region      = data.aws_region.current.name
}
