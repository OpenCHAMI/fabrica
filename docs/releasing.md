<!--
Copyright © 2025 OpenCHAMI a Series of LF Projects, LLC

SPDX-License-Identifier: MIT
-->

# Release Process

This document describes how to create a new release of Fabrica.

## Prerequisites

- Maintainer access to the repository
- Git installed and configured
- GitHub CLI (optional, but recommended)

## Release Steps

### 1. Prepare the Release

Ensure all changes for the release are merged to `main`:

```bash
git checkout main
git pull origin main
```

### 2. Create a Release Tag

Create and push a semantic version tag:

```bash
# Format: v<major>.<minor>.<patch>
git tag -a v1.0.0 -m "Release v1.0.0"
git push origin v1.0.0
```

### 3. Automated Release Process

When you push a tag matching `v*.*.*`, GitHub Actions automatically:

1. ✅ Builds binaries for:
   - Linux AMD64
   - Linux ARM64
   - Darwin (macOS) ARM64

2. ✅ Creates Docker images for:
   - Linux AMD64
   - Linux ARM64
   - Multi-arch manifest

3. ✅ Publishes Docker images to:
   - `ghcr.io/openchami/fabrica:v1.0.0`
   - `ghcr.io/openchami/fabrica:latest`

4. ✅ Creates GitHub Release with:
   - Binaries for all platforms
   - Checksums
   - Auto-generated changelog

### 4. Verify the Release

Check the GitHub Actions workflow:

```bash
gh run list --workflow=release.yaml
gh run view <run-id>
```

Or visit: https://github.com/OpenCHAMI/fabrica/actions

### 5. Test the Release

#### Test Binaries

Download and test a binary:

```bash
# Linux AMD64
wget https://github.com/OpenCHAMI/fabrica/releases/download/v1.0.0/fabrica_1.0.0_linux_x86_64.tar.gz
tar xzf fabrica_1.0.0_linux_x86_64.tar.gz
./fabrica version

# macOS ARM64
wget https://github.com/OpenCHAMI/fabrica/releases/download/v1.0.0/fabrica_1.0.0_darwin_arm64.tar.gz
tar xzf fabrica_1.0.0_darwin_arm64.tar.gz
./fabrica version
```

#### Test Docker Images

```bash
# Pull and test
docker pull ghcr.io/openchami/fabrica:v1.0.0
docker run --rm ghcr.io/openchami/fabrica:v1.0.0 version

# Test multi-arch (should work on both AMD64 and ARM64)
docker pull ghcr.io/openchami/fabrica:latest
docker run --rm ghcr.io/openchami/fabrica:latest --help
```

## Release Configuration

The release process is configured in:

- **`.goreleaser.yaml`** - GoReleaser configuration
  - Binary builds for multiple platforms
  - Docker image builds
  - Archive creation
  - Changelog generation

- **`.github/workflows/release.yaml`** - GitHub Actions workflow
  - Triggered on version tags
  - Runs GoReleaser
  - Publishes to GitHub Releases and GHCR

- **`Dockerfile`** - Multi-stage Docker build
  - Minimal Alpine-based image
  - Non-root user
  - Includes templates and docs

## Supported Platforms

### Binaries

| Platform | Architecture | Binary |
|----------|-------------|---------|
| Linux | AMD64 | `fabrica_*_linux_x86_64.tar.gz` |
| Linux | ARM64 | `fabrica_*_linux_arm64.tar.gz` |
| macOS | ARM64 | `fabrica_*_darwin_arm64.tar.gz` |

### Docker Images

| Platform | Image |
|----------|-------|
| Linux AMD64 | `ghcr.io/openchami/fabrica:*-amd64` |
| Linux ARM64 | `ghcr.io/openchami/fabrica:*-arm64` |
| Multi-arch | `ghcr.io/openchami/fabrica:*` |

## Version Numbering

Fabrica follows [Semantic Versioning](https://semver.org/):

- **Major version (v1.0.0)** - Breaking changes
- **Minor version (v1.1.0)** - New features (backward compatible)
- **Patch version (v1.0.1)** - Bug fixes

## Troubleshooting

### Release Failed

If the GitHub Actions workflow fails:

1. Check the workflow logs in GitHub Actions
2. Fix any issues
3. Delete the tag: `git tag -d v1.0.0 && git push origin :refs/tags/v1.0.0`
4. Fix the code and re-tag

### Docker Push Failed

Ensure the `GITHUB_TOKEN` has package write permissions:
- Repository Settings → Actions → General → Workflow permissions
- Enable "Read and write permissions"

### Binary Build Failed

Check the GoReleaser configuration:

```bash
# Test locally (creates snapshot, doesn't publish)
goreleaser release --snapshot --clean
```

## Manual Release

To create a release manually without pushing a tag:

```bash
# Install GoReleaser
brew install goreleaser/tap/goreleaser

# Create snapshot (doesn't publish)
goreleaser release --snapshot --clean

# Publish release (requires GITHUB_TOKEN)
export GITHUB_TOKEN=your_token
goreleaser release --clean
```

## Post-Release

After a successful release:

1. ✅ Update documentation if needed
2. ✅ Announce the release (Slack, Discord, etc.)
3. ✅ Close related issues/PRs
4. ✅ Update project roadmap

## Resources

- [GoReleaser Documentation](https://goreleaser.com)
- [GitHub Actions Documentation](https://docs.github.com/en/actions)
- [Semantic Versioning](https://semver.org/)
