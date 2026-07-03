# Installation Guide

Workspaced can be installed via a simple curl/bash script that downloads the correct pre-built binary for your platform directly from GitHub Releases.

## Quick Install

To install the latest version, run the following command:

```bash
curl -fsSL https://raw.githubusercontent.com/lucasew/workspaced/main/setup | bash
```

## Environment Variables

The installation script supports several environment variables to customize the installation process:

- `REPO` (default: `lucasew/workspaced`): The GitHub repository in `owner/repo` format to download releases from.
- `APPNAME` (default: `workspaced`): The expected binary name to extract or download.
- `VERSION` (default: `latest`): The release tag to download (e.g., `v1.2.3`), or `latest` for the most recent release.
- `OS` (default: auto-detected): Override the target operating system (e.g., `linux`, `darwin`, `windows`).
- `ARCH` (default: auto-detected): Override the target architecture (e.g., `amd64`, `arm64`, `386`).
- `DOWNLOAD_DIR` (default: temporary directory): Directory to store downloaded archives.
- `GITHUB_TOKEN` (optional): Provide a GitHub token to avoid API rate limits when fetching release information. If the `gh` CLI is installed and authenticated, it will try to use its token automatically.

### Example with overrides

```bash
curl -fsSL https://raw.githubusercontent.com/lucasew/workspaced/main/setup | VERSION=v0.1.0 ARCH=arm64 bash
```
