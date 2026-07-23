# kboot

<p align="center">
  <img src="./assets/readme/hero.svg" width="100%" alt="kboot — Boot into all your EKS clusters at once">
</p>

[![Latest Release](https://img.shields.io/github/v/release/apavanello/kboot?sort=semver&color=blue)](https://github.com/apavanello/kboot/releases)
[![Go Version](https://img.shields.io/github/go-mod/go-version/apavanello/kboot)](https://go.dev)
[![License](https://img.shields.io/github/license/apavanello/kboot)](LICENSE)
[![Go Report Card](https://goreportcard.com/badge/github.com/apavanello/kboot)](https://goreportcard.com/report/github.com/apavanello/kboot)
[![Release](https://github.com/apavanello/kboot/actions/workflows/release.yml/badge.svg)](https://github.com/apavanello/kboot/actions/workflows/release.yml)

## What is kboot?

**kboot** is a DevOps CLI tool that simplifies managing multiple Amazon EKS clusters across different AWS accounts. It automates authentication via AWS SSO, generates context-aware kubeconfigs for selected clusters in parallel, and launches [`k9s`](https://k9scli.io/) with all clusters immediately accessible — without polluting your `~/.kube/config`.

**No AWS CLI dependency required.** All AWS interactions use the AWS SDK Go v2 directly.

## Install

One command — downloads the latest release binary and installs to `~/.local/bin`:

```bash
curl -fsSL https://raw.githubusercontent.com/apavanello/kboot/main/scripts/install.sh | bash
```

Or build from source:

```bash
git clone https://github.com/apavanello/kboot
cd kboot
make install
```

<details>
<summary>All installation options</summary>

### Full Setup (YOLO)

Installs all dependencies (Go, Docker, kubectl, kind, Terraform, k9s), builds kboot, sets up LocalStack + kind clusters:

```bash
curl -fsSL https://raw.githubusercontent.com/apavanello/kboot/main/scripts/install.sh | bash -s full
```

### Installer Commands

| Command | What it does |
|---|---|
| `curl ... \| bash` | Download latest release binary |
| `curl ... \| bash -s update` | Update to latest release |
| `curl ... \| bash -s full` | Install all dependencies + kboot + test infra |
| `curl ... \| bash -s help` | Show all options |

### Using Make

```bash
make build        # Compile to ./bin/kboot
make install      # Install to ~/.local/bin
make run          # Build and launch immediately
```

### Direct Download

Download from [Releases](https://github.com/apavanello/kboot/releases) and place in your `PATH`.

</details>

## Usage

### Launch k9s with all clusters

```bash
kboot
```

Opens the TUI, authenticates to all configured clusters in parallel, generates temporary kubeconfigs, and launches k9s.

### Target a single cluster

```bash
kboot --cluster staging
```

### Headless mode (CI/CD)

```bash
export KUBECONFIG=$(kboot --headless)
kubectl get nodes --all-contexts
```

### Non-interactive mode

```bash
kboot --non-interactive
```

## How it works

<p align="center">
  <img src="./assets/readme/workflow.svg" width="100%" alt="kboot workflow diagram">
</p>

1. Load `~/.kboot.yaml` → list of clusters
2. For each cluster (parallel, 5 workers):
   - Create AWS SDK client (profile + region)
   - Check identity (STS GetCallerIdentity)
   - If auth fails → SSO login (device auth flow)
   - Describe cluster (EKS API) → endpoint + CA
3. Generate kubeconfig per cluster (temp files)
4. Launch k9s with `KUBECONFIG=temp1:temp2:...`
5. Exec replaces kboot process with k9s

## Configuration

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
| `alias` | Yes | Short name shown in TUI and used as k9s context |
| `name` | Yes | Actual EKS cluster name in AWS |
| `region` | Yes | AWS region |
| `profile` | Yes | AWS credentials profile name |
| `optional` | No | If `true`, can be skipped without error |

<details>
<summary>Isolated AWS credentials (default)</summary>

kboot uses its own isolated AWS directory at `~/.kboot/aws/`:

```
~/.kboot/
├── aws/
│   ├── credentials     # AWS access keys
│   ├── config          # AWS config with endpoint overrides
│   └── sso/cache/      # SSO token cache
└── .kboot.yaml         # Cluster definitions
```

Your system `~/.aws/` is never touched.

To use system AWS files instead:

```yaml
use_system_aws: true
```

Or specify custom paths:

```yaml
aws_credentials_file: /path/to/credentials
aws_config_file: /path/to/config
aws_sso_cache_dir: /path/to/sso/cache
```

</details>

<details>
<summary>CLI configuration</summary>

```bash
kboot config add --alias prod --name my-cluster --region us-east-1 --profile aws-prod
kboot config list
kboot config              # Opens TUI manager
```

</details>

## Security

<p align="center">
  <img src="./assets/readme/security.svg" width="100%" alt="kboot security isolation diagram">
</p>

| Feature | Description |
|---|---|
| **No credential storage** | All credential management delegated to AWS SDK |
| **Isolated by default** | Uses `~/.kboot/aws/`, never touches `~/.aws/` |
| **Temporary kubeconfigs** | Stored in `/tmp/kboot/`, cleaned after session |
| **No shell injection** | All AWS interactions use SDK directly |
| **Presigned tokens** | 60-second expiration minimizes replay window |

## Authentication

### SSO Authentication (Recommended)

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
1. Check if a valid SSO token exists in cache
2. If not, initiate OAuth 2.0 Device Authorization Grant flow
3. Open browser to AWS SSO login page
4. Poll for token approval
5. Cache token for future use

### Static Credentials

```ini
[aws-prod]
aws_access_key_id = AKIAIOSFODNN7EXAMPLE
aws_secret_access_key = wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY
```

## CLI Commands

```bash
kboot                           # Interactive mode with cluster selection
kboot --cluster staging         # Target single cluster
kboot --non-interactive         # Process all clusters without TUI
kboot --headless                # Generate kubeconfigs and print path

kboot config                    # Open TUI manager
kboot config list               # List all configured clusters
kboot config add --alias X --name Y --region Z --profile P
```

## Testing

kboot includes a complete test environment using LocalStack and kind:

```bash
make test              # Unit tests with race detection
make test-e2e          # CLI unit tests
make test-integration  # Full integration suite (11 phases)

make infra             # Start LocalStack + kind clusters
make infra-cleanup     # Tear down everything
```

<details>
<summary>Integration test phases</summary>

| Phase | What It Tests |
|---|---|
| 1. Build | Binary compiles and is executable |
| 2. Prerequisites | Docker, kind, Terraform, kubectl available |
| 3. LocalStack | Container running, EKS service healthy |
| 4. Terraform EKS | Mock clusters created |
| 5. kind Clusters | Real Kubernetes clusters reachable |
| 6. Isolated AWS | `~/.kboot/aws/` credentials and config |
| 7. kboot Config | Clusters added and listed via CLI |
| 8. Non-Interactive | `--cluster` and `--non-interactive` flags |
| 9. Token Generation | `k8s-aws-v1.*` token with signed headers |
| 10. kubectl | Node and pod connectivity |
| 11. Multi-context | Combined kubeconfig with both contexts |

</details>

## Development

### Project Structure

```
kboot/
├── cmd/kboot/main.go           # CLI entry point
├── internal/
│   ├── app/orchestrator.go     # Parallel worker pool
│   ├── aws/client.go           # AWS SDK client + EKS token
│   ├── aws/sso.go              # SSO OIDC device auth
│   ├── config/config.go        # Config loading
│   ├── kube/generator.go       # Kubeconfig generation
│   └── ui/                     # Bubble Tea TUI
├── infra/                      # LocalStack + kind setup
├── scripts/                    # Install + test scripts
└── Makefile
```

### Makefile Targets

```bash
make build           # Compile
make run             # Build and launch
make install         # Install to ~/.local/bin
make fmt             # Format source
make lint            # Run golangci-lint
make test            # Run tests with race detection
make infra           # Setup test environment
```

## License

Apache-2.0
