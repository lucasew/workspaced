package termux

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"workspaced/pkg/api"
	"workspaced/pkg/driver"
	envdriver "workspaced/pkg/driver/env"
	execdriver "workspaced/pkg/driver/exec"
	"workspaced/pkg/types"
)

type Provider struct{}

func (p *Provider) ID() string {
	return "exec_termux"
}

func (p *Provider) Name() string {
	return "Termux"
}

func (p *Provider) DefaultWeight() int {
	// Higher weight than native driver (50) to ensure Termux driver is preferred on Android
	return 60
}

func (p *Provider) CheckCompatibility(ctx context.Context) error {
	// Check if we're running in Termux
	if os.Getenv("TERMUX_VERSION") == "" {
		return fmt.Errorf("%w: not running in Termux", driver.ErrIncompatible)
	}
	return nil
}

func (p *Provider) New(ctx context.Context) (execdriver.Driver, error) {
	return &Driver{}, nil
}

type Driver struct{}

func (d *Driver) Run(ctx context.Context, name string, args ...string) *exec.Cmd {
	// Get Termux PREFIX
	prefix := os.Getenv("PREFIX")
	if prefix == "" {
		prefix = "/data/data/com.termux/files/usr"
	}

	// Resolve the full path using custom Which to avoid SIGSYS on Android
	fullPath, err := d.Which(ctx, name)
	if err != nil {
		// If Which fails, fall back to the original name
		fullPath = name
	}

	fullArgs := []string{fullPath}
	fullArgs = append(fullArgs, args...)

	// Check if we're already inside proot (avoid nesting)
	inProot := os.Getenv("WORKSPACED_IN_PROOT")
	slog.Info("proot check", "WORKSPACED_IN_PROOT", inProot, "command", fullArgs)
	if inProot == "1" {
		slog.Info("already inside proot, running directly", "command", fullArgs)
		cmd := exec.CommandContext(ctx, fullPath, args...)
		cmd.Env = setupTermuxEnv(prefix)
		return cmd
	}

	// Check if user explicitly disabled proot
	if os.Getenv("WORKSPACED_NO_PROOT") == "1" {
		slog.Debug("proot disabled via WORKSPACED_NO_PROOT", "command", fullArgs)
		cmd := exec.CommandContext(ctx, fullPath, args...)
		cmd.Env = setupTermuxEnv(prefix)
		return cmd
	}

	// Default: use proot for DNS/SSL access
	slog.Debug("using proot for command execution", "command", fullArgs)
	return d.runWithProot(ctx, fullPath, args, prefix)
}

// runWithProot wraps command execution in proot with termux-chroot-like setup
func (d *Driver) runWithProot(ctx context.Context, fullPath string, args []string, prefix string) *exec.Cmd {
	// Setup resolv.conf and SSL certs for proot environment
	if resolvPath, err := ensureResolvConf(); err != nil {
		slog.Warn("failed to setup resolv.conf", "error", err)
	} else {
		slog.Debug("resolv.conf configured", "path", resolvPath)
	}

	if certPath, err := ensureSSLCerts(); err != nil {
		slog.Warn("failed to setup SSL certificates", "error", err)
	} else {
		slog.Debug("SSL certificates configured", "path", certPath)
	}

	// Find proot binary
	prootPath, err := d.Which(ctx, "proot")
	if err != nil {
		panic(err)
	}

	// Build proot arguments (mimicking termux-chroot)
	prootArgs := []string{
		"--kill-on-exit",
		"-b", "/system:/system",
		"-b", "/vendor:/vendor",
		"-b", "/data:/data",
	}

	// Bind /sbin and /root if they exist (for Magisk)
	if _, err := os.Stat("/sbin"); err == nil {
		prootArgs = append(prootArgs, "-b", "/sbin:/sbin")
	}
	if _, err := os.Stat("/root"); err == nil {
		prootArgs = append(prootArgs, "-b", "/root:/root")
	}

	// Bind /apex if exists (Android 10+)
	if _, err := os.Stat("/apex"); err == nil {
		prootArgs = append(prootArgs, "-b", "/apex:/apex")
	}

	// Bind /linkerconfig if exists (Android 11+)
	if _, err := os.Stat("/linkerconfig/ld.config.txt"); err == nil {
		prootArgs = append(prootArgs, "-b", "/linkerconfig/ld.config.txt:/linkerconfig/ld.config.txt")
	}

	// Bind /property_contexts if exists
	if _, err := os.Stat("/property_contexts"); err == nil {
		prootArgs = append(prootArgs, "-b", "/property_contexts:/property_contexts")
	}

	// Bind /storage if exists
	if _, err := os.Stat("/storage"); err == nil {
		prootArgs = append(prootArgs, "-b", "/storage:/storage")
	}

	// Bind $PREFIX to /usr
	prootArgs = append(prootArgs, "-b", prefix+":/usr")

	// Bind Termux directories
	for _, dir := range []string{"bin", "etc", "lib", "share", "tmp", "var"} {
		prootArgs = append(prootArgs, "-b", filepath.Join(prefix, dir)+":/"+dir)
	}

	// Bind system directories
	prootArgs = append(prootArgs, "-b", "/dev:/dev", "-b", "/proc:/proc")

	// Set root and working directory
	prootArgs = append(prootArgs, "-r", filepath.Dir(prefix))
	prootArgs = append(prootArgs, "--cwd=.")

	// Append command arguments
	prootArgs = append(prootArgs, fullPath)
	prootArgs = append(prootArgs, args...)

	slog.Debug("using proot for command execution", "proot", prootPath)
	cmd := exec.CommandContext(ctx, prootPath, prootArgs...)

	// Setup environment for proot
	env := os.Environ()
	filteredEnv := make([]string, 0, len(env))

	for _, e := range env {
		// Remove LD_PRELOAD to avoid termux-exec conflicts
		if strings.HasPrefix(e, "LD_PRELOAD=") {
			continue
		}
		filteredEnv = append(filteredEnv, e)
	}

	// Add required env vars
	filteredEnv = append(filteredEnv, "LD_LIBRARY_PATH=/lib:/usr/lib")
	// Set sentinel to prevent proot nesting
	filteredEnv = append(filteredEnv, "WORKSPACED_IN_PROOT=1")

	slog.Debug("setting proot environment", "sentinel", "WORKSPACED_IN_PROOT=1")
	cmd.Env = filteredEnv
	return cmd
}

