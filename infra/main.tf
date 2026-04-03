terraform {
  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 5.0"
    }
  }

  backend "local" {
    path = ".terraform/state/terraform.tfstate"
  }
}

provider "aws" {
  access_key                  = "test"
  secret_key                  = "test"
  region                      = var.region
  s3_use_path_style           = true
  skip_credentials_validation = true
  skip_metadata_api_check     = true
  skip_requesting_account_id  = true

  endpoints {
    eks        = var.localstack_endpoint
    iam        = var.localstack_endpoint
    sts        = var.localstack_endpoint
    ec2        = var.localstack_endpoint
    s3         = var.localstack_endpoint
    cloudwatch = var.localstack_endpoint
  }
}

# ─── IAM Role for EKS ──────────────────────────────────────────────
resource "aws_iam_role" "eks_cluster_role" {
  name = "${var.project_name}-cluster-role"

  assume_role_policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Action = "sts:AssumeRole"
        Effect = "Allow"
        Principal = {
          Service = "eks.amazonaws.com"
        }
      }
    ]
  })
}

resource "aws_iam_role_policy_attachment" "eks_cluster_policy" {
  policy_arn = "arn:aws:iam::aws:policy/AmazonEKSClusterPolicy"
  role       = aws_iam_role.eks_cluster_role.name
}

# ─── EKS Cluster: Staging ──────────────────────────────────────────
resource "aws_eks_cluster" "staging" {
  name     = var.clusters.staging.name
  role_arn = aws_iam_role.eks_cluster_role.arn

  vpc_config {
    endpoint_private_access = false
    endpoint_public_access  = true
    subnet_ids              = aws_subnet.eks[*].id
  }

  tags = {
    Environment = "staging"
    ManagedBy   = "terraform"
  }
}

# ─── EKS Cluster: Production ───────────────────────────────────────
resource "aws_eks_cluster" "production" {
  name     = var.clusters.production.name
  role_arn = aws_iam_role.eks_cluster_role.arn

  vpc_config {
    endpoint_private_access = false
    endpoint_public_access  = true
    subnet_ids              = aws_subnet.eks[*].id
  }

  tags = {
    Environment = "production"
    ManagedBy   = "terraform"
  }
}

# ─── VPC & Networking (minimal for LocalStack) ─────────────────────
resource "aws_vpc" "eks" {
  cidr_block           = var.vpc_cidr
  enable_dns_support   = true
  enable_dns_hostnames = true

  tags = {
    Name = "${var.project_name}-vpc"
  }
}

resource "aws_subnet" "eks" {
  count             = 2
  vpc_id            = aws_vpc.eks.id
  cidr_block        = cidrsubnet(var.vpc_cidr, 8, count.index)
  availability_zone = "${var.region}a"

  tags = {
    Name = "${var.project_name}-subnet-${count.index}"
  }
}

resource "aws_internet_gateway" "eks" {
  vpc_id = aws_vpc.eks.id

  tags = {
    Name = "${var.project_name}-igw"
  }
}

resource "aws_route_table" "eks" {
  vpc_id = aws_vpc.eks.id

  route {
    cidr_block = "0.0.0.0/0"
    gateway_id = aws_internet_gateway.eks.id
  }

  tags = {
    Name = "${var.project_name}-rt"
  }
}

resource "aws_route_table_association" "eks" {
  count          = 2
  subnet_id      = aws_subnet.eks[count.index].id
  route_table_id = aws_route_table.eks.id
}
