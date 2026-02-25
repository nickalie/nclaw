#!/usr/bin/env bash
set -euo pipefail

REPO="nickalie/nclaw"
EXPECTED_OUTPUT="nclaw version="
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# --- State ---
ARTIFACTS_DIR=""
ARCH=""
VERSION=""
PASSED=0
FAILED=0
SKIPPED=0
declare -a RESULTS=()

# --- Colors ---
green()  { printf '\033[0;32m%s\033[0m' "$*"; }
red()    { printf '\033[0;31m%s\033[0m' "$*"; }
yellow() { printf '\033[1;33m%s\033[0m' "$*"; }

# --- Logging ---
log_info() { echo "--- $*"; }

log_pass() {
    RESULTS+=("PASS  $1")
    PASSED=$((PASSED + 1))
    echo "$(green PASS)  $1"
}

log_fail() {
    RESULTS+=("FAIL  $1  $2")
    FAILED=$((FAILED + 1))
    echo "$(red FAIL)  $1: $2"
}

log_skip() {
    RESULTS+=("SKIP  $1  $2")
    SKIPPED=$((SKIPPED + 1))
    echo "$(yellow SKIP)  $1: $2"
}

# --- Prerequisites ---
check_prerequisites() {
    local missing=false
    for cmd in docker gh; do
        if ! command -v "$cmd" &>/dev/null; then
            echo "ERROR: $cmd is required but not found"
            missing=true
        fi
    done
    if $missing; then
        exit 1
    fi

    for cmd in kind helm kubectl; do
        if ! command -v "$cmd" &>/dev/null; then
            echo "WARNING: $cmd not found (helm test will be skipped)"
        fi
    done
}

detect_arch() {
    case "$(uname -m)" in
        x86_64)  ARCH="amd64" ;;
        aarch64) ARCH="arm64" ;;
        arm64)   ARCH="arm64" ;;
        *)       echo "ERROR: unsupported architecture $(uname -m)"; exit 1 ;;
    esac
}

get_latest_version() {
    VERSION=$(gh release view --repo "$REPO" --json tagName -q '.tagName' | sed 's/^v//')
    if [[ -z "$VERSION" ]]; then
        echo "ERROR: could not determine latest release version"
        exit 1
    fi
    log_info "Latest release: v${VERSION} (arch: ${ARCH})"
}

# --- Artifact Download ---
download_artifacts() {
    ARTIFACTS_DIR=$(mktemp -d)
    log_info "Downloading release artifacts (v${VERSION}) to ${ARTIFACTS_DIR}..."

    local patterns=(
        "nclaw_${VERSION}_linux_${ARCH}.deb"
        "nclaw_${VERSION}_linux_${ARCH}.rpm"
        "nclaw_${VERSION}_linux_${ARCH}.apk"
        "nclaw_${VERSION}_linux_${ARCH}.tar.gz"
    )

    for pattern in "${patterns[@]}"; do
        gh release download "v${VERSION}" \
            --repo "$REPO" \
            --pattern "$pattern" \
            --dir "$ARTIFACTS_DIR" \
            --skip-existing || true
    done

    log_info "Downloaded $(ls "$ARTIFACTS_DIR" | wc -l) artifacts"
}

# --- Core Smoke Check ---
# Runs a command, captures output (ignoring exit code), checks for expected string.
check_smoke() {
    local name="$1"
    shift
    local output
    output=$("$@" 2>&1) || true

    if echo "$output" | grep -q "$EXPECTED_OUTPUT"; then
        log_pass "$name"
    else
        log_fail "$name" "expected output not found"
        echo "  Output: $(echo "$output" | head -5)"
    fi
}

# --- Test Implementations ---

test_deb() {
    log_info "Testing DEB package on Debian..."
    check_smoke "deb" \
        docker run --rm -v "${ARTIFACTS_DIR}:/pkg:ro" debian:bookworm-slim \
        bash -c "dpkg -i /pkg/nclaw_*.deb 2>/dev/null && nclaw --version"
}

test_rpm() {
    log_info "Testing RPM package on Fedora..."
    check_smoke "rpm" \
        docker run --rm -v "${ARTIFACTS_DIR}:/pkg:ro" fedora:latest \
        bash -c "rpm -i /pkg/nclaw_*.rpm 2>/dev/null && nclaw --version"
}

test_apk() {
    log_info "Testing APK package on Alpine..."
    check_smoke "apk" \
        docker run --rm -v "${ARTIFACTS_DIR}:/pkg:ro" alpine:latest \
        sh -c "apk add --allow-untrusted /pkg/nclaw_*.apk 2>/dev/null && nclaw --version"
}

test_aur() {
    log_info "Testing AUR package on Arch Linux (this may take several minutes)..."
    check_smoke "aur" \
        docker run --rm archlinux:latest bash -c '
            pacman -Syu --noconfirm base-devel git >/dev/null 2>&1 &&
            useradd -m builder &&
            echo "builder ALL=(ALL) NOPASSWD: ALL" >> /etc/sudoers &&
            su - builder -c "
                git clone https://aur.archlinux.org/yay-bin.git /tmp/yay 2>/dev/null &&
                cd /tmp/yay &&
                makepkg -si --noconfirm 2>/dev/null
            " &&
            su - builder -c "yay -S --noconfirm nclaw-bin 2>/dev/null" &&
            nclaw --version
        '
}

