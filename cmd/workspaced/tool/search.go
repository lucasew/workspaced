package tool

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/lewtec/lewkit/x/cmd"
	"github.com/lucasew/workspaced/internal/tool/backend/catalog"
)

type Search struct {
	query *cmd.StringArg
}

func (Search) Description() string {
	return "Search (or list) curated short names in the tool catalog"
}

func (s *Search) Run(ctx context.Context) error {
	query := ""
	if s.query != nil {
		query = strings.ToLower(strings.TrimSpace(s.query.Value()))
	}
	for _, name := range catalog.ListTools() {
		if query == "" || strings.Contains(strings.ToLower(name), query) {
			fmt.Fprintln(os.Stdout, name)
		}
	}
	return nil
}
