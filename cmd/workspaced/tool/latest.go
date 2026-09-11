package tool

import (
	"context"
	"fmt"
	"os"

	"github.com/lewtec/lewkit/x/cmd"
)

type Latest struct {
	spec cmd.StringArg
}

func (Latest) Description() string {
	return "Print the latest version string for a tool ref"
}

func (l *Latest) Run(ctx context.Context) error {
	specStr := l.spec.Value()
	versions, err := listVersions(ctx, specStr)
	if err != nil {
		return err
	}
	if len(versions) == 0 {
		return fmt.Errorf("no versions found for %s", specStr)
	}
	fmt.Fprintln(os.Stdout, versions[0])
	return nil
}