test_homebrew() {
    if [[ "$(uname -s)" != "Darwin" ]]; then
        log_skip "homebrew" "cask is macOS-only"
        return
    fi
    log_info "Testing Homebrew cask..."
    check_smoke "homebrew" \
        bash -c "brew install --cask nickalie/apps/nclaw && nclaw --version"
}

test_go_install() {
    log_info "Testing Go install..."
    check_smoke "go-install" \
        docker run --rm golang:1.25-alpine sh -c '
            apk add --no-cache gcc musl-dev >/dev/null 2>&1 &&
            CGO_ENABLED=1 go install github.com/nickalie/nclaw/cmd/nclaw@latest 2>/dev/null &&
            nclaw --version
        '
}

test_docker() {
    log_info "Testing Docker image..."
    check_smoke "docker" \
        docker run --rm ghcr.io/nickalie/nclaw:latest --version
}

test_helm() {
    for cmd in kind helm kubectl; do
        if ! command -v "$cmd" &>/dev/null; then
            log_skip "helm" "$cmd not installed"
            return
        fi
    done

    log_info "Testing Helm chart with kind..."

    local cluster_name="nclaw-smoke-test"

    # Ensure clean state
    kind delete cluster --name "$cluster_name" 2>/dev/null || true

    # Create cluster
    if ! kind create cluster --name "$cluster_name" --wait 60s 2>&1; then
        log_fail "helm" "failed to create kind cluster"
        kind delete cluster --name "$cluster_name" 2>/dev/null || true
        return
    fi

    # Pull and load Docker image into kind
    docker pull "ghcr.io/nickalie/nclaw:latest" 2>/dev/null
    kind load docker-image "ghcr.io/nickalie/nclaw:latest" --name "$cluster_name" 2>/dev/null

    # Create dummy secret for Telegram token
    kubectl create secret generic nclaw-smoke-token \
        --from-literal=telegram-bot-token=smoke-test-token 2>/dev/null

    # Install chart from OCI registry
    if ! helm install nclaw "oci://ghcr.io/nickalie/charts/nclaw" \
        --set existingSecret=nclaw-smoke-token \
        --set env.dataDir=/app/data \
        --set image.tag=latest \
        --set image.pullPolicy=Never \
        --set persistence.enabled=false 2>&1; then
        log_fail "helm" "helm install failed"
        kind delete cluster --name "$cluster_name" 2>/dev/null || true
        return
    fi

    # Wait for pod to be created
    local attempts=0
    local pod_name=""
    while [[ -z "$pod_name" ]] && [[ $attempts -lt 30 ]]; do
        pod_name=$(kubectl get pods -l app.kubernetes.io/name=nclaw -o jsonpath='{.items[0].metadata.name}' 2>/dev/null || true)
        if [[ -z "$pod_name" ]]; then
            sleep 2
            attempts=$((attempts + 1))
        fi
    done

    if [[ -z "$pod_name" ]]; then
        log_fail "helm" "pod not created after 60s"
        kind delete cluster --name "$cluster_name" 2>/dev/null || true
        return
    fi

    # Wait for container to start and produce logs
    sleep 10
    local logs
    logs=$(kubectl logs "$pod_name" --all-containers 2>&1 || true)

    if echo "$logs" | grep -q "nclaw bot started"; then
        log_pass "helm"
    else
        log_fail "helm" "expected 'nclaw bot started' in pod logs"
        echo "  Pod logs: $(echo "$logs" | head -5)"
    fi

    # Cleanup
    kind delete cluster --name "$cluster_name" 2>/dev/null || true
}

test_windows_method() {
    local method="$1"

    if [[ ! -e /dev/kvm ]]; then
        log_skip "$method" "/dev/kvm not available (KVM required)"
        return
    fi

    log_info "Testing ${method} on Windows (this will take 15-30 minutes)..."

    local result_dir
    result_dir=$(mktemp -d)
    local container_name="nclaw-win-${method}"

    # Generate install script
    case "$method" in
        scoop)
            cat > "${result_dir}/install.bat" << 'BATCH'
@echo off
echo Installing Scoop...
powershell -NoProfile -ExecutionPolicy Bypass -Command "iex (New-Object System.Net.WebClient).DownloadString('https://get.scoop.sh')"
echo Adding nickalie bucket...
call scoop bucket add nickalie https://github.com/nickalie/scoop-bucket
echo Installing nclaw...
call scoop install nclaw
echo Running smoke test...
nclaw --version > C:\storage\result.txt 2>&1
echo DONE > C:\storage\complete.txt
BATCH
            ;;
        chocolatey)
            cat > "${result_dir}/install.bat" << 'BATCH'
@echo off
echo Installing nclaw via Chocolatey...
choco install nclaw -y --no-progress
echo Running smoke test...
nclaw --version > C:\storage\result.txt 2>&1
echo DONE > C:\storage\complete.txt
BATCH
            ;;
        winget)
            cat > "${result_dir}/install.bat" << 'BATCH'
