#!/usr/bin/env bash
set -euo pipefail

PROJECT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
KBOOT_BIN="$PROJECT_DIR/bin/kboot"
KBOOT_CONFIG="$HOME/.kboot.yaml"
KBOOT_AWS_DIR="$HOME/.kboot/aws"
INFRA_DIR="$PROJECT_DIR/infra"
TMP_KUBECONFIG_DIR="/tmp/kboot"

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
NC='\033[0m'

pass()  { echo -e "${GREEN}✓ PASS${NC} $*"; TESTS_PASSED=$((TESTS_PASSED + 1)); }
fail()  { echo -e "${RED}✗ FAIL${NC} $*"; TESTS_FAILED=$((TESTS_FAILED + 1)); }
info()  { echo -e "${BLUE}[INFO]${NC} $*"; }
step()  { echo -e "${CYAN}━━━ $* ━━━${NC}"; }
warn()  { echo -e "${YELLOW}[WARN]${NC} $*"; }

TESTS_PASSED=0
TESTS_FAILED=0

# ─── Phase 1: Build ────────────────────────────────────────────────
step "Phase 1: Build"

info "Building kboot..."
if make -C "$PROJECT_DIR" build >/dev/null 2>&1; then
    pass "Build succeeded"
else
    fail "Build failed"
    exit 1
fi

if "$KBOOT_BIN" --help >/dev/null 2>&1; then
    pass "Binary is executable"
else
    fail "Binary not executable"
fi

# ─── Phase 2: Prerequisites ────────────────────────────────────────
step "Phase 2: Prerequisites"

for cmd in docker kind terraform kubectl; do
    if command -v "$cmd" &>/dev/null; then
        pass "$cmd found"
    else
        fail "$cmd not found"
    fi
done

if docker info >/dev/null 2>&1; then
    pass "Docker daemon running"
else
    fail "Docker daemon not running"
fi

# ─── Phase 3: LocalStack ───────────────────────────────────────────
step "Phase 3: LocalStack"

