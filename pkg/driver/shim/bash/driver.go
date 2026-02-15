package bash

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"workspaced/pkg/driver"
	execdriver "workspaced/pkg/driver/exec"
	shimdriver "workspaced/pkg/driver/shim"
)

type Provider struct{}

func (p *Provider) ID() string {
	return "shim_bash"
}

func (p *Provider) Name() string {
	return "Bash Shim"
}

func (p *Provider) DefaultWeight() int {
	return 50
}

func (p *Provider) CheckCompatibility(ctx context.Context) error {
	_, err := execdriver.Which(ctx, "bash")
	return err
}

func (p *Provider) New(ctx context.Context) (shimdriver.Driver, error) {
	// Prefer $SHELL if it's bash, otherwise use which bash
	bashPath := os.Getenv("SHELL")
	if bashPath == "" || !strings.Contains(bashPath, "bash") {
		var err error
		bashPath, err = execdriver.Which(ctx, "bash")
		if err != nil {
			return nil, fmt.Errorf("bash not found: %w", err)
		}
	}
	return &Driver{bashPath: bashPath}, nil
}

type Driver struct {
	bashPath string
}

// GenerateContent creates the shim script content
func (d *Driver) GenerateContent(command []string) (string, error) {
	if len(command) == 0 {
		return "", fmt.Errorf("command cannot be empty")
	}

	// Escape command for shell
	escapedCmd := make([]string, len(command))
	for i, arg := range command {
		// Quote arguments that contain spaces or special characters
		if strings.ContainsAny(arg, " \t\n\"'$`\\|&;<>()[]{}*?!") {
			escapedCmd[i] = fmt.Sprintf("'%s'", strings.ReplaceAll(arg, "'", `'"'"'`))
		} else {
			escapedCmd[i] = arg
		}
	}

	// Generate shim script
	shebang := fmt.Sprintf("#!%s\n", d.bashPath)
	shimContent := shebang + fmt.Sprintf("exec %s \"$@\"\n", strings.Join(escapedCmd, " "))

	return shimContent, nil
}

func (d *Driver) Generate(ctx context.Context, path string, command []string) error {
	content, err := d.GenerateContent(command)
	if err != nil {
		return err
	}

	// Ensure parent directory exists
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", dir, err)
	}

	// Write shim file
	if err := os.WriteFile(path, []byte(content), 0755); err != nil {
		return fmt.Errorf("failed to write shim to %s: %w", path, err)
	}

	return nil
}

func init() {
	driver.Register[shimdriver.Driver](&Provider{})
}
