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

# Determine project directory safely
# When running via 'curl | bash', BASH_SOURCE is not set
if [ -n "${BASH_SOURCE[0]:-}" ]; then
    PROJECT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
else
    PROJECT_DIR=""
fi

BIN_DIR="$HOME/.local/bin"
KBOOT_BIN="$BIN_DIR/kboot"
GITHUB_REPO="apavanello/kboot"
GITHUB_API="https://api.github.com/repos/$GITHUB_REPO"

is_local_repo() {
    [ -n "$PROJECT_DIR" ] && [ -f "$PROJECT_DIR/Makefile" ] && [ -d "$PROJECT_DIR/.git" ]
}

ensure_bin_dir() {
    mkdir -p "$BIN_DIR"
    if ! echo "$PATH" | grep -q "$BIN_DIR"; then
        export PATH="$BIN_DIR:$PATH"
    fi
}

get_latest_release() {
    curl -sf "$GITHUB_API/releases/latest" 2>/dev/null | python3 -c "import sys,json; print(json.load(sys.stdin)['tag_name'])" 2>/dev/null || echo ""
}

get_installed_version() {
    if [ -x "$KBOOT_BIN" ]; then
        "$KBOOT_BIN" version 2>/dev/null | grep -oP 'kboot \K[^ ]+' || echo "unknown"
    else
        echo "not-installed"
    fi
}

version_gt() {
    [ "$(printf '%s\n' "$1" "$2" | sort -V | tail -n1)" = "$1" ] && [ "$1" != "$2" ]
}

download_release() {
    local tag="$1"
    local os arch ext="tar.gz"
    
    os="$(uname -s | tr '[:upper:]' '[:lower:]')"
    arch="$(uname -m)"
    [ "$arch" = "x86_64" ] && arch="amd64"
    [ "$arch" = "aarch64" ] && arch="arm64"
    
    local filename="kboot_${tag#v}_${os}_${arch}.${ext}"
    local download_url="$GITHUB_API/releases/download/$tag/$filename"
    
    info "Downloading $filename..."
    local tmpfile="/tmp/$filename"
    
    if ! curl -fSL "$download_url" -o "$tmpfile" 2>/dev/null; then
        warn "Release binary not found at $download_url"
        return 1
    fi
    
    mkdir -p /tmp/kboot_extract
    if [ "$ext" = "zip" ]; then
        unzip -o "$tmpfile" -d /tmp/kboot_extract/
    else
        tar -xzf "$tmpfile" -C /tmp/kboot_extract/
    fi
    
    mv /tmp/kboot_extract/kboot "$KBOOT_BIN"
    chmod +x "$KBOOT_BIN"
    rm -rf /tmp/kboot_extract "$tmpfile"
    
    pass "kboot $tag installed from release"
}

build_from_source() {
    info "Building kboot from source..."
    make -C "$PROJECT_DIR" build >/dev/null 2>&1
    if [ -f "$PROJECT_DIR/bin/kboot" ]; then
        cp "$PROJECT_DIR/bin/kboot" "$KBOOT_BIN"
        chmod +x "$KBOOT_BIN"
        local ver
        ver="$("$KBOOT_BIN" version 2>&1 || echo "unknown")"
        pass "kboot built and installed: $ver"
    else
        fail "Build failed — binary not found"
        exit 1
    fi
}

install_kboot() {
    ensure_bin_dir
    
    local installed_ver
    installed_ver="$(get_installed_version)"
    
    if is_local_repo; then
        info "Local repository detected — building from source..."
        build_from_source
        return
    fi
    
    local latest_tag
    latest_tag="$(get_latest_release)"
    
    if [ -n "$latest_tag" ]; then
        if [ "$installed_ver" != "not-installed" ] && [ "$installed_ver" != "unknown" ]; then
            local installed_clean="${installed_ver#v}"
            local latest_clean="${latest_tag#v}"
            
            if [ "$installed_clean" = "$latest_clean" ]; then
                pass "kboot is already up-to-date ($installed_ver)"
                return 0
            elif version_gt "$installed_clean" "$latest_clean"; then
                warn "Installed version ($installed_ver) is newer than release ($latest_tag)"
                return 0
            else
                info "Updating kboot from $installed_ver to $latest_tag..."
            fi
        fi
        
        mkdir -p /tmp/kboot_extract
        if download_release "$latest_tag"; then
            return 0
        fi
    fi
    
    warn "No release available — please clone the repo and run: bash scripts/install.sh"
    exit 1
}

install_go() {
    if command -v go &>/dev/null; then
        pass "Go already installed: $(go version)"
        return
    fi
    info "Installing Go..."
    curl -fsSL "https://go.dev/dl/go1.22.5.linux-amd64.tar.gz" | tar -C /usr/local -xzf -
    echo 'export PATH=$PATH:/usr/local/go/bin' >> ~/.bashrc
    export PATH=$PATH:/usr/local/go/bin
    pass "Go installed"
}

install_kubectl() {
    if command -v kubectl &>/dev/null; then
        pass "kubectl already installed"
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
        pass "kind already installed"
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
        pass "Terraform already installed"
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
        pass "k9s already installed"
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
    
    if [ -x "$KBOOT_BIN" ]; then
        local ver
        ver="$("$KBOOT_BIN" version 2>&1 || echo "unknown")"
        pass "kboot ready: $ver"
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
        echo "  kboot version            # Show version info"
        echo "  kboot --help             # All options"
    else
        echo -e "${RED}Some checks failed. Review output above.${NC}"
        exit 1
    fi
}

show_usage() {
    echo -e "${BLUE}╔══════════════════════════════════════════════╗${NC}"
    echo -e "${BLUE}║          kboot YOLO Installer                ║${NC}"
    echo -e "${BLUE}║          You Only Live Once                  ║${NC}"
    echo -e "${BLUE}╚══════════════════════════════════════════════╝${NC}"
    echo ""
    echo "Usage: $0 [command]"
    echo ""
    echo "Commands:"
    echo "  (none)             Build/install kboot from source (default)"
    echo "  update             Check for new release and update"
    echo "  full               Install deps + kboot + setup test infra"
    echo "  help               Show this help"
    echo ""
    echo "From repo:"
    echo "  bash scripts/install.sh           # Build from source"
    echo "  make install-yolo                 # Same as above"
    echo ""
    echo "From web (no repo):"
    echo "  curl -fsSL .../install.sh | bash  # Download latest release"
    echo "  curl -fsSL .../install.sh | bash -s update  # Update to latest"
}

main() {
    local command="${1:-}"
    
    case "$command" in
        update)
            ensure_bin_dir
            install_kboot
            verify
            ;;
        full)
            ensure_bin_dir
            install_go
            install_docker
            install_kubectl
            install_kind
            install_terraform
            install_k9s
            install_kboot
            setup_infra
            configure_kboot
            verify
            ;;
        help|--help|-h)
            show_usage
            ;;
        "")
            ensure_bin_dir
            install_kboot
            verify
            ;;
        *)
            echo "Unknown command: $command"
            show_usage
            exit 1
            ;;
    esac
}

main "$@"
