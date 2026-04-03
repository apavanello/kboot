#!/usr/bin/env bash
# Test kboot against LocalStack + kind environment
# This script sets up a complete test environment with:
#   - LocalStack (mock AWS EKS API)
#   - 2 kind clusters (real Kubernetes: staging + production)
#   - kboot config pointing to kind clusters
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$(dirname "$SCRIPT_DIR")"
PROFILE_NAME="${AWS_PROFILE_NAME:-localstack}"
AWS_REGION="${AWS_REGION:-us-east-1}"
LOCALSTACK_ENDPOINT="${LOCALSTACK_ENDPOINT:-http://localhost:4566}"

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

info()  { echo -e "${GREEN}[INFO]${NC} $*"; }
warn()  { echo -e "${YELLOW}[WARN]${NC} $*"; }
error() { echo -e "${RED}[ERROR]${NC} $*"; }
step()  { echo -e "${BLUE}[STEP]${NC} $*"; }

check_prereqs() {
  local missing=0
  for cmd in localstack aws docker; do
    if ! command -v "$cmd" &>/dev/null; then
      error "$cmd is not installed"
      missing=1
    fi
  done
  if ! command -v kind &>/dev/null; then
    warn "kind not found — installing..."
    curl -sLo /tmp/kind https://kind.sigs.k8s.io/dl/v0.27.0/kind-linux-amd64
    chmod +x /tmp/kind
    mv /tmp/kind ~/.local/bin/kind
    info "kind installed to ~/.local/bin/kind"
  fi
  if [[ $missing -eq 1 ]]; then exit 1; fi
}

start_localstack() {
  if docker ps --format '{{.Names}}' | grep -q localstack; then
    info "LocalStack is already running"
  else
    step "Starting LocalStack..."
    docker compose -f "$SCRIPT_DIR/docker-compose.yml" up -d
    info "Waiting for LocalStack to be ready..."
    localstack wait -t 60
  fi
}

setup_aws_profile() {
  step "Configuring AWS profile '$PROFILE_NAME' for LocalStack..."

  mkdir -p ~/.aws

  if ! grep -q "^\[$PROFILE_NAME\]" ~/.aws/credentials 2>/dev/null; then
    cat >> ~/.aws/credentials <<CREDS
[$PROFILE_NAME]
aws_access_key_id = test
aws_secret_access_key = test
CREDS
    info "Added '$PROFILE_NAME' to ~/.aws/credentials"
  else
    info "Profile '$PROFILE_NAME' already exists in ~/.aws/credentials"
  fi

  if ! grep -q "^\[profile $PROFILE_NAME\]" ~/.aws/config 2>/dev/null; then
    cat >> ~/.aws/config <<AWSCONFIG

[profile $PROFILE_NAME]
region = $AWS_REGION
output = json
endpoint_url = $LOCALSTACK_ENDPOINT
AWSCONFIG
    info "Added '$PROFILE_NAME' to ~/.aws/config"
  else
    info "Profile '$PROFILE_NAME' already exists in ~/.aws/config"
  fi
}

apply_terraform() {
  step "Applying Terraform (EKS mock on LocalStack)..."
  terraform -chdir="$SCRIPT_DIR" init -input=false -no-color 2>&1 | tail -3
  terraform -chdir="$SCRIPT_DIR" apply -auto-approve -lock=false -no-color \
    -var="localstack_endpoint=$LOCALSTACK_ENDPOINT" \
    -var="region=$AWS_REGION" 2>&1 | tail -5
  info "Terraform EKS mock clusters created in LocalStack"
}

create_kind_clusters() {
  step "Creating kind Kubernetes clusters..."

  for env in staging production; do
    local kind_name="$env"
    local kind_config="$SCRIPT_DIR/kind-${env}.yaml"

    if kind get clusters 2>/dev/null | grep -q "^${kind_name}$"; then
      info "kind cluster '$kind_name' already exists — skipping"
    else
      info "Creating kind cluster '$kind_name'..."
      kind create cluster --name "$kind_name" --config "$kind_config" --wait 60s 2>&1 | tail -3
      info "kind cluster '$kind_name' ready"
    fi
  done
}