// setupTermuxEnv creates environment variables for Termux binaries
func setupTermuxEnv(prefix string) []string {
	env := os.Environ()
	envMap := make(map[string]string)

	// Parse existing env
	for _, e := range env {
		parts := strings.SplitN(e, "=", 2)
		if len(parts) == 2 {
			envMap[parts[0]] = parts[1]
		}
	}

	// Remove LD_PRELOAD (causes issues with some binaries)
	delete(envMap, "LD_PRELOAD")

	// Setup SSL certificate paths (for Rust/Go binaries like mise)
	certPem := filepath.Join(prefix, "etc", "tls", "cert.pem")
	if _, err := os.Stat(certPem); err == nil {
		envMap["SSL_CERT_FILE"] = certPem
		envMap["SSL_CERT_DIR"] = filepath.Join(prefix, "etc", "tls")
		envMap["CURL_CA_BUNDLE"] = certPem
		envMap["REQUESTS_CA_BUNDLE"] = certPem
	}

	// Ensure PREFIX is set
	if _, ok := envMap["PREFIX"]; !ok {
		envMap["PREFIX"] = prefix
	}

	// Fix HOME for Termux (mise and other tools use this for shims)
	// Use env driver to get correct home (handles chroot)
	if actualHome, err := envdriver.GetHomeDir(context.Background()); err == nil {
		envMap["HOME"] = actualHome

		// Configure mise to use correct paths
		envMap["MISE_DATA_DIR"] = filepath.Join(actualHome, ".local", "share", "mise")
		envMap["MISE_CACHE_DIR"] = filepath.Join(actualHome, ".cache", "mise")
		envMap["MISE_CONFIG_DIR"] = filepath.Join(actualHome, ".config", "mise")
	}

	// Convert back to []string
	result := make([]string, 0, len(envMap))
	for k, v := range envMap {
		result = append(result, k+"="+v)
	}

	return result
}

// ensureResolvConf creates /etc/resolv.conf in Termux PREFIX if it doesn't exist or is broken
func ensureResolvConf() (string, error) {
	// Get Termux PREFIX (usually /data/data/com.termux/files/usr)
	prefix := os.Getenv("PREFIX")
	if prefix == "" {
		prefix = "/data/data/com.termux/files/usr"
	}

	resolvConfPath := filepath.Join(prefix, "etc", "resolv.conf")

	// Check if resolv.conf exists and has valid content
	if content, err := os.ReadFile(resolvConfPath); err == nil {
		contentStr := string(content)
		// Check if it has our nameserver entries
		if strings.Contains(contentStr, "8.8.8.8") {
			slog.Debug("resolv.conf already configured", "path", resolvConfPath)
			return resolvConfPath, nil
		}
	}

	// Create or overwrite with working DNS servers
	dnsConfig := `# Generated by workspaced for Termux DNS resolution
nameserver 8.8.8.8
nameserver 8.8.4.4
nameserver 1.1.1.1
`

	// Ensure etc directory exists
	etcDir := filepath.Join(prefix, "etc")
	if err := os.MkdirAll(etcDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create etc directory: %w", err)
	}

	// Write resolv.conf
	if err := os.WriteFile(resolvConfPath, []byte(dnsConfig), 0644); err != nil {
		return "", fmt.Errorf("failed to write resolv.conf: %w", err)
	}

	slog.Info("created resolv.conf for Termux DNS", "path", resolvConfPath)

	// Verify it was written
	if content, err := os.ReadFile(resolvConfPath); err != nil {
		slog.Error("failed to verify resolv.conf after writing", "error", err)
	} else {
		slog.Info("resolv.conf content verified", "size", len(content))
	}

	return resolvConfPath, nil
}

