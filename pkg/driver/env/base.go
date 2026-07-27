package env

import "context"

// Base implements the shared home-relative methods of Driver.
// Platform drivers embed Base and override IsPhone, IsNixOS, and GetEssentialPaths.
type Base struct{}

func (Base) GetHomeDir(ctx context.Context) (string, error) {
	return ResolveHomeDir()
}

func (Base) GetDotfilesRoot(ctx context.Context) (string, error) {
	home, err := ResolveHomeDir()
	if err != nil {
		return "", err
	}
	return FindDotfilesRoot(home)
}

func (Base) GetHostname(ctx context.Context) (string, error) {
	return Hostname(ctx)
}

func (Base) GetUserDataDir(ctx context.Context) (string, error) {
	home, err := ResolveHomeDir()
	if err != nil {
		return "", err
	}
	return EnsureUnderHome(home, ".local/share/workspaced")
}

func (Base) GetConfigDir(ctx context.Context) (string, error) {
	home, err := ResolveHomeDir()
	if err != nil {
		return "", err
	}
	return EnsureUnderHome(home, ".config/workspaced")
}
