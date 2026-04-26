############################
# General
############################

variable "project_name" {
  description = "Project identifier used for resource naming"
  type        = string
  default     = "astrophage-hwarr"
}

variable "environment" {
  description = "Deployment environment (dev / prod)"
  type        = string
  default     = "prod"
}

variable "aws_region" {
  description = "AWS region to deploy resources"
  type        = string
  default     = "ap-northeast-2"
}

############################
# Networking
############################

variable "vpc_cidr" {
  description = "CIDR block for the VPC"
  type        = string
  default     = "10.0.0.0/16"
}

variable "public_subnet_cidrs" {
  description = "CIDR blocks for public subnets (ALB)"
  type        = list(string)
  default     = ["10.0.1.0/24", "10.0.2.0/24"]
}

variable "private_subnet_cidrs" {
  description = "CIDR blocks for private subnets (ECS, ElastiCache)"
  type        = list(string)
  default     = ["10.0.10.0/24", "10.0.11.0/24"]
}

variable "availability_zones" {
  description = "AZs to spread subnets across"
  type        = list(string)
  default     = ["ap-northeast-2a", "ap-northeast-2c"]
}

############################
# ECS / Fargate
############################

variable "ecs_cpu" {
  description = "CPU units for the Fargate task (1 vCPU = 1024)"
  type        = number
  default     = 256
}

variable "ecs_memory" {
  description = "Memory (MiB) for the Fargate task"
  type        = number
  default     = 512
}

variable "ecs_desired_count" {
  description = "Number of ECS tasks to run"
  type        = number
  default     = 1
}

variable "container_port" {
  description = "Port the backend container listens on"
  type        = number
  default     = 8000
}

variable "ecr_image_tag" {
  description = "Docker image tag to deploy (overridden by CI/CD)"
  type        = string
  default     = "latest"
}

############################
# ElastiCache (Redis)
############################

variable "redis_node_type" {
  description = "ElastiCache node type (single-node). cache.t4g.micro = 0.5GB, cache.t4g.small = 1.37GB"
  type        = string
  default     = "cache.t4g.micro"
}

variable "redis_seed_snapshot_arn" {
  description = "S3 ARN of an RDB snapshot to seed the new cluster on first create (e.g. arn:aws:s3:::bucket/dump.rdb). Leave empty for no seed."
  type        = string
  default     = ""
}

############################
# S3 / CloudFront
############################

variable "frontend_bucket_name" {
  description = "S3 bucket name for frontend static assets"
  type        = string
  default     = "astrophage-hwarr-frontend-unique-suffix"
}

############################
# Domain
############################

variable "domain_name" {
  description = "Root domain name (e.g. hwarr.com)"
  type        = string
  default     = "hwarr.com"
}

############################
# Budget
############################

variable "budget_limit_usd" {
  description = "Monthly budget limit in USD — alerts at 50%, 80%, 100%"
  type        = string
  default     = "50"
}

variable "budget_alert_email" {
  description = "Email to receive budget alerts"
  type        = string
  default     = "isac7722@gmail.com"
}