// ensureSSLCerts sets up SSL certificates in standard locations for termux-chroot
func ensureSSLCerts() (string, error) {
	prefix := os.Getenv("PREFIX")
	if prefix == "" {
		prefix = "/data/data/com.termux/files/usr"
	}

	// Source: Termux CA bundle
	sourceCert := filepath.Join(prefix, "etc", "tls", "cert.pem")
	if _, err := os.Stat(sourceCert); err != nil {
		return "", fmt.Errorf("termux CA bundle not found at %s: %w", sourceCert, err)
	}

	// Target: Standard location that most SSL libraries check
	// In termux-chroot, $PREFIX/etc becomes /etc
	sslDir := filepath.Join(prefix, "etc", "ssl", "certs")
	targetCert := filepath.Join(sslDir, "ca-certificates.crt")

	// Check if already linked/copied
	if _, err := os.Stat(targetCert); err == nil {
		slog.Debug("SSL certificates already configured", "path", targetCert)
		return targetCert, nil
	}

	// Create ssl certs directory
	if err := os.MkdirAll(sslDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create ssl certs directory %s: %w", sslDir, err)
	}

	// Verify directory was created
	if info, err := os.Stat(sslDir); err != nil {
		return "", fmt.Errorf("ssl directory does not exist after mkdir: %w", err)
	} else if !info.IsDir() {
		return "", fmt.Errorf("ssl path exists but is not a directory: %s", sslDir)
	}

	// Try to symlink first (saves space)
	// Remove existing symlink/file if it exists (broken symlink check)
	os.Remove(targetCert)

	relPath := filepath.Join("..", "..", "tls", "cert.pem")
	if err := os.Symlink(relPath, targetCert); err != nil {
		// If symlink fails, copy the file
		slog.Debug("symlink failed, copying certificate file", "error", err)
		certData, err := os.ReadFile(sourceCert)
		if err != nil {
			return "", fmt.Errorf("failed to read source cert %s: %w", sourceCert, err)
		}
		if err := os.WriteFile(targetCert, certData, 0644); err != nil {
			return "", fmt.Errorf("failed to write cert to %s: %w", targetCert, err)
		}
		slog.Info("copied SSL certificates", "from", sourceCert, "to", targetCert, "size", len(certData))
	} else {
		slog.Info("symlinked SSL certificates", "from", relPath, "to", targetCert)
	}

	return targetCert, nil
}

func (d *Driver) Which(ctx context.Context, name string) (string, error) {
	// Custom Which implementation to avoid SIGSYS errors on Android/Termux
	// Do not use os/exec.LookPath as it can trigger SIGSYS on Android with Go 1.24+

	if filepath.IsAbs(name) {
		if _, err := os.Stat(name); err == nil {
			slog.Debug("which", "binary", name, "result", name)
			return name, nil
		}
		slog.Debug("which", "binary", name, "result", api.ErrBinaryNotFound)
		return "", fmt.Errorf("%w: %s", api.ErrBinaryNotFound, name)
	}

	path := os.Getenv("PATH")
	if env, ok := ctx.Value(types.EnvKey).([]string); ok {
		for _, e := range env {
			if after, ok0 := strings.CutPrefix(e, "PATH="); ok0 {
				path = after
				break
			}
		}
	}

	for _, dir := range filepath.SplitList(path) {
		fullPath := filepath.Join(dir, name)
		if info, err := os.Stat(fullPath); err == nil && !info.IsDir() {
			slog.Debug("which", "binary", name, "result", fullPath)
			return fullPath, nil
		}
	}
	slog.Debug("which", "binary", name, "result", api.ErrBinaryNotFound)
	return "", fmt.Errorf("%w: %s", api.ErrBinaryNotFound, name)
}

func init() {
	driver.Register[execdriver.Driver](&Provider{})
}
