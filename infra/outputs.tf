output "staging_cluster_name" {
  value       = aws_eks_cluster.staging.name
  description = "Staging EKS cluster name"
}

output "staging_cluster_endpoint" {
  value       = aws_eks_cluster.staging.endpoint
  description = "Staging EKS cluster endpoint"
}

output "staging_cluster_arn" {
  value       = aws_eks_cluster.staging.arn
  description = "Staging EKS cluster ARN"
}

output "production_cluster_name" {
  value       = aws_eks_cluster.production.name
  description = "Production EKS cluster name"
}

output "production_cluster_endpoint" {
  value       = aws_eks_cluster.production.endpoint
  description = "Production EKS cluster endpoint"
}

output "production_cluster_arn" {
  value       = aws_eks_cluster.production.arn
  description = "Production EKS cluster ARN"
}

output "kubeconfig_hint" {
  value       = <<EOT

To configure kboot for LocalStack clusters, add to ~/.kboot.yaml:

clusters:
  - alias: "staging"
    name: "${aws_eks_cluster.staging.name}"
    region: "${var.region}"
    profile: "localstack"

  - alias: "production"
    name: "${aws_eks_cluster.production.name}"
    region: "${var.region}"
    profile: "localstack"

Then add a 'localstack' profile to ~/.aws/credentials:
  [localstack]
  aws_access_key_id = test
  aws_secret_access_key = test

And to ~/.aws/config:
  [profile localstack]
  region = ${var.region}
  endpoint_url = ${var.localstack_endpoint}
EOT
  description = "Instructions for configuring kboot with LocalStack clusters"
}
