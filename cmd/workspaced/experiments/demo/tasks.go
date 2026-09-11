package demo

import "context"

type Tasks struct{}

func (Tasks) Description() string {
	return "Run a set of tasks that demonstrate progress bars, logs, pools and dependencies"
}

func (*Tasks) Run(ctx context.Context) error {
	return runTasksDemo(ctx)
}
