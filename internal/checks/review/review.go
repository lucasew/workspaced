// Package review posts GitHub Actions workflow-command annotations for SARIF findings.
package review

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/lucasew/workspaced/internal/git"
	execdriver "github.com/lucasew/workspaced/pkg/driver/exec"
	"github.com/lucasew/workspaced/pkg/logging"

	"github.com/owenrumney/go-sarif/v2/sarif"
)

// AnnotateOptions controls review-mode behavior.
type AnnotateOptions struct {
	// Root is the repo / lint root (absolute).
	Root string
	// Out receives workflow commands (defaults to stdout).
	Out io.Writer
}

// AnnotateIfApplicable posts annotations when running on GitHub Actions.
// Outside GHA it logs a warning and returns nil (soft no-op).
// Exit code is never based on findings.
func AnnotateIfApplicable(ctx context.Context, report *sarif.Report, opts AnnotateOptions) error {
	logger := logging.GetLogger(ctx)
	if report == nil {
		return nil
	}
	if !IsGitHubActions() {
		logger.Warn("lint --review: not running on GitHub Actions; skipping annotations")
		return nil
	}
	out := opts.Out
	if out == nil {
		out = os.Stdout
	}
	root := opts.Root
	if root == "" {
		wd, err := os.Getwd()
		if err != nil {
			return err
		}
		root = wd
	}
	if r, err := git.GetRoot(ctx, root); err == nil {
		root = r
	}

	diffLines, err := RelevantDiffLines(ctx, root)
	if err != nil {
		logger.Warn("lint --review: cannot compute relevant diff; skipping annotations", "error", err)
		return nil
	}
	if len(diffLines) == 0 {
		logger.Warn("lint --review: empty relevant diff; skipping annotations")
		return nil
	}

	n := 0
	for _, run := range report.Runs {
		tool := run.Tool.Driver.Name
		for _, res := range run.Results {
			file, line, col, msg, level := extractFinding(res)
			if file == "" || line <= 0 {
				continue
			}
			rel := normalizeRepoPath(root, file)
			if !diffLines[fileLineKey(rel, line)] {
				continue
			}
			writeWorkflowCommand(out, level, rel, line, col, tool, msg)
			n++
		}
	}
	logger.Info("lint --review: wrote workflow annotations", "count", n)
	return nil
}

// IsGitHubActions reports whether GITHUB_ACTIONS is set truthily.
func IsGitHubActions() bool {
	v := strings.ToLower(strings.TrimSpace(os.Getenv("GITHUB_ACTIONS")))
	return v == "true" || v == "1"
}

// RelevantDiffLines returns set of "path:line" for added/changed lines in the relevant diff.
// Priority: base…HEAD when base known; else HEAD~1…HEAD.
func RelevantDiffLines(ctx context.Context, root string) (map[string]bool, error) {
	base, head, err := resolveDiffRange(ctx, root)
	if err != nil {
		return nil, err
	}
	return diffLineSet(ctx, root, base, head)
}

func resolveDiffRange(ctx context.Context, root string) (base, head string, err error) {
	head = strings.TrimSpace(os.Getenv("GITHUB_SHA"))
	if head == "" {
		head, err = gitRevParse(ctx, root, "HEAD")
		if err != nil {
			return "", "", err
		}
	}
	// Prefer explicit base from env (workflow can set WORKSPACED_REVIEW_BASE).
	if b := strings.TrimSpace(os.Getenv("WORKSPACED_REVIEW_BASE")); b != "" {
		return b, head, nil
	}
	// pull_request: GITHUB_BASE_REF is branch name; need origin/base or merge-base.
	if br := strings.TrimSpace(os.Getenv("GITHUB_BASE_REF")); br != "" {
		// Common checkout: origin/<base>
		cand := "origin/" + br
		if _, e := gitRevParse(ctx, root, cand); e == nil {
			mb, e2 := gitMergeBase(ctx, root, cand, head)
			if e2 == nil {
				return mb, head, nil
			}
			return cand, head, nil
		}
		if _, e := gitRevParse(ctx, root, br); e == nil {
			mb, e2 := gitMergeBase(ctx, root, br, head)
			if e2 == nil {
				return mb, head, nil
			}
			return br, head, nil
		}
	}
	// push: GITHUB_EVENT_BEFORE sometimes available as env in custom setups;
	// fall back to last commit.
	if before := strings.TrimSpace(os.Getenv("GITHUB_EVENT_BEFORE")); before != "" && before != "0000000000000000000000000000000000000000" {
		return before, head, nil
	}
	parent, err := gitRevParse(ctx, root, "HEAD~1")
	if err != nil {
		return "", "", fmt.Errorf("no base ref and no parent commit: %w", err)
	}
	return parent, head, nil
}

