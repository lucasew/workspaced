package executil

import (
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"time"
	"workspaced/pkg/logging"
)

type stdoutKey struct{}
type stderrKey struct{}
type envKey struct{}

// WithStdout returns a context carrying an io.Writer for standard output.
func WithStdout(ctx context.Context, w io.Writer) context.Context {
	return context.WithValue(ctx, stdoutKey{}, w)
}

// Stdout retrieves the stdout writer from the context, or nil if not set.
func Stdout(ctx context.Context) io.Writer {
	w, _ := ctx.Value(stdoutKey{}).(io.Writer)
	return w
}

// WithStderr returns a context carrying an io.Writer for standard error.
func WithStderr(ctx context.Context, w io.Writer) context.Context {
	return context.WithValue(ctx, stderrKey{}, w)
}

// Stderr retrieves the stderr writer from the context, or nil if not set.
func Stderr(ctx context.Context) io.Writer {
	w, _ := ctx.Value(stderrKey{}).(io.Writer)
	return w
}

// WithEnv returns a context carrying environment variables as a slice of "KEY=VALUE" strings.
func WithEnv(ctx context.Context, env []string) context.Context {
	return context.WithValue(ctx, envKey{}, env)
}

// Env retrieves the environment variable slice from the context, or nil if not set.
func Env(ctx context.Context) []string {
	env, _ := ctx.Value(envKey{}).([]string)
	return env
}

// InheritContextWriters configures the command's Stdout and Stderr to write to the writers
// stored in the context, allowing output capture or redirection.
func InheritContextWriters(ctx context.Context, cmd *exec.Cmd) {
	if stdout := Stdout(ctx); stdout != nil {
		cmd.Stdout = stdout
	}
	if stderr := Stderr(ctx); stderr != nil {
		cmd.Stderr = stderr
	}
}

// GetEnv retrieves an environment variable from the context or the system.
func GetEnv(ctx context.Context, key string) string {
	if env := Env(ctx); env != nil {
		for _, e := range env {
			if strings.HasPrefix(e, key+"=") {
				return e[len(key)+1:]
			}
		}
	}
	return os.Getenv(key)
}

// GetBinaryHash returns the SHA256 hash of the current executable
func GetBinaryHash(ctx context.Context) (string, error) {
	exePath, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("failed to get executable path: %w", err)
	}

	file, err := os.Open(exePath)
	if err != nil {
		return "", fmt.Errorf("failed to open executable: %w", err)
	}
	defer logging.Close(ctx, file)

	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", fmt.Errorf("failed to hash executable: %w", err)
	}

	return fmt.Sprintf("%x", hash.Sum(nil)), nil
}

// GetBinaryMtime returns the modification time of the current executable.
func GetBinaryMtime() (time.Time, error) {
	exePath, err := os.Executable()
	if err != nil {
		return time.Time{}, fmt.Errorf("failed to get executable path: %w", err)
	}

	info, err := os.Stat(exePath)
	if err != nil {
		return time.Time{}, fmt.Errorf("failed to stat executable: %w", err)
	}

	return info.ModTime(), nil
}
