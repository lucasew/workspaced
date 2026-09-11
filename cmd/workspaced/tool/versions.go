package tool

import (
	"context"
	"fmt"
	"os"

	"github.com/lewtec/lewkit/x/cmd"
	"github.com/lucasew/workspaced/internal/tool"

	parsespec "github.com/lucasew/workspaced/internal/parse/spec"
)

type Versions struct {
	spec cmd.StringArg
}

func (Versions) Description() string {
	return "List available versions for a tool ref from upstream"
}

func (v *Versions) Run(ctx context.Context) error {
	versions, err := listVersions(ctx, v.spec.Value())
	if err != nil {
		return err
	}
	for _, ver := range versions {
		fmt.Fprintln(os.Stdout, ver)
	}
	return nil
}

func listVersions(ctx context.Context, specStr string) ([]string, error) {
	spec, err := parsespec.Parse(specStr)
	if err != nil {
		return nil, err
	}
	p, err := tool.Get(spec.Provider)
	if err != nil {
		return nil, err
	}
	t, err := p.Tool(spec.Package)
	if err != nil {
		return nil, err
	}
	return t.ListVersions(ctx)
}
