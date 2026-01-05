# kboot

**kboot** is a DevOps CLI tool designed to simplify managing multiple Amazon EKS clusters across different AWS accounts. It automates authentication via AWS SSO, generates context-aware kubeconfigs for selected clusters in parallel, and launches `k9s` (or a shell) with all clusters immediately accessible.

## Features

- **Smart Authentication**: Checks if your AWS SSO session is valid in `~/.aws/sso/cache`. If expired, it runs `aws sso login` automatically.
- **Parallel Sync**: Fetches cluster details and generates kubeconfigs for multiple clusters simultaneously using Goroutines.
- **Context Aliasing**: Maps complex AWS ARNs to short, friendly aliases (e.g., `prod`, `staging`) for your Kubernetes contexts.
- **Zero Pollution**: Does **not** modify your main `~/.kube/config`. It generates temporary configs and sets the `KUBECONFIG` environment variable for the session.
- **Cross-Platform**: Works on Windows, Linux, and macOS.

## Installation

### Prerequisites
- Go 1.21+
- AWS CLI v2
- `k9s` (recommended) or `kubectl`

### Build from Source
```bash
git clone https://github.com/apavanello/kboot
cd kboot
go build -o kboot .
```

## Configuration

Create a configuration file at `~/.kboot.yaml`:

```yaml
sso_session: "my-sso-session" # Must match your ~/.aws/config session name
clusters:
  - alias: "prod"         # Short name for context (e.g. displayed in k9s)
    profile: "aws-prod"   # AWS Profile from ~/.aws/config
    region: "us-east-1"
    name: "eks-cluster-production" # Real EKS cluster name
  - alias: "staging"
    profile: "aws-staging"
    region: "us-east-1"
    name: "eks-cluster-staging"
```

## Usage

Simply run the binary:

```bash
./kboot
```

It will:
1. Ensure you are logged in to AWS SSO.
2. Generate kubeconfigs for `prod` and `staging` in a temp directory.
3. Launch `k9s` with access to both clusters.

## License

MIT
