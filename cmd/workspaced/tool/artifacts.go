package tool

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"text/tabwriter"

	"github.com/lewtec/lewkit/x/cmd"
	"github.com/lucasew/workspaced/internal/tool"
	"github.com/lucasew/workspaced/internal/tool/backend"

	parsespec "github.com/lucasew/workspaced/internal/parse/spec"
)

type Artifacts struct {
	Hint    cmd.StringArg `short:"H" long:"hint" help:"binary name hint for scoring (overrides the default derived from the package name)"`
	spec    cmd.StringArg
	version *cmd.StringArg
}

func (Artifacts) Description() string {
	return "List artifacts for a tool and rank them by ScoreArtifact weight for the current platform"
}

func (a *Artifacts) Run(ctx context.Context) error {
	specStr := a.spec.Value()
	spec, err := parsespec.Parse(specStr)
	if err != nil {
		return err
	}

	version := spec.Version
	if a.version != nil {
		version = a.version.Value()
	}

	p, err := tool.Get(spec.Provider)
	if err != nil {
		return err
	}
	t, err := p.Tool(spec.Package)
	if err != nil {
		return err
	}

	at, ok := t.(backend.ArtifactTool)
	if !ok {
		return fmt.Errorf("the resolved tool for %q does not implement ArtifactTool (cannot list raw artifacts)", specStr)
	}

	artifacts, err := at.ListArtifacts(ctx, version)
	if err != nil {
		return err
	}

	effectiveHint := a.Hint.Value()
	if effectiveHint == "" {
		effectiveHint = filepath.Base(spec.Package)
	}

	type entry struct {
		backend.Artifact
		Score int
	}

	entries := make([]entry, len(artifacts))
	for i, art := range artifacts {
		entries[i] = entry{
			Artifact: art,
			Score:    backend.ScoreArtifact(art, runtime.GOOS, runtime.GOARCH, effectiveHint),
		}
	}

	sort.Slice(entries, func(i, j int) bool {
		if entries[i].Score != entries[j].Score {
			return entries[i].Score > entries[j].Score
		}
		li := len(entries[i].URL)
		lj := len(entries[j].URL)
		if li != lj {
			return li < lj
		}
		return i < j
	})

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintf(os.Stdout, "# platform=%s/%s  hint=%q  version=%s  (0=ineligible)\n",
		runtime.GOOS, runtime.GOARCH, effectiveHint, version)
	fmt.Fprintln(w, "SCORE\tOS\tARCH\tURL\tHASH\tSIZE")

	for _, e := range entries {
		sizeStr := "-"
		if e.Size > 0 {
			sizeStr = fmt.Sprintf("%d", e.Size)
		}
		hashStr := e.Hash
		if hashStr == "" {
			hashStr = "-"
		}
		osStr := e.OS
		if osStr == "" {
			osStr = "-"
		}
		archStr := e.Arch
		if archStr == "" {
			archStr = "-"
		}
		fmt.Fprintf(w, "%d\t%s\t%s\t%s\t%s\t%s\n",
			e.Score, osStr, archStr, e.URL, hashStr, sizeStr)
	}
	return w.Flush()
}
