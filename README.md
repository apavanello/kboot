# kboot

**kboot** is a DevOps CLI tool designed to simplify managing multiple Amazon EKS clusters across different AWS accounts. It automates authentication via AWS SSO, generates context-aware kubeconfigs for selected clusters in parallel, and launches `k9s` (or a shell) with all clusters immediately accessible.

## Features

- **Unified TUI Dashboard** (v2.3.0): Manage clusters and AWS credentials from a single, intuitive terminal interface
- **Smart Authentication**: Checks if your AWS SSO session is valid. If expired, it runs `aws sso login` automatically
- **Parallel Sync**: Fetches cluster details and generates kubeconfigs for multiple clusters simultaneously using Goroutines
- **Context Aliasing**: Maps complex AWS ARNs to short, friendly aliases (e.g., `prod`, `staging`)
- **Zero Pollution**: Does **not** modify your main `~/.kube/config`. It generates temporary configs for the session
- **Cross-Platform**: Works on Windows, Linux, and macOS

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

## Usage

### Launch k9s with all clusters
```bash
./kboot
```
Syncs all configured clusters and opens k9s.

### Configuration Dashboard (TUI)
```bash
./kboot config
```
Opens the unified management dashboard where you can:

**Manage Clusters:**
- `a` - Add new cluster
- `e` / `Enter` - Edit selected cluster
- `c` - Duplicate selected cluster
- `d` - Delete selected cluster

**Manage AWS Credentials:**
- Static credentials (`~/.aws/credentials`)
- SSO profiles (`~/.aws/config`)

**Navigation:**
- `Tab` / `Shift+Tab` - Navigate form fields
- `Esc` - Go back / Cancel
- `q` - Quit

## Configuration

Clusters are stored in `~/.kboot.yaml`:

```yaml
sso_session: "my-sso-session"
clusters:
  - alias: "prod"
    profile: "aws-prod"
    region: "us-east-1"
    name: "eks-cluster-production"
  - alias: "staging"
    profile: "aws-staging"
    region: "us-east-1"
    name: "eks-cluster-staging"
```

You can also manage this file through the TUI with `kboot config`.

## License

MIT
