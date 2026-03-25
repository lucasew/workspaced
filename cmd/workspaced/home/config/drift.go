package config

import (
	"encoding/json"
	"fmt"
	"reflect"
	"sort"

	"workspaced/pkg/config"

	"github.com/spf13/cobra"
)

func GetDriftCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "drift",
		Short: "Compare TOML config loading with experimental CUE loading",
		RunE: func(c *cobra.Command, args []string) error {
			tomlCfg, err := config.LoadWithFormat("toml")
			if err != nil {
				return fmt.Errorf("load toml config: %w", err)
			}
			cueCfg, err := config.LoadWithFormat("cue")
			if err != nil {
				return fmt.Errorf("load cue config: %w", err)
			}

			tomlMap, err := toMap(tomlCfg.GlobalConfig)
			if err != nil {
				return err
			}
			cueMap, err := toMap(cueCfg.GlobalConfig)
			if err != nil {
				return err
			}

			diffs := make([]string, 0)
			collectDiffs("", tomlMap, cueMap, &diffs)
			sort.Strings(diffs)
			if len(diffs) == 0 {
				_, _ = fmt.Fprintln(c.OutOrStdout(), "no drift")
				return nil
			}
			for _, diff := range diffs {
				_, _ = fmt.Fprintln(c.OutOrStdout(), diff)
			}
			return fmt.Errorf("detected %d drift entries", len(diffs))
		},
	}
}

func toMap(v any) (map[string]any, error) {
	data, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	var out map[string]any
	if err := json.Unmarshal(data, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func collectDiffs(prefix string, left any, right any, diffs *[]string) {
	if reflect.DeepEqual(left, right) {
		return
	}

	leftMap, leftOK := left.(map[string]any)
	rightMap, rightRight := right.(map[string]any)
	if leftOK && rightRight {
		keys := make(map[string]struct{})
		for k := range leftMap {
			keys[k] = struct{}{}
		}
		for k := range rightMap {
			keys[k] = struct{}{}
		}
		for k := range keys {
			path := k
			if prefix != "" {
				path = prefix + "." + k
			}
			lv, lok := leftMap[k]
			rv, rok := rightMap[k]
			if !lok {
				*diffs = append(*diffs, fmt.Sprintf("%s: missing in toml, cue=%v", path, rv))
				continue
			}
			if !rok {
				*diffs = append(*diffs, fmt.Sprintf("%s: toml=%v, missing in cue", path, lv))
				continue
			}
			collectDiffs(path, lv, rv, diffs)
		}
		return
	}

	*diffs = append(*diffs, fmt.Sprintf("%s: toml=%v cue=%v", prefix, left, right))
}
