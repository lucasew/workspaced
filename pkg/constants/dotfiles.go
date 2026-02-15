package constants

// DotfilesCandidates is the list of paths to search for dotfiles root.
// Paths are checked in order, first match wins.
// Supports ~ for home directory and $VAR for environment variables.
var DotfilesCandidates = []string{
	"/workspaces/.codespaces/.persistedshare/dotfiles", // GitHub Codespaces
	"~/.dotfiles",                                       // User dotfiles
	"/etc/.dotfiles",                                    // System-wide dotfiles
}
