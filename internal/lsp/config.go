// Package lsp implements the workspaced language-server router proxy.
package lsp

import (
	"errors"
	"fmt"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"workspaced/internal/configcue"
)

const defaultRequestTimeout = 10 * time.Second

// Config is the decoded workspaced.lsp block.
type Config struct {
	Extensions     map[string]string              `json:"extensions"`
	LanguageIDs    map[string]string              `json:"language_ids"`
	Languages      map[string]map[string]Attachment `json:"languages"`
	Servers        map[string]Server              `json:"servers"`
	RequestTimeout string                         `json:"request_timeout"`
}

// Attachment binds an ordered language entry to capability flags.
type Attachment struct {
	Capabilities map[string]bool `json:"capabilities"`
}

// Server is a language-server process definition.
type Server struct {
	Cmd   []string        `json:"cmd"`
	Needs map[string]bool `json:"needs"`
}

// LanguageBinding is one ordered server attachment after resolving order keys.
type LanguageBinding struct {
	OrderKey     string
	ServerID     string
	Capabilities map[string]bool // nil/empty = all
}

var orderPrefix = regexp.MustCompile(`^(\d+_)?(.*)$`)

// LoadConfig decodes workspaced.lsp from a loaded config. Missing block yields empty Config.
func LoadConfig(cfg *configcue.Config) (Config, error) {
	if cfg == nil {
		return Config{}, nil
	}
	var out Config
	if err := cfg.Decode("lsp", &out); err != nil {
		if errors.Is(err, configcue.ErrKeyNotFound) {
			return Config{}, nil
		}
		return Config{}, fmt.Errorf("decode lsp config: %w", err)
	}
	normalizeConfig(&out)
	return out, nil
}

func normalizeConfig(c *Config) {
	if c.Extensions == nil {
		c.Extensions = map[string]string{}
	}
	if c.LanguageIDs == nil {
		c.LanguageIDs = map[string]string{}
	}
	// Normalize extension keys to include a leading dot and lowercase.
	normExt := make(map[string]string, len(c.Extensions))
	for k, v := range c.Extensions {
		k = strings.TrimSpace(k)
		if k == "" {
			continue
		}
		if !strings.HasPrefix(k, ".") {
			k = "." + k
		}
		normExt[strings.ToLower(k)] = strings.TrimSpace(v)
	}
	c.Extensions = normExt

	normIDs := make(map[string]string, len(c.LanguageIDs))
	for k, v := range c.LanguageIDs {
		normIDs[strings.TrimSpace(k)] = strings.TrimSpace(v)
	}
	c.LanguageIDs = normIDs
}

// Timeout returns the per-backend request soft timeout.
func (c Config) Timeout() time.Duration {
	if strings.TrimSpace(c.RequestTimeout) == "" {
		return defaultRequestTimeout
	}
	d, err := time.ParseDuration(c.RequestTimeout)
	if err != nil || d <= 0 {
		return defaultRequestTimeout
	}
	return d
}

// ResolveLanguage maps path + editor languageId to our language id.
// Extension map wins; language_ids is fallback.
func (c Config) ResolveLanguage(pathOrURI, languageID string) string {
	path := uriToPath(pathOrURI)
	ext := strings.ToLower(filepath.Ext(path))
	if ext != "" {
		if lang, ok := c.Extensions[ext]; ok && lang != "" {
			return lang
		}
	}
	// basename match for extensionless files (e.g. Dockerfile) — key ".dockerfile" style not used;
	// allow keys that are the full base name stored as ".Dockerfile" unlikely; skip.
	if languageID != "" {
		if lang, ok := c.LanguageIDs[languageID]; ok && lang != "" {
			return lang
		}
	}
	return ""
}

// BindingsFor returns ordered server bindings for a language.
func (c Config) BindingsFor(language string) []LanguageBinding {
	raw, ok := c.Languages[language]
	if !ok || len(raw) == 0 {
		return nil
	}
	keys := make([]string, 0, len(raw))
	for k := range raw {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	out := make([]LanguageBinding, 0, len(keys))
	for _, k := range keys {
		att := raw[k]
		serverID := serverIDFromOrderKey(k)
		if serverID == "" {
			continue
		}
		if _, ok := c.Servers[serverID]; !ok {
			// Still emit binding; spawn will fail with a clear error.
		}
		caps := att.Capabilities
		out = append(out, LanguageBinding{
			OrderKey:     k,
			ServerID:     serverID,
			Capabilities: caps,
		})
	}
	return out
}

func serverIDFromOrderKey(key string) string {
	m := orderPrefix.FindStringSubmatch(key)
	if len(m) == 3 && m[2] != "" {
		return m[2]
	}
	return key
}

// HasCapability reports whether the binding handles cap.
// Empty capabilities map means all.
func (b LanguageBinding) HasCapability(cap string) bool {
	if len(b.Capabilities) == 0 {
		return true
	}
	// Allow either exact or true-valued entries; missing = false when map non-empty.
	v, ok := b.Capabilities[cap]
	return ok && v
}

// CapabilityForMethod maps an LSP method to a capability flag name.
func CapabilityForMethod(method string) string {
	switch method {
	case "textDocument/hover":
		return "hover"
	case "textDocument/definition":
		return "definition"
	case "textDocument/typeDefinition":
		return "typeDefinition"
	case "textDocument/implementation":
		return "implementation"
	case "textDocument/references":
		return "references"
	case "textDocument/completion":
		return "completion"
	case "textDocument/signatureHelp":
		return "signatureHelp"
	case "textDocument/documentHighlight":
		return "documentHighlight"
	case "textDocument/documentSymbol":
		return "documentSymbol"
	case "textDocument/codeAction":
		return "codeAction"
	case "textDocument/codeLens":
		return "codeLens"
	case "textDocument/formatting":
		return "formatting"
	case "textDocument/rangeFormatting":
		return "rangeFormatting"
	case "textDocument/rename":
		return "rename"
	case "textDocument/prepareRename":
		return "prepareRename"
	case "textDocument/inlayHint":
		return "inlayHint"
	case "textDocument/semanticTokens/full", "textDocument/semanticTokens/range":
		return "semanticTokens"
	case "textDocument/documentLink":
		return "documentLink"
	case "textDocument/foldingRange":
		return "foldingRange"
	case "textDocument/selectionRange":
		return "selectionRange"
	case "workspace/symbol":
		return "workspaceSymbol"
	case "textDocument/diagnostic", "textDocument/publishDiagnostics":
		return "diagnostics"
	default:
		// Unknown methods: treat capability name as last path segment.
		if i := strings.LastIndex(method, "/"); i >= 0 {
			return method[i+1:]
		}
		return method
	}
}

func uriToPath(uri string) string {
	uri = strings.TrimSpace(uri)
	if uri == "" {
		return ""
	}
	const prefix = "file://"
	if strings.HasPrefix(uri, prefix) {
		p := strings.TrimPrefix(uri, prefix)
		// file:///path -> /path; Windows file:///C:/... left as-is for now
		if strings.HasPrefix(p, "/") && len(p) >= 3 && p[2] == ':' {
			// file:///C:/...
			return p[1:]
		}
		return p
	}
	return uri
}

func pathToURI(path string) string {
	path = filepath.ToSlash(path)
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	return "file://" + path
}
