variable "region" {
  description = "AWS region"
  type        = string
  default     = "us-east-1"
}

variable "localstack_endpoint" {
  description = "LocalStack service endpoint"
  type        = string
  default     = "http://localhost:4566"
}

variable "project_name" {
  description = "Project name prefix for resources"
  type        = string
  default     = "kboot"
}

variable "vpc_cidr" {
  description = "CIDR block for the VPC"
  type        = string
  default     = "10.0.0.0/16"
}

variable "clusters" {
  description = "EKS cluster definitions"
  type = object({
    staging = object({
      name = string
    })
    production = object({
      name = string
    })
  })
  default = {
    staging = {
      name = "kboot-staging-cluster"
    }
    production = {
      name = "kboot-production-cluster"
    }
  }
}
