package tool

import (
	"fmt"
	"path/filepath"
	"runtime"
	"sort"
	"text/tabwriter"

	"workspaced/internal/tool"
	"workspaced/internal/tool/backend"

	parsespec "workspaced/internal/parse/spec"

	"github.com/spf13/cobra"
)

func init() {
	Registry.Register(func(c *cobra.Command) {
		var hint string
		cmd := &cobra.Command{
			Use:   "artifacts <tool-spec> [version]",
			Short: "List artifacts for a tool and rank them by ScoreArtifact weight for the current platform",
			Long: `Fetches the list of release artifacts (when the backend supports ArtifactTool)
and displays them sorted by the score computed via backend.ScoreArtifact for the
current OS/architecture + the binary hint.

The top entry with a positive score for the current platform is what would be
chosen by SelectArtifact (modulo the shorter-URL tiebreaker on equal scores).

Use --hint to simulate different binary names (affects the name-matching points).`,
			Args: cobra.RangeArgs(1, 2),
			RunE: func(cmd *cobra.Command, args []string) error {
				specStr := args[0]
				spec, err := parsespec.Parse(specStr)
				if err != nil {
					return err
				}

				version := spec.Version
				if len(args) == 2 {
					version = args[1]
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

				artifacts, err := at.ListArtifacts(cmd.Context(), version)
				if err != nil {
					return err
				}

				effectiveHint := hint
				if effectiveHint == "" {
					effectiveHint = filepath.Base(spec.Package)
				}

				type entry struct {
					backend.Artifact
					Score int
				}

				entries := make([]entry, len(artifacts))
				for i, a := range artifacts {
					entries[i] = entry{
						Artifact: a,
						Score:    backend.ScoreArtifact(a, runtime.GOOS, runtime.GOARCH, effectiveHint),
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
					return i < j // keep relative order for equal scores
				})

				w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
				fmt.Fprintf(cmd.OutOrStdout(), "# platform=%s/%s  hint=%q  version=%s  (0=ineligible)\n",
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
				_ = w.Flush()
				return nil
			},
		}
		cmd.Flags().StringVarP(&hint, "hint", "H", "", "binary name hint for scoring (overrides the default derived from the package name)")
		c.AddCommand(cmd)
	})
}
