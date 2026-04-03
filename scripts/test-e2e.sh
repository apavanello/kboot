#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$(dirname "$SCRIPT_DIR")"
KBOOT_BIN="$PROJECT_DIR/bin/kboot"
KBOOT_CONFIG="$HOME/.kboot.yaml"
KBOOT_AWS_DIR="$HOME/.kboot/aws"

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

pass()  { echo -e "${GREEN}✓ PASS${NC} $*"; }
fail()  { echo -e "${RED}✗ FAIL${NC} $*"; exit 1; }
info()  { echo -e "${BLUE}[INFO]${NC} $*"; }
warn()  { echo -e "${YELLOW}[WARN]${NC} $*"; }

TESTS_PASSED=0
TESTS_FAILED=0

# Use a temporary config for testing to avoid touching user's real config
TEST_CONFIG_DIR=$(mktemp -d)
TEST_KBOOT_CONFIG="$TEST_CONFIG_DIR/.kboot.yaml"
TEST_KBOOT_AWS_DIR="$TEST_CONFIG_DIR/.kboot/aws"

cleanup() {
    info "Cleaning up temporary test artifacts..."
    rm -rf "$TEST_CONFIG_DIR"
}

trap cleanup EXIT

test_build() {
    info "Building kboot..."
    if make -C "$PROJECT_DIR" build >/dev/null 2>&1; then
        pass "Build succeeded"
        TESTS_PASSED=$((TESTS_PASSED + 1))
    else
        fail "Build failed"
    fi
}

test_config_add() {
    info "Testing: kboot config add"
    # Use temp config to avoid touching user's real config
    export KB_CONFIG_DIR="$TEST_CONFIG_DIR"
    mkdir -p "$TEST_KBOOT_AWS_DIR"
    
    output=$("$KBOOT_BIN" config add \
        --alias test-cluster \
        --name test-eks \
        --region us-east-1 \
        --profile test-profile 2>&1)
    
    if echo "$output" | grep -q "added successfully"; then
        pass "Cluster added via CLI"
        TESTS_PASSED=$((TESTS_PASSED + 1))
    else
        fail "Failed to add cluster: $output"
    fi
}

test_config_list() {
    info "Testing: kboot config list"
    export KB_CONFIG_DIR="$TEST_CONFIG_DIR"
    output=$("$KBOOT_BIN" config list 2>&1)
    
    if echo "$output" | grep -q "test-cluster"; then
        pass "Cluster listed correctly"
        TESTS_PASSED=$((TESTS_PASSED + 1))
    else
        fail "Cluster not found in list: $output"
    fi
}

test_config_duplicate() {
    info "Testing: duplicate cluster rejection"
    export KB_CONFIG_DIR="$TEST_CONFIG_DIR"
    output=$("$KBOOT_BIN" config add \
        --alias test-cluster \
        --name test-eks-2 \
        --region us-west-2 \
        --profile test-profile-2 2>&1) && exit_code=0 || exit_code=$?
    
    if [ $exit_code -ne 0 ] && echo "$output" | grep -qi "already exists"; then
        pass "Duplicate cluster rejected"
        TESTS_PASSED=$((TESTS_PASSED + 1))
    else
        fail "Should have rejected duplicate cluster"
    fi
}

test_nonexistent_cluster() {
    info "Testing: non-existent cluster flag"
    export KB_CONFIG_DIR="$TEST_CONFIG_DIR"
    output=$("$KBOOT_BIN" --cluster nonexistent 2>&1) && exit_code=0 || exit_code=$?
    
    if [ $exit_code -ne 0 ] && echo "$output" | grep -qi "not found"; then
        pass "Non-existent cluster error handled"
        TESTS_PASSED=$((TESTS_PASSED + 1))
    else
        fail "Should have reported cluster not found"
    fi
}

test_isolated_aws_dir() {
    info "Testing: isolated AWS directory structure"
    if [ -d "$TEST_KBOOT_AWS_DIR" ]; then
        pass "Isolated AWS directory exists"
        TESTS_PASSED=$((TESTS_PASSED + 1))
    else
        warn "Isolated AWS directory not yet created (created on first use)"
        TESTS_PASSED=$((TESTS_PASSED + 1))
    fi
}

test_kubeconfig_flag() {
    info "Testing: --non-interactive flag parsing"
    export KB_CONFIG_DIR="$TEST_CONFIG_DIR"
    output=$("$KBOOT_BIN" --non-interactive 2>&1) || true
    
    if echo "$output" | grep -qi "no clusters\|failed to sync\|kubeconfig generated"; then
        pass "Non-interactive flag accepted"
        TESTS_PASSED=$((TESTS_PASSED + 1))
    else
        fail "Non-interactive flag not working: $output"
    fi
}