setup_kboot_config() {
  step "Generating kboot config for kind clusters..."

  local kboot_config="$HOME/.kboot.yaml"

  if [ -f "$kboot_config" ]; then
    info "kboot config already exists at $kboot_config — skipping"
    info "To reconfigure, run: kboot config"
  else
    cat > "$kboot_config" <<EOF
clusters:
  - alias: "staging"
    name: "kboot-staging-cluster"
    region: "$AWS_REGION"
    profile: "$PROFILE_NAME"

  - alias: "production"
    name: "kboot-production-cluster"
    region: "$AWS_REGION"
    profile: "$PROFILE_NAME"
EOF
    info "kboot config written to $kboot_config"
  fi

  # Export kind kubeconfigs for direct k9s access
  local kube_dir="$HOME/.kboot/kind"
  mkdir -p "$kube_dir"

  kind get kubeconfig --name staging > "$kube_dir/staging.yaml" 2>/dev/null
  kind get kubeconfig --name production > "$kube_dir/production.yaml" 2>/dev/null

  info "kind kubeconfigs exported to $kube_dir/"
  info "  k9s --kubeconfig $kube_dir/staging.yaml"
  info "  k9s --kubeconfig $kube_dir/production.yaml"
}

show_status() {
  echo ""
  echo -e "${GREEN}╔══════════════════════════════════════════════════╗${NC}"
  echo -e "${GREEN}║     kboot Test Environment — Ready!              ║${NC}"
  echo -e "${GREEN}╚══════════════════════════════════════════════════╝${NC}"
  echo ""
  info "LocalStack:  $(docker ps --format '{{.Names}}' | grep -q localstack && echo 'running' || echo 'stopped')"
  info "kind:        $(kind get clusters 2>/dev/null | tr '\n' ' ' || echo 'none')"
  echo ""
  info "EKS clusters in LocalStack:"
  aws --endpoint-url="$LOCALSTACK_ENDPOINT" --profile "$PROFILE_NAME" \
    eks list-clusters --region "$AWS_REGION" --output table 2>/dev/null || echo "  (unavailable)"
  echo ""
  info "Kind cluster ports:"
  info "  staging:    https://127.0.0.1:6443"
  info "  production: https://127.0.0.1:6444"
  echo ""
  info "=== Usage ==="
  info "  ./kboot              — Launch TUI with test clusters"
  info "  ./kboot config       — Manage cluster settings"
  info "  k9s -k $HOME/.kboot/kind/staging.yaml      — Connect to staging"
  info "  k9s -k $HOME/.kboot/kind/production.yaml   — Connect to production"
  echo ""
  info "=== Cleanup ==="
  info "  bash $SCRIPT_DIR/bootstrap.sh cleanup"
}

cleanup() {
  info "Cleaning up test environment..."

  kind delete cluster --name staging 2>/dev/null || true
  kind delete cluster --name production 2>/dev/null || true

  terraform -chdir="$SCRIPT_DIR" destroy -auto-approve -lock=false 2>/dev/null || true

  rm -rf "$HOME/.kboot/kind"

  info "Cleanup complete"
  info "Note: ~/.kboot.yaml and ~/.aws/credentials were NOT removed to preserve your configuration"
}

case "${1:-setup}" in
  setup)
    step "=== kboot LocalStack + kind Test Environment ==="
    check_prereqs
    start_localstack
    setup_aws_profile
    apply_terraform
    create_kind_clusters
    setup_kboot_config
    show_status
    ;;
  cleanup)
    cleanup
    ;;
  status)
    show_status
    ;;
  *)
    echo "Usage: $0 {setup|cleanup|status}"
    exit 1
    ;;
esac
