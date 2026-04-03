# kboot

**kboot** is a DevOps CLI tool designed to simplify managing multiple Amazon EKS clusters across different AWS accounts. It automates authentication via AWS SSO, generates context-aware kubeconfigs for selected clusters in parallel, and launches `k9s` with all clusters immediately accessible.

## Features

- **Unified TUI Dashboard** (v2.3): Manage clusters and AWS credentials from a single, intuitive terminal interface
- **Smart Authentication**: Automatically validates SSO sessions using pure AWS SDK (no AWS CLI required)
- **Parallel Sync**: Generates kubeconfigs for multiple clusters simultaneously
- **Context Aliasing**: Maps complex AWS ARNs to short, friendly aliases (e.g., `prod`, `staging`)
- **Zero Pollution**: Does **not** modify your `~/.kube/config`. Uses temporary configs for the session
- **Cross-Platform**: Works on Windows, Linux, and macOS

## Installation

### Prerequisites
- Go 1.21+
- `k9s` (recommended)

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

### Configuration Dashboard
```bash
./kboot config
```

Opens the unified TUI with three options:

| Menu | Description |
|------|-------------|
| **Gerenciar Clusters** | Add, edit, duplicate, or delete EKS clusters |
| **Credenciais Estáticas** | Manage `~/.aws/credentials` profiles |
| **Perfis SSO** | Manage `~/.aws/config` SSO profiles |

### Keybindings

| Key | Action |
|-----|--------|
| `a` | Add new item |
| `e` / `Enter` | Edit selected |
| `c` | Duplicate selected |
| `d` | Delete selected |
| `Tab` | Next field |
| `Shift+Tab` | Previous field |
| `Esc` | Back / Cancel |
| `q` | Quit |

## Configuration

Clusters are stored in `~/.kboot.yaml`:

```yaml
clusters:
  - alias: "prod"
    name: "eks-cluster-production"
    region: "us-east-1"
    profile: "aws-prod"
  - alias: "staging"
    name: "eks-cluster-staging"
    region: "us-east-1"
    profile: "aws-staging"
```

> **Note:** AWS credentials and SSO profiles are managed separately in `~/.aws/credentials` and `~/.aws/config`. Use `kboot config` to configure everything from the TUI.

## License

MIT