func gitRevParse(ctx context.Context, root, rev string) (string, error) {
	cmd := execdriver.MustRun(ctx, "git", "-C", root, "rev-parse", rev)
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

func gitMergeBase(ctx context.Context, root, a, b string) (string, error) {
	cmd := execdriver.MustRun(ctx, "git", "-C", root, "merge-base", a, b)
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

func diffLineSet(ctx context.Context, root, base, head string) (map[string]bool, error) {
	cmd := execdriver.MustRun(ctx, "git", "-C", root, "diff", "--unified=0", base, head)
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git diff %s %s: %w", base, head, err)
	}
	return parseUnifiedDiffLines(string(out)), nil
}

// parseUnifiedDiffLines builds path:line keys for added lines (+).
func parseUnifiedDiffLines(diff string) map[string]bool {
	set := map[string]bool{}
	var file string
	var newLine int
	for _, line := range strings.Split(diff, "\n") {
		if strings.HasPrefix(line, "+++ ") {
			p := strings.TrimPrefix(line, "+++ ")
			p = strings.TrimPrefix(p, "b/")
			if p == "/dev/null" {
				file = ""
				continue
			}
			file = filepath.ToSlash(p)
			continue
		}
		if strings.HasPrefix(line, "@@ ") {
			// @@ -a,b +c,d @@
			parts := strings.Split(line, " ")
			for _, p := range parts {
				if strings.HasPrefix(p, "+") && !strings.HasPrefix(p, "+++") {
					num := strings.TrimPrefix(p, "+")
					num = strings.Split(num, ",")[0]
					n, err := strconv.Atoi(num)
					if err == nil {
						newLine = n
					}
					break
				}
			}
			continue
		}
		if file == "" {
			continue
		}
		if strings.HasPrefix(line, "+") && !strings.HasPrefix(line, "+++") {
			set[fileLineKey(file, newLine)] = true
			newLine++
			continue
		}
		if strings.HasPrefix(line, "-") && !strings.HasPrefix(line, "---") {
			// deleted line in old file only
			continue
		}
		// context line
		if !strings.HasPrefix(line, "\\") && line != "" {
			// unprefixed context in unified=0 is rare; still advance
			newLine++
		}
	}
	return set
}

func fileLineKey(path string, line int) string {
	return filepath.ToSlash(path) + ":" + strconv.Itoa(line)
}

func normalizeRepoPath(root, file string) string {
	file = strings.TrimPrefix(file, "file://")
	if filepath.IsAbs(file) {
		if rel, err := filepath.Rel(root, file); err == nil && !strings.HasPrefix(rel, "..") {
			return filepath.ToSlash(rel)
		}
	}
	return filepath.ToSlash(file)
}

func extractFinding(res *sarif.Result) (file string, line, col int, msg, level string) {
	level = "warning"
	if res.Level != nil {
		level = *res.Level
	}
	if res.Message.Text != nil {
		msg = *res.Message.Text
	}
	if len(res.Locations) == 0 {
		return
	}
	loc := res.Locations[0].PhysicalLocation
	if loc == nil {
		return
	}
	if loc.ArtifactLocation != nil && loc.ArtifactLocation.URI != nil {
		file = *loc.ArtifactLocation.URI
	}
	if loc.Region != nil {
		if loc.Region.StartLine != nil {
			line = *loc.Region.StartLine
		}
		if loc.Region.StartColumn != nil {
			col = *loc.Region.StartColumn
		}
	}
	return
}

func writeWorkflowCommand(w io.Writer, level, file string, line, col int, tool, msg string) {
	// Map SARIF levels to workflow command types.
	cmd := "warning"
	switch strings.ToLower(level) {
	case "error":
		cmd = "error"
	case "note", "none":
		cmd = "notice"
	}
	msg = strings.ReplaceAll(msg, "\r", " ")
	msg = strings.ReplaceAll(msg, "\n", " ")
	if tool != "" {
		msg = tool + ": " + msg
	}
	// title optional; keep file/line/col
	props := fmt.Sprintf("file=%s,line=%d", file, line)
	if col > 0 {
		props += fmt.Sprintf(",col=%d", col)
	}
	_, _ = fmt.Fprintf(w, "::%s %s::%s\n", cmd, props, msg)
}
