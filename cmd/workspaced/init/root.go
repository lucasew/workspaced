package init

import (
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"net"
	"os"
	"path/filepath"
	"text/template"
	"workspaced/pkg/constants"
	envdriver "workspaced/pkg/driver/env"
	"workspaced/pkg/env"

	"github.com/spf13/cobra"
)

//go:embed templates
var templatesFS embed.FS

func NewCommand() *cobra.Command {
	var force bool

	cmd := &cobra.Command{
		Use:   "init",
		Short: "Initialize workspaced dotfiles",
		Long: `Initialize workspaced configuration and modules.

This command will:
  1. Generate settings.toml in $DOTFILES (or ~/.dotfiles)
  2. Copy example module to $DOTFILES/modules/
  3. Auto-detect hostname and local IPs

Before running this, install the binary with:
  workspaced self-install`,
		RunE: func(c *cobra.Command, args []string) error {
			return runInit(force)
		},
	}

	cmd.Flags().BoolVarP(&force, "force", "f", false, "Force overwrite existing config")

	return cmd
}

func runInit(force bool) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get home directory: %w", err)
	}

	// 1. Detect dotfiles root
	dotfilesRoot, err := env.GetDotfilesRoot()
	if err != nil || dotfilesRoot == "" {
		// Use first candidate from constants (typically ~/.dotfiles)
		dotfilesRoot = envdriver.ExpandPath(constants.DotfilesCandidates[0])
		if dotfilesRoot == "" {
			dotfilesRoot = filepath.Join(home, ".dotfiles") // Absolute fallback
		}
		fmt.Printf("📁 Creating dotfiles directory: %s\n", dotfilesRoot)
		if err := os.MkdirAll(dotfilesRoot, 0755); err != nil {
			return fmt.Errorf("failed to create dotfiles directory: %w", err)
		}
	} else {
		fmt.Printf("📁 Using dotfiles directory: %s\n", dotfilesRoot)
	}

	// 2. Generate config from template
	configPath := filepath.Join(dotfilesRoot, "settings.toml")
	if !force {
		if _, err := os.Stat(configPath); err == nil {
			return fmt.Errorf("config already exists at %s (use --force to overwrite)", configPath)
		}
	}

	fmt.Printf("\n📝 Generating config from template...\n")
	if err := generateConfig(configPath); err != nil {
		return fmt.Errorf("failed to generate config: %w", err)
	}
	fmt.Printf("   ✓ Config created: %s\n", configPath)

	// 3. Copy modules
	fmt.Printf("\n📦 Installing example module...\n")
	modulesDir := filepath.Join(dotfilesRoot, "modules")
	if err := copyEmbeddedModules(modulesDir); err != nil {
		return fmt.Errorf("failed to copy modules: %w", err)
	}
	fmt.Printf("   ✓ Modules installed: %s\n", modulesDir)

	// 4. Success message
	fmt.Printf("\n✅ Initialization complete!\n\n")
	fmt.Printf("Next steps:\n")
	fmt.Printf("  1. Edit config: %s\n", configPath)
	fmt.Printf("  2. Review example module: %s\n", filepath.Join(modulesDir, "example"))
	fmt.Printf("  3. Apply config: workspaced apply\n")

	return nil
}

func generateConfig(configPath string) error {
	// Read embedded template
	tmplContent, err := templatesFS.ReadFile("templates/init/settings.toml.tmpl")
	if err != nil {
		return fmt.Errorf("failed to read template: %w", err)
	}

	// Prepare template data
	hostname, _ := os.Hostname()
	if hostname == "" {
		hostname = "localhost"
	}

	localIPs := getLocalIPs()

	data := map[string]interface{}{
		"Hostname": hostname,
		"LocalIPs": localIPs,
	}

	// Parse and execute template
	tmpl, err := template.New("settings").Funcs(template.FuncMap{
		"toJSON": func(v interface{}) string {
			b, err := json.Marshal(v)
			if err != nil {
				return "[]"
			}
			return string(b)
		},
	}).Parse(string(tmplContent))
	if err != nil {
		return fmt.Errorf("failed to parse template: %w", err)
	}

	// Create config file
	f, err := os.Create(configPath)
	if err != nil {
		return fmt.Errorf("failed to create config file: %w", err)
	}
	defer f.Close()

	if err := tmpl.Execute(f, data); err != nil {
		return fmt.Errorf("failed to execute template: %w", err)
	}

	return nil
}

func copyEmbeddedModules(modulesDir string) error {
	// Walk the embedded templates/init/modules directory
	return fs.WalkDir(templatesFS, "templates/init/modules", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		// Calculate relative path from templates/init/modules
		relPath, err := filepath.Rel("templates/init/modules", path)
		if err != nil {
			return err
		}

		targetPath := filepath.Join(modulesDir, relPath)

		if d.IsDir() {
			return os.MkdirAll(targetPath, 0755)
		}

		// Copy file
		content, err := templatesFS.ReadFile(path)
		if err != nil {
			return err
		}

		return os.WriteFile(targetPath, content, 0644)
	})
}

func getLocalIPs() []string {
	var ips []string
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return ips
	}

	for _, addr := range addrs {
		if ipnet, ok := addr.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
			ips = append(ips, ipnet.IP.String())
		}
	}
	return ips
}
