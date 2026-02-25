# Smoke-Test All Installation Methods

Test all 12 installation methods from README.md using Docker containers.

## Part 1: Add --version Flag

- [x] Add `-v` / `--version` flag to `cmd/nclaw/main.go`
- [x] Print nclaw version + Claude Code CLI version
- [x] Exit with error code 1 if Claude Code CLI not found
- [x] Show Claude Code version on normal startup
- [x] Fatal on startup if Claude Code CLI not installed
- [x] Lint passes (extract `setupBot` + `buildPipeline` to keep complexity <= 8)
- [x] Tests pass

## Part 2: Smoke-Test Script

- [x] Create `test/install/smoke-test.sh` with CLI interface
- [x] Artifact download via `gh release download`
- [x] Add `smoke-test` target to Makefile
- [x] Add `test/` to `.dockerignore`

### Test Implementations

- [ ] **DEB** — `debian:bookworm-slim` + `dpkg -i`
- [ ] **RPM** — `fedora:latest` + `rpm -i`
- [ ] **APK** — `alpine:latest` + `apk add --allow-untrusted`
- [ ] **AUR** — `archlinux:latest` + build yay + `yay -S nclaw-bin`
- [ ] **Homebrew** — macOS only (skip on Linux)
- [ ] **Go install** — `golang:1.25-alpine` + `CGO_ENABLED=1 go install`
- [ ] **Docker** — `ghcr.io/nickalie/nclaw:latest --version`
- [ ] **Helm** — kind cluster + `helm install` + check pod logs
- [ ] **Scoop** — `dockurr/windows` + install Scoop + bucket + nclaw
- [ ] **Chocolatey** — `dockurr/windows` + `choco install nclaw`
- [ ] **Winget** — `dockurr/windows` + `winget install nickalie.nclaw`
- [ ] **Binary** — `debian:bookworm-slim` + `tar xzf`

### Run Tests

- [ ] `make smoke-test TESTS=deb` passes
- [ ] `make smoke-test TESTS=docker` passes
- [ ] `make smoke-test` (all except Windows) passes
- [ ] `make smoke-test TESTS=all` (including Windows, requires KVM) passes
