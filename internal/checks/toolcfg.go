package checks

import (
	"context"
	"errors"
	"fmt"
	"sort"

	"workspaced/internal/configcue"
)

// Tool is one CUE-declared linter or formatter.
type Tool struct {
	Name          string
	Enable        bool
	Detect        map[string]DetectRule
	Needs         map[string]bool
	Cmd           []string
	Output        string
	ArgsFromGlobs bool
}

// DetectRule is one ordered firewall entry.
type DetectRule struct {
	Path   string `json:"path"`
	Glob   string `json:"glob"`
	Enable bool   `json:"enable"`
}

// ToolsSection is the decoded lint or formatter block.
type ToolsSection struct {
	Tools map[string]toolJSON `json:"tools"`
}

type toolJSON struct {
	Enable        *bool                 `json:"enable"`
	Detect        map[string]DetectRule `json:"detect"`
	Needs         map[string]bool       `json:"needs"`
	Cmd           []string              `json:"cmd"`
	Output        string                `json:"output"`
	ArgsFromGlobs bool                  `json:"args_from_globs"`
}

// LoadTools decodes workspaced.<section> (lint or formatter) into ordered tools.
func LoadTools(cfg *configcue.Config, section string) ([]Tool, error) {
	if cfg == nil {
		return nil, nil
	}
	var raw ToolsSection
	if err := cfg.Decode(section, &raw); err != nil {
		if errors.Is(err, configcue.ErrKeyNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("decode %s config: %w", section, err)
	}
	if len(raw.Tools) == 0 {
		return nil, nil
	}
	names := make([]string, 0, len(raw.Tools))
	for name := range raw.Tools {
		names = append(names, name)
	}
	sort.Strings(names)

	out := make([]Tool, 0, len(names))
	for _, name := range names {
		j := raw.Tools[name]
		enable := true
		if j.Enable != nil {
			enable = *j.Enable
		}
		if len(j.Cmd) == 0 {
			return nil, fmt.Errorf("%s.tools.%s: empty cmd", section, name)
		}
		out = append(out, Tool{
			Name:          name,
			Enable:        enable,
			Detect:        j.Detect,
			Needs:         j.Needs,
			Cmd:           append([]string(nil), j.Cmd...),
			Output:        j.Output,
			ArgsFromGlobs: j.ArgsFromGlobs,
		})
	}
	return out, nil
}

// LoadToolsForDir loads workspace config for dir and returns section tools.
func LoadToolsForDir(ctx context.Context, dir, section string) ([]Tool, error) {
	cfg, err := configcue.LoadForWorkspace(ctx, dir)
	if err != nil {
		return nil, err
	}
	return LoadTools(cfg, section)
}
