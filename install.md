# Install

Curl the setup script; it pulls the right GitHub Release binary for your platform:

```bash
curl -fsSL https://raw.githubusercontent.com/lucasew/workspaced/main/setup | bash
```

## Env vars the script reads

| Var | Default | Meaning |
|-----|---------|---------|
| `REPO` | `lucasew/workspaced` | `owner/repo` for releases |
| `APPNAME` | `workspaced` | binary name inside the archive |
| `VERSION` | `latest` | tag, or `latest` |
| `OS` | auto | `linux`, `darwin`, `windows` |
| `ARCH` | auto | `amd64`, `arm64`, `386` |
| `DOWNLOAD_DIR` | temp dir | where archives land |
| `GITHUB_TOKEN` | unset | raises API rate limits; uses `gh` auth if present |

Pin version/arch:

```bash
curl -fsSL https://raw.githubusercontent.com/lucasew/workspaced/main/setup | VERSION=v0.1.0 ARCH=arm64 bash
```
