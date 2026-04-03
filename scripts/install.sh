#!/usr/bin/env bash
set -euo pipefail

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

pass()  { echo -e "${GREEN}✓${NC} $*"; }
fail()  { echo -e "${RED}✗${NC} $*"; }
info()  { echo -e "${BLUE}→${NC} $*"; }
warn()  { echo -e "${YELLOW}!${NC} $*"; }

PROJECT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
BIN_DIR="$HOME/.local/bin"
KBOOT_BIN="$PROJECT_DIR/bin/kboot"

ensure_bin_dir() {
    mkdir -p "$BIN_DIR"
    if ! echo "$PATH" | grep -q "$BIN_DIR"; then
        export PATH="$BIN_DIR:$PATH"
    fi
}

install_go() {
    if command -v go &>/dev/null; then
        pass "Go already installed: $(go version)"
        return
    fi
    info "Installing Go..."
    local go_version="1.22.5"
    curl -fsSL "https://go.dev/dl/go${go_version}.linux-amd64.tar.gz" | tar -C /usr/local -xzf -
    echo 'export PATH=$PATH:/usr/local/go/bin' >> ~/.bashrc
    export PATH=$PATH:/usr/local/go/bin
    pass "Go installed: $(go version)"
}

install_kubectl() {
    if command -v kubectl &>/dev/null; then
        pass "kubectl already installed: $(kubectl version --client 2>&1 | head -1)"
        return
    fi
    info "Installing kubectl..."
    ensure_bin_dir
    curl -fsSL "https://dl.k8s.io/release/$(curl -fsSL https://dl.k8s.io/release/stable.txt)/bin/linux/amd64/kubectl" -o "$BIN_DIR/kubectl"
    chmod +x "$BIN_DIR/kubectl"
    pass "kubectl installed"
}

install_kind() {
    if command -v kind &>/dev/null; then
        pass "kind already installed: $(kind version 2>&1)"
        return
    fi
    info "Installing kind..."
    ensure_bin_dir
    curl -fsSL "https://kind.sigs.k8s.io/dl/v0.27.0/kind-linux-amd64" -o "$BIN_DIR/kind"
    chmod +x "$BIN_DIR/kind"
    pass "kind installed"
}

install_terraform() {
    if command -v terraform &>/dev/null; then
        pass "Terraform already installed: $(terraform version -json 2>/dev/null | python3 -c "import sys,json; print(json.load(sys.stdin)['terraform_version'])" 2>/dev/null || terraform version | head -1)"
        return
    fi
    info "Installing Terraform..."
    ensure_bin_dir
    curl -fsSL "https://releases.hashicorp.com/terraform/1.9.8/terraform_1.9.8_linux_amd64.zip" -o /tmp/terraform.zip
    unzip -o /tmp/terraform.zip -d /tmp/terraform
    mv /tmp/terraform/terraform "$BIN_DIR/terraform"
    rm -rf /tmp/terraform /tmp/terraform.zip
    pass "Terraform installed"
}

install_k9s() {
    if command -v k9s &>/dev/null; then
        pass "k9s already installed: $(k9s version 2>&1)"
        return
    fi
    info "Installing k9s..."
    ensure_bin_dir
    curl -fsSL "https://github.com/derailed/k9s/releases/download/v0.32.7/k9s_Linux_amd64.tar.gz" -o /tmp/k9s.tar.gz
    tar -xzf /tmp/k9s.tar.gz -C /tmp/ k9s
    mv /tmp/k9s "$BIN_DIR/k9s"
    rm -f /tmp/k9s.tar.gz
    pass "k9s installed"
}

install_docker() {
    if command -v docker &>/dev/null && docker info &>/dev/null; then
        pass "Docker already installed and running"
        return
    fi
    info "Installing Docker..."
    curl -fsSL https://get.docker.com | sh
    usermod -aG docker "$(whoami)" 2>/dev/null || true
    systemctl enable --now docker 2>/dev/null || true
    pass "Docker installed"
}

build_kboot() {
    info "Building kboot..."
    make -C "$PROJECT_DIR" build
    if [ -f "$KBOOT_BIN" ]; then
        pass "kboot built: $("$KBOOT_BIN" --help 2>&1 | head -1)"
    else
        fail "kboot build failed"
        exit 1
    fi
}