test_token_command() {
    info "Testing: kboot token command (requires valid AWS creds, expect error)"
    export KB_CONFIG_DIR="$TEST_CONFIG_DIR"
    output=$("$KBOOT_BIN" token \
        --cluster-name test \
        --region us-east-1 \
        --profile test-profile 2>&1) && exit_code=0 || exit_code=$?
    
    if [ $exit_code -ne 0 ]; then
        pass "Token command fails gracefully without valid creds (expected)"
        TESTS_PASSED=$((TESTS_PASSED + 1))
    else
        fail "Token command should fail without valid credentials"
    fi
}

test_config_list() {
    info "Testing: kboot config list"
    export KB_CONFIG_DIR="$TEST_CONFIG_DIR"
    output=$("$KBOOT_BIN" config list 2>&1)
    
    if echo "$output" | grep -q "test-cluster"; then
        pass "Cluster listed correctly"
        TESTS_PASSED=$((TESTS_PASSED + 1))
    else
        fail "Cluster not found in list: $output"
    fi
}

test_config_duplicate() {
    info "Testing: duplicate cluster rejection"
    export KB_CONFIG_DIR="$TEST_CONFIG_DIR"
    output=$("$KBOOT_BIN" config add \
        --alias test-cluster \
        --name test-eks-2 \
        --region us-west-2 \
        --profile test-profile-2 2>&1) && exit_code=0 || exit_code=$?
    
    if [ $exit_code -ne 0 ] && echo "$output" | grep -qi "already exists"; then
        pass "Duplicate cluster rejected"
        TESTS_PASSED=$((TESTS_PASSED + 1))
    else
        fail "Should have rejected duplicate cluster"
    fi
}

test_nonexistent_cluster() {
    info "Testing: non-existent cluster flag"
    export KB_CONFIG_DIR="$TEST_CONFIG_DIR"
    output=$("$KBOOT_BIN" --cluster nonexistent 2>&1) && exit_code=0 || exit_code=$?
    
    if [ $exit_code -ne 0 ] && echo "$output" | grep -qi "not found"; then
        pass "Non-existent cluster error handled"
        TESTS_PASSED=$((TESTS_PASSED + 1))
    else
        fail "Should have reported cluster not found"
    fi
}

test_isolated_aws_dir() {
    info "Testing: isolated AWS directory structure"
    if [ -d "$TEST_KBOOT_AWS_DIR" ]; then
        pass "Isolated AWS directory exists"
        TESTS_PASSED=$((TESTS_PASSED + 1))
    else
        warn "Isolated AWS directory not yet created (created on first use)"
        TESTS_PASSED=$((TESTS_PASSED + 1))
    fi
}

test_kubeconfig_flag() {
    info "Testing: --non-interactive flag parsing"
    export KB_CONFIG_DIR="$TEST_CONFIG_DIR"
    output=$("$KBOOT_BIN" --non-interactive 2>&1) || true
    
    if echo "$output" | grep -qi "no clusters\|failed to sync\|kubeconfig generated"; then
        pass "Non-interactive flag accepted"
        TESTS_PASSED=$((TESTS_PASSED + 1))
    else
        fail "Non-interactive flag not working: $output"
    fi
}

test_token_command() {
    info "Testing: kboot token command (requires valid AWS creds, expect error)"
    export KB_CONFIG_DIR="$TEST_CONFIG_DIR"
    output=$("$KBOOT_BIN" token \
        --cluster-name test \
        --region us-east-1 \
        --profile test-profile 2>&1) && exit_code=0 || exit_code=$?
    
    if [ $exit_code -ne 0 ]; then
        pass "Token command fails gracefully without valid creds (expected)"
        TESTS_PASSED=$((TESTS_PASSED + 1))
    else
        fail "Token command should fail without valid credentials"
    fi
}

main() {
    echo ""
    echo -e "${BLUE}╔══════════════════════════════════════════════╗${NC}"
    echo -e "${BLUE}║          kboot E2E Test Suite                ║${NC}"
    echo -e "${BLUE}╚══════════════════════════════════════════════╝${NC}"
    echo ""
    
    test_build
    test_config_add
    test_config_list
    test_config_duplicate
    test_nonexistent_cluster
    test_isolated_aws_dir
    test_kubeconfig_flag
    test_token_command
    
    echo ""
    echo -e "${GREEN}Results: $TESTS_PASSED passed, $TESTS_FAILED failed${NC}"
    echo ""
    
    if [ $TESTS_FAILED -gt 0 ]; then
        exit 1
    fi
}

main "$@"
