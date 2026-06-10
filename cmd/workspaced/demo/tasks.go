package demo

import (
	"context"
	"fmt"
	"time"

	"workspaced/pkg/output"
	"workspaced/pkg/taskgroup"

	"github.com/spf13/cobra"
)

func init() {
	Registry.Register(func(parent *cobra.Command) {
		parent.AddCommand(&cobra.Command{
			Use:   "tasks",
			Short: "Run a set of tasks that demonstrate progress bars, logs, pools and dependencies",
			RunE: func(cmd *cobra.Command, args []string) error {
				return runTasksDemo(cmd)
			},
		})
	})
}

func init() {
	Registry.Register(func(parent *cobra.Command) {
		parent.AddCommand(&cobra.Command{
			Use:   "plain",
			Short: "Run a self-contained demo using output.NewPlain against a local task group",
			RunE: func(cmd *cobra.Command, args []string) error {
				cmd.Println("=== workspaced demo: plain renderer (standalone) ===")
				cmd.Println("This exercises pkg/output/plain.go and pkg/taskgroup directly.")
				cmd.Println()

				g, _ := taskgroup.New(context.Background(), taskgroup.Limits{IO: 2, CPU: 2, Internet: 2})
				r := output.NewPlain(cmd.ErrOrStderr())

				// Start renderer in background like the real root does.
				done := make(chan struct{})
				go func() {
					defer close(done)
					_ = r.Run(g)
				}()

				g.Go("fetch", taskgroup.Internet, func(ctx context.Context, s *taskgroup.Status) error {
					s.Update("contacting API")
					time.Sleep(80 * time.Millisecond)
					s.Log("200 OK")
					s.Progress(0, 4)
					for i := 1; i <= 4; i++ {
						s.Progress(int64(i), 4)
						s.Update(fmt.Sprintf("page %d", i))
						time.Sleep(90 * time.Millisecond)
					}
					return nil
				})

				g.Go("process", taskgroup.CPU, func(ctx context.Context, s *taskgroup.Status) error {
					s.Update("crunching numbers")
					for i := 0; i < 3; i++ {
						s.Log(fmt.Sprintf("batch %d processed", i))
						time.Sleep(110 * time.Millisecond)
					}
					return nil
				}, "fetch")

				g.Go("write", taskgroup.IO, func(ctx context.Context, s *taskgroup.Status) error {
					s.Update("writing artifacts")
					time.Sleep(150 * time.Millisecond)
					s.Log("fsync complete")
					return nil
				}, "process")

				if err := g.Wait(); err != nil {
					return err
				}
				<-done
				cmd.Println("\n(plain renderer finished)")
				return nil
			},
		})
	})
}

func init() {
	Registry.Register(func(parent *cobra.Command) {
		parent.AddCommand(&cobra.Command{
			Use:   "nested",
			Short: "Demonstrate a parent task owning a SubGroup with child tasks",
			RunE: func(cmd *cobra.Command, args []string) error {
				ctx := cmd.Context()
				g := taskgroup.FromContext(ctx)
				if g == nil {
					var c context.Context
					g, c = taskgroup.New(ctx, taskgroup.DefaultLimits())
					_ = c
				}

				cmd.Println("=== workspaced demo: nested SubGroup ===")
				cmd.Println("A top-level task 'bundle' will create a SubGroup and schedule children inside it.")
				cmd.Println("Children share pools but have their own namespacing for snapshots.")
				cmd.Println()

				g.Go("bundle", taskgroup.CPU, func(ctx context.Context, s *taskgroup.Status) error {
					s.Update("starting bundle phase")
					time.Sleep(60 * time.Millisecond)

					child, childCtx := g.SubGroup(ctx)
					_ = childCtx
					child.Go("bundle:icons", taskgroup.CPU, func(ctx context.Context, s *taskgroup.Status) error {
						s.Update("generating icons")
						for i := 0; i < 3; i++ {
							s.Log(fmt.Sprintf("icon %d", i))
							time.Sleep(90 * time.Millisecond)
						}
						return nil
					})
					child.Go("bundle:manifest", taskgroup.IO, func(ctx context.Context, s *taskgroup.Status) error {
						s.Update("writing manifest.json")
						time.Sleep(130 * time.Millisecond)
						s.Log("manifest written")
						return nil
					}, "bundle:icons")

					s.Update("waiting on subgroup children (not directly visible on root snapshot)")
					if err := child.Wait(); err != nil {
						return err
					}
					s.Update("bundle complete (children ran in SubGroup)")
					s.Log("subgroup done")
					return nil
				})

				time.Sleep(30 * time.Millisecond)
				return nil
			},
		})
	})
}