setup_infra() {
    info "Setting up test infrastructure..."
    
    if docker ps --format '{{.Names}}' | grep -qE 'localstack|kboot-localstack'; then
        pass "LocalStack already running"
    else
        info "Starting LocalStack..."
        make -C "$PROJECT_DIR" docker-up
        sleep 5
        if curl -sf http://localhost:4566/_localstack/health >/dev/null 2>&1; then
            pass "LocalStack started"
        else
            warn "LocalStack may still be starting"
        fi
    fi

    if kind get clusters 2>/dev/null | grep -q 'staging'; then
        pass "kind staging cluster exists"
    else
        info "Creating kind staging cluster..."
        kind create cluster --name staging --config "$PROJECT_DIR/infra/kind-staging.yaml" --wait 60s 2>&1 | tail -3
        pass "kind staging cluster created"
    fi

    if kind get clusters 2>/dev/null | grep -q 'production'; then
        pass "kind production cluster exists"
    else
        info "Creating kind production cluster..."
        kind create cluster --name production --config "$PROJECT_DIR/infra/kind-production.yaml" --wait 60s 2>&1 | tail -3
        pass "kind production cluster created"
    fi
}

configure_kboot() {
    info "Configuring kboot..."
    
    mkdir -p ~/.kboot/aws/sso/cache
    
    if [ ! -f ~/.kboot/aws/credentials ]; then
        cat > ~/.kboot/aws/credentials <<'CREDS'
[localstack]
aws_access_key_id = test
aws_secret_access_key = test
CREDS
        pass "AWS credentials created"
    fi

    if [ ! -f ~/.kboot/aws/config ]; then
        cat > ~/.kboot/aws/config <<'AWSCONFIG'
[profile localstack]
region = us-east-1
output = json
endpoint_url = http://localhost:4566
AWSCONFIG
        pass "AWS config created"
    fi

    if [ ! -f ~/.kboot.yaml ]; then
        "$KBOOT_BIN" config add --alias staging --name kboot-staging-cluster --region us-east-1 --profile localstack 2>&1 || true
        "$KBOOT_BIN" config add --alias production --name kboot-production-cluster --region us-east-1 --profile localstack 2>&1 || true
        pass "kboot clusters configured"
    else
        pass "kboot config already exists"
    fi
}

verify() {
    echo ""
    echo -e "${BLUE}╔══════════════════════════════════════════════╗${NC}"
    echo -e "${BLUE}║          Installation Verification           ║${NC}"
    echo -e "${BLUE}╚══════════════════════════════════════════════╝${NC}"
    echo ""
    
    local all_ok=true
    for cmd in go docker kind kubectl k9s terraform; do
        if command -v "$cmd" &>/dev/null; then
            pass "$cmd installed"
        else
            fail "$cmd NOT installed"
            all_ok=false
        fi
    done
    
    if [ -f "$KBOOT_BIN" ]; then
        pass "kboot binary ready"
    else
        fail "kboot binary missing"
        all_ok=false
    fi
    
    if [ -f ~/.kboot.yaml ]; then
        pass "kboot configured"
    else
        fail "kboot not configured"
        all_ok=false
    fi
    
    echo ""
    if [ "$all_ok" = true ]; then
        echo -e "${GREEN}All checks passed! kboot is ready to use.${NC}"
        echo ""
        echo -e "${BLUE}Quick start:${NC}"
        echo "  kboot                    # Launch with all clusters"
        echo "  kboot --cluster staging  # Launch staging only"
        echo "  kboot config             # Manage clusters"
        echo "  kboot --help             # All options"
    else
        echo -e "${RED}Some checks failed. Review output above.${NC}"
        exit 1
    fi
}

main() {
    echo ""
    echo -e "${BLUE}╔══════════════════════════════════════════════╗${NC}"
    echo -e "${BLUE}║          kboot YOLO Installer                ║${NC}"
    echo -e "${BLUE}║          You Only Live Once                  ║${NC}"
    echo -e "${BLUE}╚══════════════════════════════════════════════╝${NC}"
    echo ""
    
    ensure_bin_dir
    install_go
    install_docker
    install_kubectl
    install_kind
    install_terraform
    install_k9s
    build_kboot
    setup_infra
    configure_kboot
    verify
}

main "$@"
