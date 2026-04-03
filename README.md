# kboot

> **Boot into all your EKS clusters at once.**

[![Latest Release](https://img.shields.io/github/v/release/apavanello/kboot?sort=semver&color=blue)](https://github.com/apavanello/kboot/releases)
[![Go Version](https://img.shields.io/github/go-mod/go-version/apavanello/kboot)](https://go.dev)
[![License](https://img.shields.io/github/license/apavanello/kboot)](LICENSE)
[![Go Report Card](https://goreportcard.com/badge/github.com/apavanello/kboot)](https://goreportcard.com/report/github.com/apavanello/kboot)
[![Release](https://github.com/apavanello/kboot/actions/workflows/release.yml/badge.svg)](https://github.com/apavanello/kboot/actions/workflows/release.yml)

**kboot** is a DevOps CLI tool designed to simplify managing multiple Amazon EKS clusters across different AWS accounts. It automates authentication via AWS SSO, generates context-aware kubeconfigs for selected clusters in parallel, and launches [`k9s`](https://k9scli.io/) with all clusters immediately accessible — without polluting your `~/.kube/config`.

## Table of Contents

- [Features](#features)
- [How It Works](#how-it-works)
- [Prerequisites](#prerequisites)
- [Installation](#installation)
- [Usage](#usage)
- [Configuration](#configuration)
- [Authentication](#authentication)
- [CLI Commands](#cli-commands)
- [Test Infrastructure](#test-infrastructure)
- [Testing](#testing)
- [Development](#development)
- [Project Structure](#project-structure)
- [Security](#security)
- [License](#license)

## Features

- **Unified TUI Dashboard** — Manage clusters, static credentials, and SSO profiles from a single terminal interface built with [Bubble Tea](https://github.com/charmbracelet/bubbletea)
- **Pure AWS SDK Authentication** — SSO login and EKS token generation use AWS SDK Go v2 exclusively. **No AWS CLI dependency required.**
- **Isolated AWS Credentials** — By default, kboot uses `~/.kboot/aws/` for credentials, config, and SSO cache. Your system `~/.aws/` is never touched. Set `use_system_aws: true` to use system AWS files.
- **Parallel Cluster Sync** — Authenticates and fetches cluster metadata for multiple clusters simultaneously with a configurable worker pool (default: 5 workers)
- **Context Aliasing** — Maps complex AWS ARNs (`arn:aws:eks:us-east-1:123456789012:cluster/prod`) to short, friendly aliases (`prod`, `staging`, `dev`)
- **Zero Pollution** — Does **not** modify your `~/.kube/config`. Uses temporary kubeconfig files in `/tmp/kboot/` that are cleaned up after the session
- **Non-Interactive Mode** — Target a single cluster with `--cluster <alias>` for scripting and automation
- **Headless Mode** — Generate kubeconfigs and exit without launching k9s, perfect for CI/CD pipelines
- **Cross-Platform** — Works on Windows, Linux, and macOS
- **LocalStack Test Environment** — Full Terraform + kind infrastructure for testing without touching real AWS resources
- **YOLO Installer** — One-command setup for all dependencies and test infrastructure

## How It Works

```
┌─────────────────────────────────────────────────────────┐
│  kboot                                                  │
│                                                         │
│  1. Load ~/.kboot.yaml → list of clusters               │
│  2. For each cluster (parallel, 5 workers):             │
│     a. Create AWS SDK client (profile + region)         │
│     b. Check identity (STS GetCallerIdentity)           │
│     c. If auth fails → SSO login (device auth flow)     │
│     d. Describe cluster (EKS API) → endpoint + CA       │
│  3. Generate kubeconfig per cluster (temp files)        │
│  4. Launch k9s with KUBECONFIG=temp1:temp2:...          │
│  5. Exec replaces kboot process with k9s                │
└─────────────────────────────────────────────────────────┘
```

## Prerequisites

| Dependency | Required | Purpose |
|---|---|---|
| **Go 1.21+** | Build only | Compile from source |
| **k9s** | Runtime (recommended) | Terminal Kubernetes UI |
| **AWS SDK Go v2** | Runtime (vendored) | Authentication, EKS API — no external CLI needed |

> **Note:** kboot does **not** require the AWS CLI. All AWS interactions use the AWS SDK Go v2 directly, including SSO login via the OIDC device authorization grant flow.

## Installation

### Quick Install (Recommended)

One command — downloads the latest release binary and installs to `~/.local/bin`:

```bash
curl -fsSL https://raw.githubusercontent.com/apavanello/kboot/main/scripts/install.sh | bash
```

### Full Setup (YOLO)

Installs all dependencies (Go, Docker, kubectl, kind, Terraform, k9s), builds kboot, sets up LocalStack + kind clusters, and configures everything:

```bash
curl -fsSL https://raw.githubusercontent.com/apavanello/kboot/main/scripts/install.sh | bash -s full
```

### YOLO Installer Commands

| Command | What it does |
|---|---|
| `curl ... \| bash` | Download latest release binary (or build from source if no release) |
| `curl ... \| bash -s update` | Update to latest release if newer version available |
| `curl ... \| bash -s full` | Install all dependencies + kboot + setup test infrastructure |
| `curl ... \| bash -s help` | Show all available options |

### Build from Source

```bash
git clone https://github.com/apavanello/kboot
cd kboot
make install      # Build + install to ~/.local/bin
```

### Using Make

```bash
make build        # Compile to ./bin/kboot
make install      # Install to ~/.local/bin
make run          # Build and launch immediately
```

### Direct Download

Download the latest binary from [Releases](https://github.com/apavanello/kboot/releases) and place it in your `PATH`.

## Usage

### Launch k9s with All Clusters

```bash
./kboot
```

Opens the TUI loading screen, authenticates to all configured clusters in parallel, generates temporary kubeconfigs, and launches k9s.

### Target a Single Cluster

```bash
./kboot --cluster staging
```

Skips the TUI cluster selection and boots only the specified cluster. Implies `--non-interactive`.

### Headless Mode (Scripting / CI/CD)

```bash
./kboot --headless
```

Generates kubeconfigs and prints the `KUBECONFIG` path to stdout. Does not launch k9s:

```bash
export KUBECONFIG=$(./kboot --headless)
kubectl get nodes --all-contexts
```

### Non-Interactive Mode

```bash
./kboot --non-interactive
```

Processes all configured clusters without TUI prompts. Shows kubeconfig paths on completion.

## Configuration

### Cluster Configuration

Clusters are stored in `~/.kboot.yaml`:

```yaml
clusters:
  - alias: "prod"
    name: "eks-cluster-production"
    region: "us-east-1"
    profile: "aws-prod"
    optional: false

  - alias: "staging"
    name: "eks-cluster-staging"
    region: "us-east-1"
    profile: "aws-staging"
    optional: true
```

| Field | Required | Description |
|---|---|---|
| `alias` | Yes | Short friendly name shown in TUI and used as k9s context |
| `name` | Yes | Actual EKS cluster name as registered in AWS |
| `region` | Yes | AWS region where the cluster is deployed |
| `profile` | Yes | AWS credentials profile name |
| `optional` | No | If `true`, cluster can be skipped at launch without error |

### Isolated AWS Credentials

By default, kboot uses its own isolated AWS directory at `~/.kboot/aws/`:

```
~/.kboot/
├── aws/
│   ├── credentials     # AWS access keys
│   ├── config          # AWS config with endpoint overrides
│   └── sso/cache/      # SSO token cache
└── .kboot.yaml         # Cluster definitions
```

This keeps kboot completely isolated from your system `~/.aws/` configuration.

To use your system AWS files instead, add to `~/.kboot.yaml`:

```yaml
use_system_aws: true
```

Or specify custom paths:

```yaml
aws_credentials_file: /path/to/credentials
aws_config_file: /path/to/config
aws_sso_cache_dir: /path/to/sso/cache
```

### CLI Configuration

Add clusters directly from the command line:

```bash
kboot config add --alias prod --name my-cluster --region us-east-1 --profile aws-prod
kboot config list
kboot config              # Opens TUI manager
```

## Authentication

kboot supports two authentication methods, both handled entirely through the AWS SDK Go v2:

### 1. SSO Authentication (Recommended)

When you configure an SSO profile in your AWS config:

```ini
[profile my-sso-profile]
sso_session = my-sso
sso_account_id = 123456789012
sso_role_name = EKSAdmin
region = us-east-1

[sso-session my-sso]
sso_start_url = https://my-company.awsapps.com/start
sso_region = us-east-1
```

kboot will:
1. Check if a valid SSO token exists in the cache
2. If not, initiate the **OAuth 2.0 Device Authorization Grant** flow
3. Open your browser to the AWS SSO login page
4. Poll for token approval
5. Cache the token for future use

### 2. Static Credentials

Configure access keys in your credentials file:

```ini
[aws-prod]
aws_access_key_id = AKIAIOSFODNN7EXAMPLE
aws_secret_access_key = wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY
```

### EKS Token Generation

For kubeconfig authentication, kboot generates presigned STS `GetCallerIdentity` tokens using AWS Signature V4. The `x-k8s-aws-id` header is signed as part of the canonical request, producing a valid `k8s-aws-v1.*` token that the EKS API server accepts. This replaces the traditional `aws eks get-token` CLI call.

## CLI Commands

### `kboot` — Launch

```bash
./kboot                           # Interactive mode with cluster selection
./kboot --cluster staging         # Target single cluster
./kboot --non-interactive         # Process all clusters without TUI
./kboot --headless                # Generate kubeconfigs and print path
```

### `kboot config` — Manage

```bash
kboot config                      # Open TUI manager
kboot config list                 # List all configured clusters
kboot config add --alias X --name Y --region Z --profile P
```

### `kboot token` — Generate EKS Auth Token

Internal command used as the kubeconfig exec plugin:

```bash
./kboot token --cluster-name my-cluster --region us-east-1 --profile my-profile
```

Output:
```json
{
  "kind": "ExecCredential",
  "apiVersion": "client.authentication.k8s.io/v1beta1",
  "status": {
    "token": "k8s-aws-v1.aHR0cHM6Ly9zdHMudXMtZWFzdC0xLmFtYXpvbmF3cy5jb20v..."
  }
}
```

## Test Infrastructure

kboot includes a complete test environment using **LocalStack** (mock AWS) and **kind** (real Kubernetes):

```bash
make infra           # Start LocalStack + create 2 kind clusters
make infra-status    # Show current infrastructure status
make infra-cleanup   # Tear down everything
make docker-up       # Start LocalStack container only
make docker-down     # Stop LocalStack container
```

The test environment provisions:
- **LocalStack** — Mock AWS EKS, IAM, STS, EC2, S3, CloudWatch
- **kind staging cluster** — Real Kubernetes on port 6443
- **kind production cluster** — Real Kubernetes on port 6444
- **Terraform** — EKS cluster definitions in LocalStack

See [`infra/`](infra/) for all configuration files.

## Testing

```bash
make test              # Go unit tests with race detection
make test-verbose      # Unit tests with verbose output
make test-coverage     # Unit tests with coverage report
make test-e2e          # CLI unit tests (config CRUD, flags, token)
make test-integration  # Full integration suite (11 phases)
```

### Integration Test Phases

| Phase | What It Tests |
|---|---|
| 1. Build | Binary compiles and is executable |
| 2. Prerequisites | Docker, kind, Terraform, kubectl available |
| 3. LocalStack | Container running, EKS service healthy |
| 4. Terraform EKS | Mock clusters created in LocalStack |
| 5. kind Clusters | Real Kubernetes clusters reachable |
| 6. Isolated AWS | `~/.kboot/aws/` credentials and config |
| 7. kboot Config | Clusters added and listed via CLI |
| 8. Non-Interactive | `--cluster` and `--non-interactive` flags |
| 9. Token Generation | `k8s-aws-v1.*` token with signed headers |
| 10. kubectl | Node and pod connectivity on both clusters |
| 11. Multi-context | Combined kubeconfig with both contexts |

## Development

### Makefile Targets

```bash
make build           # Compile to ./bin/kboot
make run             # Build and launch
make install         # Install to $GOPATH/bin
make clean           # Remove build artifacts

make fmt             # Format source with gofmt
make vet             # Run go vet
make lint            # Run golangci-lint
make check           # fmt + vet + lint
make tidy            # Clean up go.mod

make test            # Run tests with race detection
make test-verbose    # Run tests (verbose output)
make test-coverage   # Run tests with coverage report
make test-e2e        # CLI unit tests
make test-integration # Full integration suite

make install-yolo    # Automated install of all dependencies + infra

make infra           # Setup LocalStack + kind test environment
make infra-cleanup   # Tear down test environment
make infra-status    # Show current infra status
make infra-destroy   # Full destroy (Terraform + kind)
make docker-up       # Start LocalStack container
make docker-down     # Stop LocalStack container

make help            # Show all available targets
```

### Project Structure

```
kboot/
├── cmd/kboot/
│   └── main.go              # CLI entry point, flag parsing
├── internal/
│   ├── app/
│   │   └── orchestrator.go  # Parallel worker pool
│   ├── aws/
│   │   ├── client.go        # AWS SDK client + EKS token (v4 presign)
│   │   └── sso.go           # SSO OIDC device authorization
│   ├── config/
│   │   └── config.go        # Config loading + isolated AWS paths
│   ├── kube/
│   │   └── generator.go     # Kubeconfig YAML generation
│   └── ui/
│       ├── dashboard.go     # Loading screen
│       └── manager.go       # TUI manager
├── infra/
│   ├── main.tf              # Terraform: VPC, IAM, 2 EKS clusters
│   ├── variables.tf         # Terraform variables
│   ├── outputs.tf           # Terraform outputs
│   ├── bootstrap.sh         # Setup script
│   ├── docker-compose.yml   # LocalStack container
│   ├── kind-staging.yaml    # kind staging cluster config
│   └── kind-production.yaml # kind production cluster config
├── scripts/
│   ├── install.sh           # YOLO installer
│   ├── test-e2e.sh          # CLI unit tests
│   └── test-integration.sh  # Full integration suite
├── Makefile
├── go.mod
├── .kboot.yaml.example
├── README.md
└── README.pt-br.md
```

## Security

- **No credential storage** — kboot never stores AWS access keys or secrets. All credential management is delegated to the AWS SDK and standard AWS config files.
- **Isolated by default** — kboot uses `~/.kboot/aws/` for all AWS files, leaving your system `~/.aws/` untouched.
- **Temporary kubeconfigs** — Generated kubeconfig files are stored in `/tmp/kboot/` and cleaned up after the session. Your `~/.kube/config` is never modified.
- **SSO token caching** — SSO tokens are cached with proper file permissions (0600) and expiration tracking, following the AWS SDK standard format.
- **Presigned tokens** — EKS authentication tokens are generated via presigned STS requests with a 60-second expiration, minimizing the window for token replay attacks.
- **No shell injection** — All AWS interactions use the SDK directly. No subprocess calls to `aws` CLI eliminate shell injection vectors.

## License

Apache-2.0