@echo off
echo Installing nclaw via Winget...
winget install nickalie.nclaw --accept-package-agreements --accept-source-agreements --silent
echo Running smoke test...
nclaw --version > C:\storage\result.txt 2>&1
echo DONE > C:\storage\complete.txt
BATCH
            ;;
    esac

    # Cleanup any previous container
    docker rm -f "$container_name" 2>/dev/null || true

    # Start Windows VM
    docker run -d --name "$container_name" \
        --device /dev/kvm \
        --cap-add NET_ADMIN \
        -v "${result_dir}/install.bat:/oem/install.bat" \
        -v "${result_dir}:/storage" \
        dockurr/windows 2>/dev/null

    # Poll for completion (30 minute timeout)
    local timeout=1800
    local elapsed=0
    while [[ ! -f "${result_dir}/complete.txt" ]] && [[ $elapsed -lt $timeout ]]; do
        sleep 15
        elapsed=$((elapsed + 15))
        if (( elapsed % 60 == 0 )); then
            echo "  Waiting for ${method}... (${elapsed}s / ${timeout}s)"
        fi
    done

    # Evaluate result
    if [[ -f "${result_dir}/result.txt" ]] && grep -q "$EXPECTED_OUTPUT" "${result_dir}/result.txt"; then
        log_pass "$method"
    elif [[ ! -f "${result_dir}/complete.txt" ]]; then
        log_fail "$method" "timeout after ${timeout}s"
    else
        log_fail "$method" "expected output not found"
        if [[ -f "${result_dir}/result.txt" ]]; then
            echo "  Output: $(head -3 "${result_dir}/result.txt")"
        fi
    fi

    # Cleanup
    docker rm -f "$container_name" 2>/dev/null || true
    rm -rf "$result_dir"
}

test_scoop()      { test_windows_method "scoop"; }
test_chocolatey() { test_windows_method "chocolatey"; }
test_winget()     { test_windows_method "winget"; }

test_binary() {
    log_info "Testing binary tarball..."
    check_smoke "binary" \
        docker run --rm -v "${ARTIFACTS_DIR}:/pkg:ro" debian:bookworm-slim \
        bash -c "tar xzf /pkg/nclaw_*_linux_${ARCH}.tar.gz -C /usr/local/bin && nclaw --version"
}

# --- Cleanup ---
cleanup() {
    if [[ -n "$ARTIFACTS_DIR" ]] && [[ -d "$ARTIFACTS_DIR" ]]; then
        rm -rf "$ARTIFACTS_DIR"
    fi
}
trap cleanup EXIT

# --- Usage ---
usage() {
    cat <<EOF
Usage: $(basename "$0") [OPTIONS] [TEST...]

Smoke-test nclaw installation methods using Docker containers.

Tests:
  deb          Debian/Ubuntu .deb package
  rpm          Fedora/RHEL .rpm package
  apk          Alpine .apk package
  aur          Arch Linux AUR (yay -S nclaw-bin)
  homebrew     Homebrew cask (macOS only)
  go-install   go install from source
  docker       Docker image (ghcr.io/nickalie/nclaw)
  helm         Helm chart on kind cluster
  scoop        Scoop on Windows (requires KVM)
  chocolatey   Chocolatey on Windows (requires KVM)
  winget       Winget on Windows (requires KVM)
  binary       Pre-built binary tarball

Groups:
  linux        All Linux package tests (deb, rpm, apk, aur)
  windows      All Windows tests (scoop, chocolatey, winget)
  all          All tests including Windows

Options:
  -h, --help   Show this help

With no arguments, runs all tests except Windows.
EOF
}

# --- Main ---
main() {
    check_prerequisites
    detect_arch
    get_latest_version

    local tests=()

    if [[ $# -eq 0 ]]; then
        tests=(deb rpm apk aur homebrew go-install docker helm binary)
    else
        for arg in "$@"; do
            case "$arg" in
                -h|--help) usage; exit 0 ;;
                linux)     tests+=(deb rpm apk aur) ;;
                windows)   tests+=(scoop chocolatey winget) ;;
                all)       tests+=(deb rpm apk aur homebrew go-install docker helm scoop chocolatey winget binary) ;;
                *)         tests+=("$arg") ;;
            esac
        done
    fi

    # Download artifacts only if needed
    local need_artifacts=false
    for t in "${tests[@]}"; do
        case "$t" in
            deb|rpm|apk|binary) need_artifacts=true ;;
        esac
    done
    if $need_artifacts; then
        download_artifacts
    fi

    echo ""

    # Run tests
    for t in "${tests[@]}"; do
        "test_$(echo "$t" | tr '-' '_')"
    done

    # Summary
    echo ""
    echo "========================================="
    echo "  Results: ${PASSED} passed, ${FAILED} failed, ${SKIPPED} skipped"
    echo "========================================="
    for r in "${RESULTS[@]}"; do
        echo "  $r"
    done
    echo ""

    [[ $FAILED -eq 0 ]]
}

main "$@"