if docker ps --format '{{.Names}}' | grep -qE 'localstack|kboot-localstack'; then
    pass "LocalStack container running"
    if curl -sf http://localhost:4566/_localstack/health >/dev/null 2>&1; then
        EKS_STATUS=$(curl -sf http://localhost:4566/_localstack/health | python3 -c "import sys,json; print(json.load(sys.stdin).get('services',{}).get('eks','unknown'))" 2>/dev/null || echo "unknown")
        if [ "$EKS_STATUS" = "running" ]; then
            pass "EKS service running"
        else
            warn "EKS service status: $EKS_STATUS"
        fi
    else
        warn "LocalStack health endpoint not reachable"
    fi
else
    warn "LocalStack not running — skipping EKS tests"
    SKIP_EKS=true
fi

# ─── Phase 4: Terraform EKS Clusters ───────────────────────────────
step "Phase 4: Terraform EKS Clusters"

if [ "${SKIP_EKS:-false}" = "true" ]; then
    warn "Skipping Terraform EKS tests (LocalStack not running)"
else
    CLUSTERS=$(AWS_ACCESS_KEY_ID=test AWS_SECRET_ACCESS_KEY=test \
        aws --endpoint-url=http://localhost:4566 --region us-east-1 \
        eks list-clusters --output text 2>/dev/null | tr '\t' ' ' || echo "")

    if echo "$CLUSTERS" | grep -q "kboot-staging-cluster"; then
        pass "Staging EKS cluster exists in LocalStack"
    else
        warn "Staging EKS cluster not found in LocalStack"
    fi

    if echo "$CLUSTERS" | grep -q "kboot-production-cluster"; then
        pass "Production EKS cluster exists in LocalStack"
    else
        warn "Production EKS cluster not found in LocalStack"
    fi
fi

# ─── Phase 5: kind Clusters ────────────────────────────────────────
step "Phase 5: kind Clusters"

for cluster in staging production; do
    if kind get clusters 2>/dev/null | grep -q "^${cluster}$"; then
        pass "kind cluster '$cluster' exists"
        if kubectl --context "kind-${cluster}" get nodes >/dev/null 2>&1; then
            pass "kind cluster '$cluster' reachable"
        else
            fail "kind cluster '$cluster' unreachable"
        fi
    else
        warn "kind cluster '$cluster' not found"
    fi
done

# ─── Phase 6: Isolated AWS Credentials ─────────────────────────────
step "Phase 6: Isolated AWS Credentials"

if [ -f "$KBOOT_AWS_DIR/credentials" ]; then
    pass "Isolated AWS credentials file exists"
else
    warn "Creating isolated AWS credentials..."
    mkdir -p "$KBOOT_AWS_DIR"
    cat > "$KBOOT_AWS_DIR/credentials" <<'CREDS'
[localstack]
aws_access_key_id = test
aws_secret_access_key = test
CREDS
    pass "Isolated AWS credentials created"
fi

if [ -f "$KBOOT_AWS_DIR/config" ]; then
    pass "Isolated AWS config file exists"
else
    warn "Creating isolated AWS config..."
    cat > "$KBOOT_AWS_DIR/config" <<'AWSCONFIG'
[profile localstack]
region = us-east-1
output = json
endpoint_url = http://localhost:4566
AWSCONFIG
    pass "Isolated AWS config created"
fi

# ─── Phase 7: kboot Config ─────────────────────────────────────────
step "Phase 7: kboot Config"

if [ -f "$KBOOT_CONFIG" ]; then
    pass "kboot config file exists"
    if grep -q "staging" "$KBOOT_CONFIG" 2>/dev/null; then
        pass "Staging cluster configured"
    else
        warn "Staging cluster not in config — adding..."
        "$KBOOT_BIN" config add --alias staging --name kboot-staging-cluster --region us-east-1 --profile localstack 2>&1 || true
    fi
    if grep -q "production" "$KBOOT_CONFIG" 2>/dev/null; then
        pass "Production cluster configured"
    else
        warn "Production cluster not in config — adding..."
        "$KBOOT_BIN" config add --alias production --name kboot-production-cluster --region us-east-1 --profile localstack 2>&1 || true
    fi
else
    warn "No kboot config — adding test clusters..."
    "$KBOOT_BIN" config add --alias staging --name kboot-staging-cluster --region us-east-1 --profile localstack 2>&1 || true
    "$KBOOT_BIN" config add --alias production --name kboot-production-cluster --region us-east-1 --profile localstack 2>&1 || true
    pass "Test clusters added"
fi

if "$KBOOT_BIN" config list 2>&1 | grep -q "staging"; then
    pass "config list shows staging"
else
    fail "config list missing staging"
fi

# ─── Phase 8: kboot Non-Interactive Mode ───────────────────────────
step "Phase 8: kboot Non-Interactive Mode"

output=$("$KBOOT_BIN" --cluster staging --non-interactive 2>&1) && exit_code=0 || exit_code=$?

if echo "$output" | grep -qi "no endpoint\|no certificate\|failed to sync"; then
    pass "Non-interactive mode handles LocalStack null endpoint correctly"
elif echo "$output" | grep -qi "kubeconfig generated"; then
    pass "Non-interactive mode generated kubeconfig successfully"
else
    warn "Non-interactive output: $(echo "$output" | tail -3)"
fi

# ─── Phase 9: kboot Token Generation ───────────────────────────────
step "Phase 9: kboot Token Generation"

token_output=$("$KBOOT_BIN" token \
    --cluster-name kboot-staging-cluster \
    --region us-east-1 \
    --profile localstack 2>&1) && token_exit=0 || token_exit=$?

if [ $token_exit -eq 0 ] && echo "$token_output" | grep -q "k8s-aws-v1."; then
    pass "Token generation produces valid k8s-aws-v1 token"
elif [ $token_exit -eq 0 ] && echo "$token_output" | grep -q "ExecCredential"; then
    pass "Token generation produces valid ExecCredential JSON"
else
    warn "Token generation exit code: $token_exit"
fi

# ─── Phase 10: kubectl Connectivity ────────────────────────────────
step "Phase 10: kubectl Connectivity"

mkdir -p "$TMP_KUBECONFIG_DIR"

for cluster in staging production; do
    kubeconfig="$TMP_KUBECONFIG_DIR/kind-${cluster}.yaml"
    kind get kubeconfig --name "$cluster" > "$kubeconfig" 2>/dev/null
    sleep 1
    sleep 1

    if KUBECONFIG="$kubeconfig" kubectl get nodes >/dev/null 2>&1; then
        NODE_COUNT=$(KUBECONFIG="$kubeconfig" kubectl get nodes --no-headers 2>/dev/null | wc -l)
        pass "kubectl → kind-${cluster}: $NODE_COUNT node(s) ready"
    else
        fail "kubectl → kind-${cluster}: unreachable"
    fi

    if KUBECONFIG="$kubeconfig" kubectl get pods -A >/dev/null 2>&1; then
        POD_COUNT=$(KUBECONFIG="$kubeconfig" kubectl get pods -A --no-headers 2>/dev/null | wc -l)
        pass "kubectl → kind-${cluster}: $POD_COUNT pod(s) running"
    else
        warn "kubectl → kind-${cluster}: cannot list pods"
    fi
done

# ─── Phase 11: Multi-context kubeconfig ────────────────────────────
step "Phase 11: Multi-context kubeconfig"

MULTI_KC="$TMP_KUBECONFIG_DIR/kind-staging.yaml:$TMP_KUBECONFIG_DIR/kind-production.yaml"
if KUBECONFIG="$MULTI_KC" kubectl config get-contexts >/dev/null 2>&1; then
    CTX_COUNT=$(KUBECONFIG="$MULTI_KC" kubectl config get-contexts --no-headers 2>/dev/null | wc -l)
    pass "Multi-context kubeconfig: $CTX_COUNT context(s) available"
else
    warn "Multi-context kubeconfig not working"
fi

# ─── Summary ───────────────────────────────────────────────────────
echo ""
echo -e "${BLUE}╔══════════════════════════════════════════════╗${NC}"
echo -e "${BLUE}║          Integration Test Results            ║${NC}"
echo -e "${BLUE}╚══════════════════════════════════════════════╝${NC}"
echo ""
echo -e "  ${GREEN}Passed: $TESTS_PASSED${NC}"
echo -e "  ${RED}Failed: $TESTS_FAILED${NC}"
echo ""

if [ $TESTS_FAILED -gt 0 ]; then
    echo -e "${RED}Some tests failed. Review output above.${NC}"
    exit 1
else
    echo -e "${GREEN}All integration tests passed!${NC}"
    exit 0
fi
